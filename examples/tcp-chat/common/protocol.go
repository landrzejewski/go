package common

import (
	"encoding/json"
	"sync"
	"time"
)

// MessageType represents the type of message being sent
type MessageType string

const (
	// Message types
	TypeText         MessageType = "TEXT"
	TypeFile         MessageType = "FILE"
	TypeFileChunk    MessageType = "FILE_CHUNK"
	TypeFileComplete MessageType = "FILE_COMPLETE"
	TypeStatus       MessageType = "STATUS"
	TypeRoom         MessageType = "ROOM"
	TypeInvite       MessageType = "INVITE"
	TypeInviteResp   MessageType = "INVITE_RESP"
	TypeUserList     MessageType = "USER_LIST"
	TypeError        MessageType = "ERROR"
	TypeConnect      MessageType = "CONNECT"
	TypeDisconnect   MessageType = "DISCONNECT"
	TypeAck          MessageType = "ACK"
)

// UserStatus represents the status of a user
type UserStatus string

const (
	StatusActive    UserStatus = "ACTIVE"
	StatusBusy      UserStatus = "BUSY"
	StatusInvisible UserStatus = "INVISIBLE"
)

// RoomAction represents actions related to rooms
type RoomAction string

const (
	RoomCreate       RoomAction = "CREATE"
	RoomJoin         RoomAction = "JOIN"
	RoomLeave        RoomAction = "LEAVE"
	RoomLeaveConfirm RoomAction = "LEAVE_CONFIRM"
	RoomMsg          RoomAction = "MSG"
	RoomMembers      RoomAction = "MEMBERS"
	RoomKick         RoomAction = "KICK"
	RoomDelete       RoomAction = "DELETE"
	RoomSetTopic     RoomAction = "TOPIC"
)

// Message represents a message in the chat protocol
type Message struct {
	Type        MessageType `json:"type"`
	Sender      string      `json:"sender"`
	Recipient   string      `json:"recipient,omitempty"` // Empty for broadcast, "*" for all
	Room        string      `json:"room,omitempty"`
	Content     string      `json:"content,omitempty"`
	Status      UserStatus  `json:"status,omitempty"`
	Action      RoomAction  `json:"action,omitempty"`
	Filename    string      `json:"filename,omitempty"`
	Filesize    int64       `json:"filesize,omitempty"`
	FileID      string      `json:"file_id,omitempty"`
	ChunkNum    int         `json:"chunk_num,omitempty"`
	TotalChunks int         `json:"total_chunks,omitempty"`
	Data        []byte      `json:"data,omitempty"`
	Users       []string    `json:"users,omitempty"`
	Timestamp   time.Time   `json:"timestamp"`
	Error       string      `json:"error,omitempty"`
}

// NewTextMessage creates a new text message
func NewTextMessage(sender, recipient, content string) *Message {
	return &Message{
		Type:      TypeText,
		Sender:    sender,
		Recipient: recipient,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewBroadcastMessage creates a new broadcast message
func NewBroadcastMessage(sender, content string) *Message {
	return &Message{
		Type:      TypeText,
		Sender:    sender,
		Recipient: "*",
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewStatusMessage creates a new status update message
func NewStatusMessage(sender string, status UserStatus) *Message {
	return &Message{
		Type:      TypeStatus,
		Sender:    sender,
		Status:    status,
		Timestamp: time.Now(),
	}
}

// NewErrorMessage creates a new error message
func NewErrorMessage(sender, recipient, error string) *Message {
	return &Message{
		Type:      TypeError,
		Sender:    sender,
		Recipient: recipient,
		Error:     error,
		Timestamp: time.Now(),
	}
}

// Encode serializes the message to JSON
func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage deserializes a JSON message
func DecodeMessage(data []byte) (*Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	return &msg, err
}

// FileTransfer represents an ongoing file transfer
type FileTransfer struct {
	FileID         string
	Filename       string
	Filesize       int64
	Sender         string
	Recipient      string
	TotalChunks    int
	ReceivedChunks map[int][]byte
	StartTime      time.Time
	mutex          sync.RWMutex
}

// IsComplete checks if all chunks have been received
func (ft *FileTransfer) IsComplete() bool {
	ft.mutex.RLock()
	defer ft.mutex.RUnlock()
	return len(ft.ReceivedChunks) == ft.TotalChunks
}

// GetProgress returns the progress percentage
func (ft *FileTransfer) GetProgress() float64 {
	ft.mutex.RLock()
	defer ft.mutex.RUnlock()
	if ft.TotalChunks == 0 {
		return 0
	}
	return float64(len(ft.ReceivedChunks)) / float64(ft.TotalChunks) * 100
}

// AddChunk adds a chunk to the file transfer
func (ft *FileTransfer) AddChunk(chunkNum int, data []byte) {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()
	ft.ReceivedChunks[chunkNum] = data
}

// GetChunk retrieves a specific chunk
func (ft *FileTransfer) GetChunk(chunkNum int) ([]byte, bool) {
	ft.mutex.RLock()
	defer ft.mutex.RUnlock()
	data, exists := ft.ReceivedChunks[chunkNum]
	return data, exists
}
