package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"training.pl/go/examples/chat/common"
)

// Server represents the chat server
type Server struct {
	listener       net.Listener
	clients        sync.Map // map[string]*Client (nickname -> client)
	roomManager    *RoomManager
	fileTransfers  sync.Map // map[string]*common.FileTransfer
	rateLimiter    *RateLimiter
	cleanupManager *CleanupManager
	shutdown       chan bool
	regMutex       sync.Mutex // Mutex for client registration
}

// NewServer creates a new server instance
func NewServer() *Server {
	s := &Server{
		roomManager: NewRoomManager(),
		rateLimiter: NewRateLimiter(),
		shutdown:    make(chan bool),
	}
	s.cleanupManager = NewCleanupManager(s)
	return s
}

// Start starts the server on the specified port
func (s *Server) Start(port string) error {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %v", port, err)
	}

	s.listener = listener
	common.Info("Server started on port %s", port)

	// Start cleanup manager
	s.cleanupManager.Start()

	// Handle graceful shutdown
	go s.handleShutdown()

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				return nil
			default:
				common.Error("Error accepting connection: %v", err)
				continue
			}
		}

		// Limit check and connection registration in one atomic operation.
		if err := s.rateLimiter.TryAddConnection(conn.RemoteAddr()); err != nil {
			common.Warn("Connection rejected from %s: %v", conn.RemoteAddr(), err)
			conn.Close()
			continue
		}

		go s.handleNewConnection(conn)
	}
}

// handleNewConnection handles a new client connection
func (s *Server) handleNewConnection(conn net.Conn) {
	// The connection slot was already claimed in the accept loop
	// (TryAddConnection), so all that is needed here is to guarantee its release -
	// UNCONDITIONALLY, including when the client disconnects before registering a
	// nickname.
	//
	// Previously the release happened only in UnregisterClient, AFTER the early
	// return for an empty nickname - so every port scan and every rejected client
	// took a slot permanently. After MaxConnections such connections the server
	// refused everyone.
	defer s.rateLimiter.RemoveConnection(conn.RemoteAddr())

	// Set initial read/write deadlines
	conn.SetReadDeadline(time.Now().Add(common.ReadTimeout))
	conn.SetWriteDeadline(time.Now().Add(common.WriteTimeout))

	client := NewClient(conn, s)
	client.RemoteAddr = conn.RemoteAddr().String()
	common.Info("New connection from %s", conn.RemoteAddr())

	// Start blocks until both pumps finish, so the defer above runs exactly when
	// the connection really ends.
	client.Start()
}

// RegisterClient registers a new client with a nickname
func (s *Server) RegisterClient(client *Client, nickname string) (bool, error) {
	// Validate nickname
	if err := ValidateNickname(nickname); err != nil {
		return false, err
	}

	// Make registration atomic
	s.regMutex.Lock()
	defer s.regMutex.Unlock()

	// Double-check if nickname is already taken
	if _, exists := s.clients.Load(nickname); exists {
		return false, fmt.Errorf("nickname '%s' is already taken", nickname)
	}

	// Nickname is read by other clients' goroutines - go through the setter only.
	client.SetNickname(nickname)
	s.clients.Store(nickname, client)

	// Notify all users about new connection
	s.BroadcastUserList()

	// Send welcome message
	welcomeMsg := common.NewTextMessage("Server", nickname, fmt.Sprintf("Welcome to the chat, %s!", nickname))
	client.SendMessage(welcomeMsg)

	// Announce to others
	announceMsg := common.NewBroadcastMessage("Server", fmt.Sprintf("%s has joined the chat", nickname))
	s.BroadcastMessage(announceMsg, nickname)

	common.Info("Client registered: %s from %s", nickname, client.RemoteAddr)
	return true, nil
}

