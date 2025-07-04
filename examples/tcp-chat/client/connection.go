package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"tcp-chat/common"
)

// Connection manages the server connection
type Connection struct {
	conn          net.Conn
	nickname      string
	status        common.UserStatus
	sendChan      chan *common.Message
	receiveChan   chan *common.Message
	fileTransfers map[string]*FileTransferProgress
	connected     bool
	mutex         sync.RWMutex
	reconnectChan chan bool
	connectedChan chan bool
	ctx           context.Context
	cancel        context.CancelFunc
}

// FileTransferProgress tracks file transfer progress
type FileTransferProgress struct {
	FileID      string
	Filename    string
	Filesize    int64
	IsIncoming  bool
	Progress    float64
	StartTime   time.Time
	Chunks      map[int][]byte
	TotalChunks int
	mutex       sync.Mutex
}

// NewConnection creates a new connection manager
func NewConnection(nickname string) *Connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &Connection{
		nickname:      nickname,
		status:        common.StatusActive,
		sendChan:      make(chan *common.Message, 100),
		receiveChan:   make(chan *common.Message, 100),
		fileTransfers: make(map[string]*FileTransferProgress),
		reconnectChan: make(chan bool, 1),
		connectedChan: make(chan bool, 1),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Connect establishes connection to the server
func (c *Connection) Connect(address string) error {
	// Cancel any existing goroutines
	if c.cancel != nil {
		c.cancel()
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())

	// Set connection timeout
	conn, err := net.DialTimeout("tcp", address, common.ConnectionTimeout)
	if err != nil {
		return err
	}

	c.mutex.Lock()
	c.conn = conn
	c.connected = true
	c.mutex.Unlock()

	// Set read/write deadlines
	conn.SetReadDeadline(time.Now().Add(common.ReadTimeout))
	conn.SetWriteDeadline(time.Now().Add(common.WriteTimeout))

	// Signal that we're connected
	select {
	case c.connectedChan <- true:
	default:
	}

	// Send connection message with nickname
	connectMsg := &common.Message{
		Type:    common.TypeConnect,
		Content: c.nickname,
	}

	if err := c.sendMessage(connectMsg); err != nil {
		conn.Close()
		return err
	}

	// Start read and write pumps with context
	go c.readPump(c.ctx)
	go c.writePump(c.ctx)

	return nil
}

// ConnectWithRetry connects with automatic retry
func (c *Connection) ConnectWithRetry(address string) {
	backoff := time.Second
	maxBackoff := time.Minute

	for {
		log.Printf("Connecting to %s...", address)
		err := c.Connect(address)

		if err == nil {
			log.Println("Connected successfully!")
			c.SetConnected(true)
			return
		}

		log.Printf("Connection failed: %v. Retrying in %v...", err, backoff)
		time.Sleep(backoff)

		// Exponential backoff
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}

		// Check if we should stop retrying
		select {
		case <-c.reconnectChan:
			return
		default:
		}
	}
}

// IsConnected returns connection status
func (c *Connection) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.connected
}

// WaitForConnection blocks until connected
func (c *Connection) WaitForConnection() {
	<-c.connectedChan
}

// SetConnected sets connection status
func (c *Connection) SetConnected(connected bool) {
	c.mutex.Lock()
	c.connected = connected
	c.mutex.Unlock()
}

// Disconnect closes the connection
func (c *Connection) Disconnect() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Cancel context to stop goroutines
	if c.cancel != nil {
		c.cancel()
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			log.Printf("Error closing connection: %v", err)
		}
		c.connected = false
	}

	// Signal to stop reconnection attempts
	select {
	case c.reconnectChan <- true:
	default:
	}
}

// SendTextMessage sends a text message
func (c *Connection) SendTextMessage(recipient, content string) {
	msg := common.NewTextMessage(c.nickname, recipient, content)
	c.sendChan <- msg
}

// SendBroadcastMessage sends a broadcast message
func (c *Connection) SendBroadcastMessage(content string) {
	msg := common.NewBroadcastMessage(c.nickname, content)
	c.sendChan <- msg
}

// SendRoomMessage sends a message to a room
func (c *Connection) SendRoomMessage(roomID, content string) {
	msg := &common.Message{
		Type:      common.TypeText,
		Room:      roomID,
		Content:   content,
		Timestamp: time.Now(),
	}
	c.sendChan <- msg
}

// ChangeStatus updates user status
func (c *Connection) ChangeStatus(status common.UserStatus) {
	c.status = status
	msg := common.NewStatusMessage(c.nickname, status)
	c.sendChan <- msg
}

// CreateRoom creates a new room
func (c *Connection) CreateRoom(name string) {
	msg := &common.Message{
		Type:      common.TypeRoom,
		Action:    common.RoomCreate,
		Content:   name,
		Timestamp: time.Now(),
	}
	c.sendChan <- msg
}

// InviteToRoom invites a user to a room
func (c *Connection) InviteToRoom(roomID, userNickname string) {
	msg := &common.Message{
		Type:      common.TypeInvite,
		Room:      roomID,
		Recipient: userNickname,
		Timestamp: time.Now(),
	}
	c.sendChan <- msg
}

