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

	"tcp-chat/common"
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

		// Sprawdzenie limitu i rejestracja połączenia w jednej, atomowej operacji.
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
	// Slot połączenia zajęto już w pętli accept (TryAddConnection), więc tutaj
	// wystarczy zagwarantować jego zwolnienie - BEZWARUNKOWO, także gdy klient
	// rozłączy się przed rejestracją nicku.
	//
	// Wcześniej zwolnienie działo się dopiero w UnregisterClient, PO wczesnym
	// return dla pustego nicku - więc każdy skan portów i każdy odrzucony
	// klient zabierał slot na stałe. Po MaxConnections takich połączeń serwer
	// odmawiał już wszystkim.
	defer s.rateLimiter.RemoveConnection(conn.RemoteAddr())

	// Set initial read/write deadlines
	conn.SetReadDeadline(time.Now().Add(common.ReadTimeout))
	conn.SetWriteDeadline(time.Now().Add(common.WriteTimeout))

	client := NewClient(conn, s)
	client.RemoteAddr = conn.RemoteAddr().String()
	common.Info("New connection from %s", conn.RemoteAddr())

	// Start blokuje aż do zakończenia obu pomp, żeby defer powyżej wykonał się
	// dokładnie wtedy, gdy połączenie faktycznie się kończy.
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

	// Nickname jest czytane przez goroutines innych klientów - tylko przez setter.
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
	// Slot połączenia zwalnia teraz defer w handleNewConnection - bezwarunkowo
	// i dla każdego gniazda, także takiego, które nigdy się nie zarejestrowało.
	// Tutaj sprzątamy wyłącznie stan związany z NICKIEM.
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
	s.clients.Range(func(key, value interface{}) bool {
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

	s.clients.Range(func(key, value interface{}) bool {
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

	// Wszystko poza CONNECT wymaga rejestracji. Bez tego anonimowy klient mógł
	// rozgłaszać wiadomości, tworzyć pokoje i uruchamiać transfery plików,
	// a wszystkie mapy limitów kluczowały się pustym nickiem, więc niezalogowani
	// dzielili jeden wspólny kubełek.
	if msg.Type != common.TypeConnect && nickname == "" {
		return common.NewChatError(common.ErrUnauthorized, "you must register with CONNECT first")
	}

	switch msg.Type {
	case common.TypeConnect:
		// Ponowny CONNECT po udanej rejestracji zostawiałby stary nick w mapie
		// klientów wskazujący na to samo połączenie - "duch" na liście użytkowników.
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

			// Wiadomość jest tylko KOLEJKOWANA, więc natychmiastowe Conn.Close()
			// prawie zawsze ucinało ją przed wysłaniem i odrzucony użytkownik
			// widział gołe rozłączenie bez powodu. Dajemy WritePump chwilę na
			// opróżnienie kolejki.
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
		// KOLEJNOŚĆ MA ZNACZENIE: test na Room musi być PIERWSZY. Klient
		// (Connection.SendRoomMessage) ustawia tylko pole Room i zostawia
		// Recipient puste, więc przy poprzedniej kolejności każda wiadomość
		// pokojowa wpadała w gałąź `Recipient == ""` i była rozgłaszana do
		// WSZYSTKICH zalogowanych - a sprawdzenie członkostwa poniżej było
		// martwym kodem. Prywatne pokoje nie były więc w ogóle prywatne.
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
		// Keep-alive od klienta. Nie wymaga odpowiedzi - samo dotarcie
		// wiadomości odświeżyło już read deadline w ReadPump. Bez tej gałęzi
		// keep-alive wpadał w default i wracał jako błąd "unknown message type".

	case common.TypeDisconnect:
		// Klient wysyła to przy Ctrl-C (client/main.go). Bez tej gałęzi wpadało
		// w default i odbijało się błędem "unknown message type: DISCONNECT".
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
			// Dołączyć może twórca albo osoba zaproszona. Wcześniej ta gałąź
			// nie sprawdzała NICZEGO, więc znajomość identyfikatora pokoju
			// wystarczyła, by wejść do cudzego "prywatnego" pokoju - podczas
			// gdy handleInviteResponse starannie weryfikuje IsInvited.
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
			// Zwalniamy limit pokoi twórcy - RateLimiter.RemoveRoom nie było
			// wołane znikąd, więc licznik roomsPerUser tylko rósł i po
			// MaxRoomsPerUser użytkownik nie mógł już utworzyć pokoju,
			// nawet po skasowaniu wszystkich swoich.
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

	// Tylko nadawca transferu może dosyłać jego fragmenty. Bez tego dowolny
	// zalogowany klient, który zna FileID, mógł wstrzykiwać dane w cudzy transfer.
	if client.GetNickname() != ft.Sender {
		errMsg := common.NewErrorMessage("Server", client.GetNickname(), "You are not the sender of this transfer")
		client.SendMessage(errMsg)
		return
	}

	// Serwer tylko PRZEKAZUJE fragmenty - nie musi trzymać zawartości pliku.
	// Wcześniejsze ft.AddChunk(msg.ChunkNum, msg.Data) buforowało w pamięci
	// serwera całe pliki (do MaxFileSize na transfer), choć nikt tych bajtów
	// nie odczytywał. Do wykrycia końca wystarczy licznik fragmentów.
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

			// Clean up. Zwolnienie limitu było wołane WYŁĄCZNIE ze ścieżki
			// timeoutu (cleanup.go), więc po FileTransfersPerUser UDANYCH
			// transferach użytkownik nie mógł już wysłać nic więcej.
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
		s.clients.Range(func(key, value interface{}) bool {
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

	// KOLEJNOŚĆ: najpierw sygnał shutdown, POTEM zamknięcie listenera.
	// Odwrotnie pętla accept dostawała błąd "use of closed network connection",
	// a jej gałąź default (bo s.shutdown było jeszcze otwarte) logowała go
	// w kółko, zajmując rdzeń na 100%.
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
	// GlobalLogger zostaje nil, gdy InitLogger zawiedzie - bezwarunkowe
	// GlobalLogger.Close() dereferencjonowało wtedy nil i panikowało przy wyjściu.
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