// UnregisterClient removes a client from the server
func (s *Server) UnregisterClient(client *Client) {
	// The connection slot is now released by the defer in handleNewConnection -
	// unconditionally and for every socket, including one that never registered.
	// Here we clean up only the state tied to the NICKNAME.
	nickname := client.GetNickname()
	if nickname == "" {
		return
	}

	s.clients.Delete(nickname)

	// Remove from all rooms and notify room members
	rooms := s.roomManager.GetUserRooms(nickname)
	for _, room := range rooms {
		room.RemoveMember(nickname)

		leaveMsg := common.NewTextMessage("Server", "", fmt.Sprintf("%s has disconnected from the room", nickname))
		leaveMsg.Room = room.ID
		s.roomManager.BroadcastToRoom(s, room.ID, leaveMsg)
	}

	// Notify all users
	disconnectMsg := common.NewBroadcastMessage("Server", fmt.Sprintf("%s has left the chat", nickname))
	s.BroadcastMessage(disconnectMsg, "")

	s.BroadcastUserList()

	// Clean up rate limiter
	s.rateLimiter.RemoveUser(nickname)

	common.Info("Client unregistered: %s", nickname)
}

// GetClient retrieves a client by nickname
func (s *Server) GetClient(nickname string) (*Client, bool) {
	value, exists := s.clients.Load(nickname)
	if !exists {
		return nil, false
	}
	return value.(*Client), true
}

// BroadcastMessage sends a message to all connected clients
func (s *Server) BroadcastMessage(msg *common.Message, exclude string) {
	s.clients.Range(func(key, value any) bool {
		client := value.(*Client)

		nickname := client.GetNickname()

		// Skip excluded client
		if nickname == exclude {
			return true
		}

		// Skip invisible clients
		if client.GetStatus() == common.StatusInvisible && msg.Sender != nickname {
			return true
		}

		client.SendMessage(msg)
		return true
	})
}

// BroadcastUserList sends the list of online users to all clients
func (s *Server) BroadcastUserList() {
	var users []string

	s.clients.Range(func(key, value any) bool {
		client := value.(*Client)
		// Don't include invisible users in the list
		if client.GetStatus() != common.StatusInvisible {
			users = append(users, fmt.Sprintf("%s:%s", client.GetNickname(), client.GetStatus()))
		}
		return true
	})

	msg := &common.Message{
		Type:  common.TypeUserList,
		Users: users,
	}

	s.BroadcastMessage(msg, "")
}

