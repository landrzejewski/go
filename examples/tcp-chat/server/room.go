package main

import (
	"sync"
	"time"

	"tcp-chat/common"
)

// Room represents a private chat room
type Room struct {
	ID          string
	Name        string
	Description string
	Creator     string
	Members     map[string]bool
	Invitations map[string]bool
	CreatedAt   time.Time
	mutex       sync.RWMutex
}

// NewRoom creates a new room
func NewRoom(name, creator string) *Room {
	return &Room{
		ID:          common.GenerateID("room"),
		Name:        name,
		Description: "",
		Creator:     creator,
		Members:     map[string]bool{creator: true},
		Invitations: make(map[string]bool),
		CreatedAt:   time.Now(),
	}
}

// AddMember adds a member to the room
func (r *Room) AddMember(nickname string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.Members[nickname] = true
	delete(r.Invitations, nickname)
}

// RemoveMember removes a member from the room
func (r *Room) RemoveMember(nickname string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.Members, nickname)
}

// IsMember checks if a user is a member of the room
func (r *Room) IsMember(nickname string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.Members[nickname]
}

// InviteUser adds a user to the invitation list
func (r *Room) InviteUser(nickname string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.Invitations[nickname] = true
}

// IsInvited checks if a user is invited to the room
func (r *Room) IsInvited(nickname string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.Invitations[nickname]
}

// GetMembers returns a list of room members
func (r *Room) GetMembers() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	members := make([]string, 0, len(r.Members))
	for member := range r.Members {
		members = append(members, member)
	}
	return members
}

// SetDescription sets the room description
func (r *Room) SetDescription(description string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.Description = description
}

// GetDescription returns the room description
func (r *Room) GetDescription() string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.Description
}

// RoomManager manages all rooms
type RoomManager struct {
	rooms map[string]*Room
	mutex sync.RWMutex
}

// NewRoomManager creates a new room manager
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms: make(map[string]*Room),
	}
}

// CreateRoom creates a new room
func (rm *RoomManager) CreateRoom(name, creator string) *Room {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	room := NewRoom(name, creator)
	rm.rooms[room.ID] = room
	return room
}

// GetRoom retrieves a room by ID
func (rm *RoomManager) GetRoom(roomID string) (*Room, bool) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	room, exists := rm.rooms[roomID]
	return room, exists
}

// GetUserRooms returns all rooms a user is member of
func (rm *RoomManager) GetUserRooms(nickname string) []*Room {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	var userRooms []*Room
	for _, room := range rm.rooms {
		if room.IsMember(nickname) {
			userRooms = append(userRooms, room)
		}
	}
	return userRooms
}

// RemoveRoom removes a room
func (rm *RoomManager) RemoveRoom(roomID string) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	delete(rm.rooms, roomID)
}

// BroadcastToRoom sends a message to all room members
func (rm *RoomManager) BroadcastToRoom(server *Server, roomID string, msg *common.Message) {
	room, exists := rm.GetRoom(roomID)
	if !exists {
		return
	}

	members := room.GetMembers()
	for _, member := range members {
		if client, ok := server.GetClient(member); ok {
			// Don't send to invisible users unless they're the sender
			if client.GetStatus() == common.StatusInvisible && member != msg.Sender {
				continue
			}
			client.SendMessage(msg)
		}
	}
}
