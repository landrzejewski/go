package main

import (
	"bufio"
	"log"
	"net"
	"sync"
	"time"

	"training.pl/go/examples/chat/common"
)

// Client represents a connected client
type Client struct {
	ID       string
	Conn     net.Conn
	SendChan chan *common.Message
	Server   *Server

	// RemoteAddr is set once, before the pumps start, and only read afterwards.
	RemoteAddr string

	// Nickname, Status and Rooms are read by the goroutines of OTHER clients
	// (BroadcastMessage, BroadcastUserList), so access them only through
	// the GetNickname/SetNickname, GetStatus/SetStatus accessors.
	Nickname string
	Status   common.UserStatus
	Rooms    map[string]bool
	mutex    sync.RWMutex

	// done instead of closing SendChan. A send channel is closed only by the
	// SENDER, and here there are many senders (every client broadcasting a
	// message). Closing it in Close() caused a "send on closed channel" panic that
	// took down the whole server - a select with default guards only against a
	// full channel, not a closed one.
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
//
// The channel is never closed, so a send cannot panic; a finished client is
// recognised by its closed done channel.
func (c *Client) SendMessage(msg *common.Message) {
	select {
	case <-c.done:
		return
	default:
	}

	// No `case <-c.done` here: with a default branch present it would be
	// unreachable, because default is taken as soon as the send would block. The
	// policy is deliberately "drop when the buffer is full" - a slow client must
	// not stall whoever is broadcasting.
	select {
	case c.SendChan <- msg:
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
	// bufio.Scanner defaults to 64 KiB per token - raise it to the constant shared
	// with the client instead of hardcoding a size here.
	scanner.Buffer(make([]byte, 0, 64*1024), common.MaxScannerBuffer)

	for scanner.Scan() {
		// Reset read deadline on successful read
		c.Conn.SetReadDeadline(time.Now().Add(common.ReadTimeout))
		data := scanner.Bytes()
		msg, err := common.DecodeMessage(data)
		if err != nil {
			log.Printf("Error decoding message from %s: %v", c.GetNickname(), err)
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
// Start runs both pumps and BLOCKS until they finish, so the caller
// (handleNewConnection) knows when the connection has really ended and can release
// its slot in the rate limiter at that point.
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

// Close signals completion and closes the connection. It is idempotent thanks to
// sync.Once - Close is called both from ReadPump and from handleShutdown.
// SendChan is deliberately not closed: that would be the receiver closing it, and
// the senders (other clients) would panic on their next send.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.Conn != nil {
			c.Conn.Close()
		}
	})
}