// HandleMessage processes incoming messages from clients
func (s *Server) HandleMessage(client *Client, msg *common.Message) error {
	nickname := client.GetNickname()
	common.Debug("Handling %s message from %s", msg.Type, nickname)

	// Everything except CONNECT requires registration. Without this an anonymous
	// client could broadcast messages, create rooms and start file transfers - and
	// every rate-limit map was keyed by the empty nickname, so all unregistered
	// clients shared a single bucket.
	if msg.Type != common.TypeConnect && nickname == "" {
		return common.NewChatError(common.ErrUnauthorized, "you must register with CONNECT first")
	}

	switch msg.Type {
	case common.TypeConnect:
		// A second CONNECT after a successful registration would leave the old
		// nickname in the client map pointing at the same connection - a "ghost" on
		// the user list.
		if nickname != "" {
			return common.NewChatError(common.ErrValidation, "already registered")
		}

		// Handle client connection with nickname
		if success, err := s.RegisterClient(client, msg.Content); success {
			ackMsg := common.NewTextMessage("Server", msg.Content, "Connected successfully")
			client.SendMessage(ackMsg)
		} else {
			errMsg := common.NewErrorMessage("Server", msg.Sender, err.Error())
			client.SendMessage(errMsg)

			// The message is only QUEUED, so an immediate Conn.Close() almost
			// always cut it off before it was sent and the rejected user saw a bare
			// disconnect with no reason. Give WritePump a moment to drain the queue.
			go func() {
				time.Sleep(100 * time.Millisecond)
				client.Close()
			}()
		}

	case common.TypeText:
		// Check rate limit
		if err := s.rateLimiter.CanSendMessage(nickname); err != nil {
			common.Warn("Rate limit exceeded for %s: %v", nickname, err)
			errMsg := common.NewErrorMessage("Server", msg.Sender, err.Error())
			client.SendMessage(errMsg)
			return nil
		}

		// Validate message content
		if err := ValidateMessage(msg.Content); err != nil {
			errMsg := common.NewErrorMessage("Server", msg.Sender, err.Error())
			client.SendMessage(errMsg)
			return nil
		}

		// Handle text messages.
		//
		// ORDER MATTERS: the Room test must come FIRST. The client
		// (Connection.SendRoomMessage) sets only the Room field and
		// leaves Recipient empty, so with the previous ordering every room message
		// fell into the `Recipient == ""` branch and was broadcast to EVERY
		// registered user - and the membership check below was dead code. Private
		// rooms were therefore not private at all.
		if msg.Room != "" {
			// Room message - validate sender is a member
			if room, exists := s.roomManager.GetRoom(msg.Room); exists {
				if !room.IsMember(nickname) {
					errMsg := common.NewErrorMessage("Server", nickname, "You are not a member of this room")
					client.SendMessage(errMsg)
					return nil
				}
				s.roomManager.BroadcastToRoom(s, msg.Room, msg)
			} else {
				errMsg := common.NewErrorMessage("Server", nickname, "Room not found")
				client.SendMessage(errMsg)
			}
		} else if msg.Recipient == "*" || msg.Recipient == "" {
			// Broadcast message
			s.BroadcastMessage(msg, "")
		} else {
			// Private message
			if recipient, ok := s.GetClient(msg.Recipient); ok {
				recipient.SendMessage(msg)
				// Send copy to sender
				client.SendMessage(msg)
			} else {
				errMsg := common.NewErrorMessage("Server", msg.Sender, fmt.Sprintf("User %s not found", msg.Recipient))
				client.SendMessage(errMsg)
			}
		}

	case common.TypeStatus:
		// Handle status update
		client.SetStatus(msg.Status)
		s.BroadcastUserList()

		// Notify about status change
		statusMsg := common.NewBroadcastMessage("Server", fmt.Sprintf("%s is now %s", nickname, msg.Status))
		s.BroadcastMessage(statusMsg, nickname)

	case common.TypeRoom:
		s.handleRoomMessage(client, msg)

	case common.TypeInvite:
		s.handleInviteMessage(client, msg)

	case common.TypeInviteResp:
		s.handleInviteResponse(client, msg)

	case common.TypeFile:
		s.handleFileTransferInit(client, msg)

	case common.TypeFileChunk:
		s.handleFileChunk(client, msg)

	case common.TypeAck:
		// A keep-alive from the client. No reply is needed - merely
		// receiving the message has already refreshed the read deadline in
		// ReadPump. Without this branch a keep-alive fell into default and came
		// back as an "unknown message type" error.

	case common.TypeDisconnect:
		// The client sends this on Ctrl-C (client/main.go). Without this branch it
		// fell into default and bounced back as "unknown message type: DISCONNECT".
		common.Info("Client %s requested disconnect", nickname)
		s.UnregisterClient(client)
		client.Close()

	default:
		return common.NewChatError(common.ErrValidation, fmt.Sprintf("unknown message type: %s", msg.Type))
	}
	return nil
}

