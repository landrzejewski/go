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
	ID         string
	Nickname   string
	Conn       net.Conn
	RemoteAddr string
	Status     common.UserStatus
	Rooms      map[string]bool
	SendChan   chan *common.Message
	Server     *Server
	mutex      sync.RWMutex
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
	}
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

// SendMessage sends a message to the client
func (c *Client) SendMessage(msg *common.Message) {
	select {
	case c.SendChan <- msg:
	default:
		log.Printf("Client %s send channel full, dropping message", c.Nickname)
	}
}

// ReadPump reads messages from the client connection
func (c *Client) ReadPump() {
	defer func() {
		c.Server.UnregisterClient(c)
		c.Close()
	}()

	scanner := bufio.NewScanner(c.Conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max message size

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
		msg.Sender = c.Nickname
		msg.Timestamp = time.Now()

		// Handle the message
		if err := c.Server.HandleMessage(c, msg); err != nil {
			log.Printf("Error handling message from %s: %v", c.Nickname, err)
			// Send error message back to client
			errMsg := common.NewErrorMessage("Server", c.Nickname, err.Error())
			c.SendMessage(errMsg)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from %s: %v", c.Nickname, err)
	}
}

// WritePump writes messages to the client connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.SendChan:
			if !ok {
				return
			}

			data, err := msg.Encode()
			if err != nil {
				log.Printf("Error encoding message: %v", err)
				continue
			}

			// Set write deadline
			c.Conn.SetWriteDeadline(time.Now().Add(common.WriteTimeout))

			if _, err := c.Conn.Write(append(data, '\n')); err != nil {
				log.Printf("Error writing to %s: %v", c.Nickname, err)
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
func (c *Client) Start() {
	go c.WritePump()
	go c.ReadPump()
}

// Close properly closes the client connection and channels
func (c *Client) Close() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Close send channel to signal WritePump to exit
	if c.SendChan != nil {
		close(c.SendChan)
		c.SendChan = nil
	}

	// Close connection
	if c.Conn != nil {
		c.Conn.Close()
	}
}
