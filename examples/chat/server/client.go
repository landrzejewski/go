package main

import (
	"bufio"
	"log"
	"net"
	"sync"
	"time"

	"tcp-chat/common"
)

// Client represents a connected client
type Client struct {
	ID       string
	Conn     net.Conn
	SendChan chan *common.Message
	Server   *Server

	// RemoteAddr jest ustawiane raz, przed startem pomp, i dalej tylko czytane.
	RemoteAddr string

	// Nickname, Status i Rooms są czytane przez goroutines INNYCH klientów
	// (BroadcastMessage, BroadcastUserList), więc dostęp wyłącznie przez
	// metody GetNickname/SetNickname, GetStatus/SetStatus itd.
	Nickname string
	Status   common.UserStatus
	Rooms    map[string]bool
	mutex    sync.RWMutex

	// done zamiast zamykania SendChan. Kanał wysyłkowy zamyka się tylko po
	// stronie NADAWCY, a nadawców jest tu wielu (każdy klient rozgłaszający
	// wiadomość). Zamykanie go w Close() powodowało panikę "send on closed
	// channel", która ubijała cały serwer - select z default chroni wyłącznie
	// przed pełnym kanałem, nie przed zamkniętym.
	done      chan struct{}
	closeOnce sync.Once
}

// NewClient creates a new client instance
func NewClient(conn net.Conn, server *Server) *Client {
	return &Client{
		ID:       common.GenerateID("client"),
		Conn:     conn,
		Status:   common.StatusActive,
		Rooms:    make(map[string]bool),
		SendChan: make(chan *common.Message, 256),
		Server:   server,
		done:     make(chan struct{}),
	}
}

// GetNickname returns the client's nickname (safe for concurrent use)
func (c *Client) GetNickname() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.Nickname
}

// SetNickname updates the client's nickname
func (c *Client) SetNickname(nickname string) {
	c.mutex.Lock()
	c.Nickname = nickname
	c.mutex.Unlock()
}

// GetStatus returns the client's current status
func (c *Client) GetStatus() common.UserStatus {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.Status
}

// SetStatus updates the client's status
func (c *Client) SetStatus(status common.UserStatus) {
	c.mutex.Lock()
	c.Status = status
	c.mutex.Unlock()
}

// AddRoom adds a room to the client's room list
func (c *Client) AddRoom(roomID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.Rooms[roomID] = true
}

// RemoveRoom removes a room from the client's room list
func (c *Client) RemoveRoom(roomID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.Rooms, roomID)
}

// IsInRoom checks if the client is in a specific room
func (c *Client) IsInRoom(roomID string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.Rooms[roomID]
}

// SendMessage sends a message to the client.
// Kanał nigdy nie jest zamykany, więc wysyłka nie może zapanikować; zakończonego
// klienta rozpoznajemy po zamkniętym done.
func (c *Client) SendMessage(msg *common.Message) {
	select {
	case <-c.done:
		return
	default:
	}

	select {
	case c.SendChan <- msg:
	case <-c.done:
	default:
		log.Printf("Client %s send channel full, dropping message", c.GetNickname())
	}
}

// ReadPump reads messages from the client connection
func (c *Client) ReadPump() {
	defer func() {
		c.Server.UnregisterClient(c)
		c.Close()
	}()

	scanner := bufio.NewScanner(c.Conn)
	// Domyślny limit bufio.Scanner to 64 KiB na token - podnosimy go do stałej
	// współdzielonej z klientem, zamiast wpisywać rozmiar na sztywno.
	scanner.Buffer(make([]byte, 0, 64*1024), common.MaxScannerBuffer)

	for scanner.Scan() {
		// Reset read deadline on successful read
		c.Conn.SetReadDeadline(time.Now().Add(common.ReadTimeout))
		data := scanner.Bytes()
		msg, err := common.DecodeMessage(data)
		if err != nil {
			log.Printf("Error decoding message from %s: %v", c.Nickname, err)
			continue
		}

		// Set sender to client's nickname
		msg.Sender = c.GetNickname()
		msg.Timestamp = time.Now()

		// Handle the message
		if err := c.Server.HandleMessage(c, msg); err != nil {
			nickname := c.GetNickname()
			log.Printf("Error handling message from %s: %v", nickname, err)
			// Send error message back to client
			errMsg := common.NewErrorMessage("Server", nickname, err.Error())
			c.SendMessage(errMsg)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from %s: %v", c.GetNickname(), err)
	}
}

// WritePump writes messages to the client connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(common.KeepAliveInterval)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case <-c.done:
			return

		case msg := <-c.SendChan:
			data, err := msg.Encode()
			if err != nil {
				log.Printf("Error encoding message: %v", err)
				continue
			}

			// Set write deadline
			c.Conn.SetWriteDeadline(time.Now().Add(common.WriteTimeout))

			if _, err := c.Conn.Write(append(data, '\n')); err != nil {
				log.Printf("Error writing to %s: %v", c.GetNickname(), err)
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			ping := &common.Message{
				Type:      common.TypeAck,
				Timestamp: time.Now(),
			}

			data, _ := ping.Encode()

			// Set write deadline for ping
			c.Conn.SetWriteDeadline(time.Now().Add(common.WriteTimeout))

			if _, err := c.Conn.Write(append(data, '\n')); err != nil {
				return
			}
		}
	}
}

// Start begins the client's read and write pumps
// Start uruchamia obie pompy i BLOKUJE do ich zakończenia, dzięki czemu
// wywołujący (handleNewConnection) wie, kiedy połączenie faktycznie się
// skończyło, i może wtedy zwolnić slot w limiterze.
func (c *Client) Start() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		c.WritePump()
	}()
	go func() {
		defer wg.Done()
		c.ReadPump()
	}()
	wg.Wait()
}

// Close sygnalizuje zakończenie i zamyka połączenie. Idempotentne dzięki
// sync.Once - Close bywa wołane i z ReadPump, i z handleShutdown.
// Nie zamykamy SendChan: zrobiłby to odbiorca, a nadawcy (inni klienci)
// zapanikowaliby przy próbie wysyłki.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.Conn != nil {
			c.Conn.Close()
		}
	})
}