// handleRoomMessage handles room-related messages
func (s *Server) handleRoomMessage(client *Client, msg *common.Message) {
	nickname := client.GetNickname()

	switch msg.Action {
	case common.RoomCreate:
		// Check rate limit for room creation
		if err := s.rateLimiter.CanCreateRoom(client.GetNickname()); err != nil {
			errMsg := common.NewErrorMessage("Server", client.GetNickname(), err.Error())
			client.SendMessage(errMsg)
			return
		}

		// Validate room name
		if err := ValidateRoomName(msg.Content); err != nil {
			errMsg := common.NewErrorMessage("Server", client.GetNickname(), err.Error())
			client.SendMessage(errMsg)
			return
		}

		room := s.roomManager.CreateRoom(strings.TrimSpace(msg.Content), client.GetNickname())
		client.AddRoom(room.ID)
		s.rateLimiter.AddRoom(client.GetNickname())

		response := &common.Message{
			Type:     common.TypeRoom,
			Action:   common.RoomCreate,
			Room:     room.ID,
			RoomName: room.Name,
			Content:  fmt.Sprintf("Room '%s' created successfully", room.Name),
		}
		client.SendMessage(response)

	case common.RoomJoin:
		if room, exists := s.roomManager.GetRoom(msg.Room); exists {
			// Only the creator or an invited user may join. This branch used to
			// check NOTHING, so knowing a room id was enough to walk into someone
			// else's "private" room - while handleInviteResponse right below
			// carefully verifies IsInvited.
			if room.Creator != nickname && !room.IsInvited(nickname) {
				errMsg := common.NewErrorMessage("Server", nickname, "You have not been invited to this room")
				client.SendMessage(errMsg)
				return
			}
			if !room.IsMember(nickname) {
				room.AddMember(nickname)
				client.AddRoom(room.ID)

				// Notify room members
				joinMsg := common.NewTextMessage("Server", "", fmt.Sprintf("%s has joined the room", client.GetNickname()))
				joinMsg.Room = msg.Room
				s.roomManager.BroadcastToRoom(s, msg.Room, joinMsg)

				// Send success message to joiner
				response := &common.Message{
					Type:     common.TypeRoom,
					Action:   common.RoomJoin,
					Room:     room.ID,
					RoomName: room.Name,
					Content:  fmt.Sprintf("Joined room '%s'", room.Name),
				}
				client.SendMessage(response)
			}
		} else {
			errMsg := common.NewErrorMessage("Server", client.GetNickname(), "Room not found")
			client.SendMessage(errMsg)
		}

	case common.RoomLeave:
		if room, exists := s.roomManager.GetRoom(msg.Room); exists {
			room.RemoveMember(client.GetNickname())
			client.RemoveRoom(msg.Room)

			// Send confirmation to the leaving user
			confirmMsg := &common.Message{
				Type:     common.TypeRoom,
				Action:   common.RoomLeaveConfirm,
				Room:     room.ID,
				RoomName: room.Name,
				Content:  fmt.Sprintf("Left room '%s'", room.Name),
			}
			client.SendMessage(confirmMsg)

			// Notify room members
			leaveMsg := common.NewTextMessage("Server", "", fmt.Sprintf("%s has left the room", client.GetNickname()))
			leaveMsg.Room = msg.Room
			s.roomManager.BroadcastToRoom(s, msg.Room, leaveMsg)
		}

	case common.RoomMembers:
		if room, exists := s.roomManager.GetRoom(msg.Room); exists {
			// Check if user is a member
			if !room.IsMember(client.GetNickname()) {
				errMsg := common.NewErrorMessage("Server", client.GetNickname(), "You are not a member of this room")
				client.SendMessage(errMsg)
				return
			}

			// Get members and their status
			members := room.GetMembers()
			var memberList []string
			for _, member := range members {
				status := "offline"
				if memberClient, ok := s.GetClient(member); ok {
					status = string(memberClient.GetStatus())
				}
				memberList = append(memberList, fmt.Sprintf("%s (%s)", member, status))
			}

			// Send member list
			roomInfo := fmt.Sprintf("Room '%s'", room.Name)
			if desc := room.GetDescription(); desc != "" {
				roomInfo = fmt.Sprintf("%s (Topic: %s)", roomInfo, desc)
			}
			response := &common.Message{
				Type:    common.TypeRoom,
				Action:  common.RoomMembers,
				Room:    room.ID,
				Content: fmt.Sprintf("%s members: %s", roomInfo, strings.Join(memberList, ", ")),
			}
			client.SendMessage(response)
		} else {
			errMsg := common.NewErrorMessage("Server", client.GetNickname(), "Room not found")
			client.SendMessage(errMsg)
		}

	case common.RoomKick:
		if room, exists := s.roomManager.GetRoom(msg.Room); exists {
			// Check if user is the room creator
			if room.Creator != client.GetNickname() {
				errMsg := common.NewErrorMessage("Server", client.GetNickname(), "Only the room creator can kick members")
				client.SendMessage(errMsg)
				return
			}

			// Check if target is a member
			if !room.IsMember(msg.Recipient) {
				errMsg := common.NewErrorMessage("Server", client.GetNickname(), fmt.Sprintf("%s is not a member of this room", msg.Recipient))
				client.SendMessage(errMsg)
				return
			}

			// Can't kick yourself
			if msg.Recipient == client.GetNickname() {
				errMsg := common.NewErrorMessage("Server", client.GetNickname(), "You cannot kick yourself")
				client.SendMessage(errMsg)
				return
			}

			// Remove the member
			room.RemoveMember(msg.Recipient)

			// Remove room from kicked user's list
			if kickedClient, ok := s.GetClient(msg.Recipient); ok {
				kickedClient.RemoveRoom(msg.Room)

				// Notify the kicked user
				kickMsg := &common.Message{
					Type:     common.TypeRoom,
					Action:   common.RoomLeaveConfirm,
					Room:     room.ID,
					RoomName: room.Name,
					Content:  fmt.Sprintf("You have been kicked from room '%s'", room.Name),
				}
				kickedClient.SendMessage(kickMsg)
			}

			// Notify room members
			kickNotifyMsg := common.NewTextMessage("Server", "", fmt.Sprintf("%s has been kicked from the room by %s", msg.Recipient, client.GetNickname()))
			kickNotifyMsg.Room = msg.Room
			s.roomManager.BroadcastToRoom(s, msg.Room, kickNotifyMsg)

			// Confirm to the kicker
			confirmMsg := common.NewTextMessage("Server", client.GetNickname(), fmt.Sprintf("%s has been kicked from the room", msg.Recipient))
			client.SendMessage(confirmMsg)
		} else {
			errMsg := common.NewErrorMessage("Server", client.GetNickname(), "Room not found")
			client.SendMessage(errMsg)
		}

	case common.RoomDelete:
		if room, exists := s.roomManager.GetRoom(msg.Room); exists {
			// Check if user is the room creator
			if room.Creator != client.GetNickname() {
				errMsg := common.NewErrorMessage("Server", client.GetNickname(), "Only the room creator can delete the room")
				client.SendMessage(errMsg)
				return
			}

			// Notify all members about room deletion
			deleteMsg := common.NewTextMessage("Server", "", fmt.Sprintf("Room '%s' has been deleted by the creator", room.Name))
			deleteMsg.Room = msg.Room
			s.roomManager.BroadcastToRoom(s, msg.Room, deleteMsg)

			// Send leave confirmation to all members
			members := room.GetMembers()
			for _, member := range members {
				if memberClient, ok := s.GetClient(member); ok {
					memberClient.RemoveRoom(msg.Room)
					leaveMsg := &common.Message{
						Type:     common.TypeRoom,
						Action:   common.RoomLeaveConfirm,
						Room:     room.ID,
						RoomName: room.Name,
						Content:  fmt.Sprintf("Room '%s' has been deleted", room.Name),
					}
					memberClient.SendMessage(leaveMsg)
				}
			}

			// Remove the room
			// Release the creator's room quota - RateLimiter.RemoveRoom was never
			// called from anywhere, so the roomsPerUser counter only grew and after
			// MaxRoomsPerUser the user could no longer create a room, even after
			// deleting every room they owned.
			s.rateLimiter.RemoveRoom(room.Creator)
			s.roomManager.RemoveRoom(msg.Room)

			// Confirm to the creator
			confirmMsg := common.NewTextMessage("Server", client.GetNickname(), fmt.Sprintf("Room '%s' has been deleted", room.Name))
			client.SendMessage(confirmMsg)
		} else {
			errMsg := common.NewErrorMessage("Server", client.GetNickname(), "Room not found")
			client.SendMessage(errMsg)
		}

	case common.RoomSetTopic:
		if room, exists := s.roomManager.GetRoom(msg.Room); exists {
			// Check if user is a member
			if !room.IsMember(client.GetNickname()) {
				errMsg := common.NewErrorMessage("Server", client.GetNickname(), "You must be a member to set the room topic")
				client.SendMessage(errMsg)
				return
			}

			// Set the topic
			room.SetDescription(msg.Content)

			// Notify all room members
			topicMsg := common.NewTextMessage("Server", "", fmt.Sprintf("%s set the room topic to: %s", client.GetNickname(), msg.Content))
			topicMsg.Room = msg.Room
			s.roomManager.BroadcastToRoom(s, msg.Room, topicMsg)

			// Confirm to the setter
			confirmMsg := common.NewTextMessage("Server", client.GetNickname(), "Room topic updated")
			client.SendMessage(confirmMsg)
		} else {
			errMsg := common.NewErrorMessage("Server", client.GetNickname(), "Room not found")
			client.SendMessage(errMsg)
		}
	}
}

