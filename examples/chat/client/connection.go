package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"training.pl/go/examples/chat/common"
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
	// Cancel any existing goroutines. c.cancel is read under c.mutex
	// in Disconnect(), so swapping it must happen under the lock too.
	c.mutex.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	ctx := c.ctx
	c.mutex.Unlock()

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

	// Send connection message with nickname
	connectMsg := &common.Message{
		Type:    common.TypeConnect,
		Content: c.nickname,
	}

	if err := c.sendMessage(connectMsg); err != nil {
		conn.Close()
		return err
	}

	// Signal only AFTER the CONNECT message was sent successfully - previously
	// WaitForConnection() could also return when sendMessage then failed.
	select {
	case c.connectedChan <- true:
	default:
	}

	// Start read and write pumps with context.
	// ctx and conn are passed as values read earlier under the lock - reading
	// c.ctx / c.conn here would race with another Connect().
	go c.readPump(ctx, conn)
	go c.writePump(ctx, conn)

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
	c.mutex.Lock()
	c.status = status
	c.mutex.Unlock()
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
// readPump takes the net.Conn as a parameter instead of reading the c.conn field,
// which Connect() writes under c.mutex - otherwise it would be a data race.
func (c *Connection) readPump(ctx context.Context, conn net.Conn) {
	defer func() {
		c.SetConnected(false)
		conn.Close()
	}()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), common.MaxScannerBuffer)

	for scanner.Scan() {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Reset read deadline on successful read
		conn.SetReadDeadline(time.Now().Add(common.ReadTimeout))

		data := scanner.Bytes()
		msg, err := common.DecodeMessage(data)
		if err != nil {
			log.Printf("Error decoding message: %v", err)
			continue
		}

		// Handle file chunks separately
		if msg.Type == common.TypeFileChunk {
			c.handleFileChunk(ctx, msg)
		} else {
			// The init message carries the file name, size and chunk count - the
			// chunks themselves do not. Without registering the transfer here the
			// record was created only from the first chunk, with an EMPTY name, so
			// writing it failed with an error
			// "invalid filename".
			if msg.Type == common.TypeFile {
				c.registerIncomingTransfer(msg)
			}
			// select with ctx.Done(): if the UI stopped receiving, a bare
			// `c.receiveChan <- msg` would block this goroutine forever.
			select {
			case c.receiveChan <- msg:
			case <-ctx.Done():
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Read error: %v", err)
	}
}

// writePump writes messages to the server.
//
// Like readPump it receives conn as a parameter rather than reading the c.conn
// field: Connect() writes that field under c.mutex, so touching it here would be a
// data race - and after a reconnect the deferred Close would shut down the NEW
// connection.
func (c *Connection) writePump(ctx context.Context, conn net.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		conn.Close()
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
			// A real keep-alive: the SERVER resets its read deadline only after a
			// successful read, so something has to be SENT to it. Merely setting a
			// write deadline (the previous version) sent nothing, so a silent user
			// was disconnected after ReadTimeout (60 s).
			keepAlive := &common.Message{
				Type:      common.TypeAck,
				Sender:    c.nickname,
				Timestamp: time.Now(),
			}
			if err := c.sendMessage(keepAlive); err != nil {
				log.Printf("Keep-alive error: %v", err)
				return
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

// registerIncomingTransfer records the metadata of an incoming transfer before its
// chunks arrive.
func (c *Connection) registerIncomingTransfer(msg *common.Message) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if _, exists := c.fileTransfers[msg.FileID]; exists {
		return
	}
	c.fileTransfers[msg.FileID] = &FileTransferProgress{
		FileID:      msg.FileID,
		Filename:    msg.Filename,
		Filesize:    msg.Filesize,
		IsIncoming:  true,
		StartTime:   time.Now(),
		Chunks:      make(map[int][]byte),
		TotalChunks: msg.TotalChunks,
	}
}

// handleFileChunk processes incoming file chunks
func (c *Connection) handleFileChunk(ctx context.Context, msg *common.Message) {
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
	// A chunk may arrive before we have processed the init message.
	if transfer.TotalChunks == 0 && msg.TotalChunks > 0 {
		transfer.TotalChunks = msg.TotalChunks
	}
	transfer.Chunks[msg.ChunkNum] = msg.Data
	// Guard against division by zero (it produced +Inf in the message).
	if transfer.TotalChunks > 0 {
		transfer.Progress = float64(len(transfer.Chunks)) / float64(transfer.TotalChunks) * 100
	}
	// Progress and Filename are guarded by this lock too, so read them inside the
	// critical section rather than after Unlock.
	progress := transfer.Progress
	filename := transfer.Filename
	transfer.mutex.Unlock()

	// Forward to UI for progress display
	progressMsg := &common.Message{
		Type:     common.TypeFileChunk,
		FileID:   msg.FileID,
		Filename: filename,
		Content:  fmt.Sprintf("%.1f%%", progress),
	}
	select {
	case c.receiveChan <- progressMsg:
	case <-ctx.Done():
		return
	}

	// Completion is signalled by the SERVER (TypeFileComplete). The client used to
	// add its own message as well, so a file was "received" twice - the second time
	// the transfer record no longer existed and the user saw a confusing
	// "Error saving file: file transfer not found".
}