// RespondToInvite responds to a room invitation
func (c *Connection) RespondToInvite(roomID string, accept bool) {
	response := "decline"
	if accept {
		response = "accept"
	}

	msg := &common.Message{
		Type:      common.TypeInviteResp,
		Room:      roomID,
		Content:   response,
		Timestamp: time.Now(),
	}
	c.sendChan <- msg
}

// LeaveRoom sends a leave room message
func (c *Connection) LeaveRoom(roomID string) {
	msg := &common.Message{
		Type:      common.TypeRoom,
		Action:    common.RoomLeave,
		Room:      roomID,
		Timestamp: time.Now(),
	}
	c.sendChan <- msg
}

// GetRoomMembers requests the member list for a room
func (c *Connection) GetRoomMembers(roomID string) {
	msg := &common.Message{
		Type:      common.TypeRoom,
		Action:    common.RoomMembers,
		Room:      roomID,
		Timestamp: time.Now(),
	}
	c.sendChan <- msg
}

// KickFromRoom kicks a user from a room (creator only)
func (c *Connection) KickFromRoom(roomID, nickname string) {
	msg := &common.Message{
		Type:      common.TypeRoom,
		Action:    common.RoomKick,
		Room:      roomID,
		Recipient: nickname,
		Timestamp: time.Now(),
	}
	c.sendChan <- msg
}

// DeleteRoom deletes a room (creator only)
func (c *Connection) DeleteRoom(roomID string) {
	msg := &common.Message{
		Type:      common.TypeRoom,
		Action:    common.RoomDelete,
		Room:      roomID,
		Timestamp: time.Now(),
	}
	c.sendChan <- msg
}

// SetRoomTopic sets the topic/description for a room
func (c *Connection) SetRoomTopic(roomID, description string) {
	msg := &common.Message{
		Type:      common.TypeRoom,
		Action:    common.RoomSetTopic,
		Room:      roomID,
		Content:   description,
		Timestamp: time.Now(),
	}
	c.sendChan <- msg
}

// GetMessages returns the receive channel
func (c *Connection) GetMessages() <-chan *common.Message {
	return c.receiveChan
}

// readPump reads messages from the server
func (c *Connection) readPump(ctx context.Context) {
	defer func() {
		c.SetConnected(false)
		c.conn.Close()
	}()

	scanner := bufio.NewScanner(c.conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Reset read deadline on successful read
		c.conn.SetReadDeadline(time.Now().Add(common.ReadTimeout))

		data := scanner.Bytes()
		msg, err := common.DecodeMessage(data)
		if err != nil {
			log.Printf("Error decoding message: %v", err)
			continue
		}

		// Handle file chunks separately
		if msg.Type == common.TypeFileChunk {
			c.handleFileChunk(msg)
		} else {
			c.receiveChan <- msg
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Read error: %v", err)
	}
}

// writePump writes messages to the server
func (c *Connection) writePump(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-c.sendChan:
			if err := c.sendMessage(msg); err != nil {
				log.Printf("Write error: %v", err)
				return
			}

		case <-ticker.C:
			// Keep alive
			if c.conn != nil {
				c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			}
		}
	}
}

// sendMessage sends a message to the server
func (c *Connection) sendMessage(msg *common.Message) error {
	data, err := msg.Encode()
	if err != nil {
		return err
	}

	c.mutex.RLock()
	conn := c.conn
	c.mutex.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	// Set write deadline
	conn.SetWriteDeadline(time.Now().Add(common.WriteTimeout))

	_, err = conn.Write(append(data, '\n'))
	return err
}

// handleFileChunk processes incoming file chunks
func (c *Connection) handleFileChunk(msg *common.Message) {
	c.mutex.Lock()
	transfer, exists := c.fileTransfers[msg.FileID]
	if !exists {
		// New incoming file transfer
		transfer = &FileTransferProgress{
			FileID:      msg.FileID,
			Filename:    msg.Filename,
			Filesize:    msg.Filesize,
			IsIncoming:  true,
			StartTime:   time.Now(),
			Chunks:      make(map[int][]byte),
			TotalChunks: msg.TotalChunks,
		}
		c.fileTransfers[msg.FileID] = transfer
	}
	c.mutex.Unlock()

	// Store chunk with transfer-specific lock
	transfer.mutex.Lock()
	transfer.Chunks[msg.ChunkNum] = msg.Data
	transfer.Progress = float64(len(transfer.Chunks)) / float64(transfer.TotalChunks) * 100
	chunkCount := len(transfer.Chunks)
	transfer.mutex.Unlock()

	// Forward to UI for progress display
	progressMsg := &common.Message{
		Type:     common.TypeFileChunk,
		FileID:   msg.FileID,
		Filename: transfer.Filename,
		Content:  fmt.Sprintf("%.1f%%", transfer.Progress),
	}
	c.receiveChan <- progressMsg

	// Check if complete
	if chunkCount == transfer.TotalChunks {
		completeMsg := &common.Message{
			Type:     common.TypeFileComplete,
			FileID:   msg.FileID,
			Filename: transfer.Filename,
		}
		c.receiveChan <- completeMsg
	}
}