// handleInviteMessage handles room invitations
func (s *Server) handleInviteMessage(client *Client, msg *common.Message) {
	room, exists := s.roomManager.GetRoom(msg.Room)
	if !exists {
		errMsg := common.NewErrorMessage("Server", client.GetNickname(), "Room not found")
		client.SendMessage(errMsg)
		return
	}

	// Check if sender is room member
	if !room.IsMember(client.GetNickname()) {
		errMsg := common.NewErrorMessage("Server", client.GetNickname(), "You are not a member of this room")
		client.SendMessage(errMsg)
		return
	}

	// Send invitation to recipient
	if recipient, ok := s.GetClient(msg.Recipient); ok {
		room.InviteUser(msg.Recipient)

		inviteMsg := &common.Message{
			Type:      common.TypeInvite,
			Sender:    client.GetNickname(),
			Recipient: msg.Recipient,
			Room:      msg.Room,
			Content:   fmt.Sprintf("%s invited you to join room '%s'", client.GetNickname(), room.Name),
		}
		recipient.SendMessage(inviteMsg)

		// Confirm to sender
		confirmMsg := common.NewTextMessage("Server", client.GetNickname(), fmt.Sprintf("Invitation sent to %s", msg.Recipient))
		client.SendMessage(confirmMsg)
	} else {
		errMsg := common.NewErrorMessage("Server", client.GetNickname(), fmt.Sprintf("User %s not found", msg.Recipient))
		client.SendMessage(errMsg)
	}
}

// handleInviteResponse handles invitation responses
func (s *Server) handleInviteResponse(client *Client, msg *common.Message) {
	room, exists := s.roomManager.GetRoom(msg.Room)
	if !exists {
		errMsg := common.NewErrorMessage("Server", client.GetNickname(), "Room no longer exists")
		client.SendMessage(errMsg)
		return
	}

	if msg.Content == "accept" && room.IsInvited(client.GetNickname()) {
		room.AddMember(client.GetNickname())
		client.AddRoom(room.ID)

		// Send room info to the joining user
		roomInfo := room.Name
		if desc := room.GetDescription(); desc != "" {
			roomInfo = fmt.Sprintf("%s - Topic: %s", room.Name, desc)
		}
		response := &common.Message{
			Type:    common.TypeRoom,
			Action:  common.RoomJoin,
			Room:    room.ID,
			Content: roomInfo,
		}
		client.SendMessage(response)

		// Notify room members
		joinMsg := common.NewTextMessage("Server", "", fmt.Sprintf("%s has joined the room", client.GetNickname()))
		joinMsg.Room = msg.Room
		s.roomManager.BroadcastToRoom(s, msg.Room, joinMsg)
	} else if msg.Content == "decline" {
		// Remove invitation
		room.mutex.Lock()
		delete(room.Invitations, client.GetNickname())
		room.mutex.Unlock()

		// Confirm decline
		confirmMsg := common.NewTextMessage("Server", client.GetNickname(), "Invitation declined")
		client.SendMessage(confirmMsg)
	}
}

// handleFileTransferInit initiates a file transfer
func (s *Server) handleFileTransferInit(client *Client, msg *common.Message) {
	// Check rate limit for file transfers
	if err := s.rateLimiter.CanStartFileTransfer(client.GetNickname()); err != nil {
		errMsg := common.NewErrorMessage("Server", client.GetNickname(), err.Error())
		client.SendMessage(errMsg)
		return
	}

	// Validate file name
	if err := ValidateFileName(msg.Filename); err != nil {
		errMsg := common.NewErrorMessage("Server", client.GetNickname(), err.Error())
		client.SendMessage(errMsg)
		return
	}

	// Validate file size
	if err := ValidateFileSize(msg.Filesize); err != nil {
		errMsg := common.NewErrorMessage("Server", client.GetNickname(), err.Error())
		client.SendMessage(errMsg)
		return
	}

	recipient, exists := s.GetClient(msg.Recipient)
	if !exists {
		errMsg := common.NewErrorMessage("Server", client.GetNickname(), fmt.Sprintf("User %s not found", msg.Recipient))
		client.SendMessage(errMsg)
		return
	}

	// Create file transfer record
	ft := &common.FileTransfer{
		FileID:         msg.FileID,
		Filename:       msg.Filename,
		Filesize:       msg.Filesize,
		Sender:         client.GetNickname(),
		Recipient:      msg.Recipient,
		TotalChunks:    msg.TotalChunks,
		ReceivedChunks: make(map[int][]byte),
		StartTime:      msg.Timestamp,
	}

	s.fileTransfers.Store(msg.FileID, ft)
	s.rateLimiter.AddFileTransfer(client.GetNickname())

	// Forward to recipient
	recipient.SendMessage(msg)
}

// handleFileChunk handles file chunk transfer
func (s *Server) handleFileChunk(client *Client, msg *common.Message) {
	value, exists := s.fileTransfers.Load(msg.FileID)
	if !exists {
		return
	}

	ft := value.(*common.FileTransfer)

	// Only the transfer's sender may push its chunks. Without this any registered
	// client who knew a FileID could inject data into someone else's transfer.
	if client.GetNickname() != ft.Sender {
		errMsg := common.NewErrorMessage("Server", client.GetNickname(), "You are not the sender of this transfer")
		client.SendMessage(errMsg)
		return
	}

	// The server only RELAYS chunks - it need not hold the file contents. The
	// earlier ft.AddChunk(msg.ChunkNum, msg.Data) buffered whole files in server
	// memory (up to MaxFileSize per transfer) even though nobody read those bytes.
	// A chunk counter is enough to detect the end.
	ft.MarkChunkReceived(msg.ChunkNum)

	// Forward to recipient
	if recipient, ok := s.GetClient(ft.Recipient); ok {
		recipient.SendMessage(msg)

		// Check if transfer is complete
		if ft.IsComplete() {
			completeMsg := &common.Message{
				Type:     common.TypeFileComplete,
				FileID:   msg.FileID,
				Filename: ft.Filename,
			}
			recipient.SendMessage(completeMsg)
			client.SendMessage(completeMsg)

			// Clean up. The quota release used to be called ONLY from the timeout
			// path (cleanup.go), so after FileTransfersPerUser SUCCESSFUL transfers
			// the user could not send anything more.
			s.fileTransfers.Delete(msg.FileID)
			s.rateLimiter.RemoveFileTransfer(ft.Sender)
		}
	}
}

// handleShutdown handles graceful server shutdown
func (s *Server) handleShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	common.Info("Shutting down server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), common.ShutdownTimeout)
	defer cancel()

	// Notify all clients
	shutdownMsg := common.NewBroadcastMessage("Server", "Server is shutting down")
	s.BroadcastMessage(shutdownMsg, "")

	// Give clients time to receive the message
	time.Sleep(100 * time.Millisecond)

	// Close all client connections
	connClosed := make(chan bool)
	go func() {
		s.clients.Range(func(key, value any) bool {
			client := value.(*Client)
			client.Close()
			return true
		})
		close(connClosed)
	}()

	// Wait for connections to close or timeout
	select {
	case <-connClosed:
		common.Info("All client connections closed")
	case <-ctx.Done():
		common.Warn("Shutdown timeout exceeded, forcing shutdown")
	}

	// Stop cleanup manager
	s.cleanupManager.Stop()

	// Stop rate limiter
	s.rateLimiter.Stop()

	// ORDER: signal shutdown first, THEN close the listener. The other way round
	// the accept loop got a "use of closed network connection" error, and its
	// default branch (because s.shutdown was still open) logged it in a tight loop,
	// pinning a core at 100%.
	close(s.shutdown)

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	common.Info("Server shutdown complete")
}

func main() {
	port := flag.String("port", "8080", "Server port")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Initialize logging
	level := common.LogInfo
	switch *logLevel {
	case "debug":
		level = common.LogDebug
	case "warn":
		level = common.LogWarn
	case "error":
		level = common.LogError
	}

	if err := common.InitLogger("server.log", level); err != nil {
		log.Printf("Failed to initialize logger: %v", err)
	}
	// GlobalLogger stays nil when InitLogger fails - an unconditional
	// GlobalLogger.Close() would then dereference nil and panic on exit.
	defer func() {
		if common.GlobalLogger != nil {
			common.GlobalLogger.Close()
		}
	}()

	common.Info("Starting TCP Chat Server on port %s", *port)

	server := NewServer()
	if err := server.Start(*port); err != nil {
		common.Fatal("Server error: %v", err)
	}
}
