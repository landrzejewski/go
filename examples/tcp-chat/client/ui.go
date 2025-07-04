package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"

	"tcp-chat/common"
)

// UI handles terminal user interface
type UI struct {
	conn         *Connection
	fileTransfer *FileTransfer
	rooms        map[string]string // roomID -> roomName
	users        []string
	running      bool
	mutex        sync.RWMutex
}

// NewUI creates a new UI instance
func NewUI(conn *Connection, ft *FileTransfer) *UI {
	return &UI{
		conn:         conn,
		fileTransfer: ft,
		rooms:        make(map[string]string),
		running:      true,
	}
}

// Start starts the UI
func (ui *UI) Start() {
	// Clear screen and show welcome
	clearScreen()
	ui.showWelcome()

	// Start message receiver
	go ui.receiveMessages()

	// Start input handler
	ui.handleInput()
}

// Use platform-specific clearScreen function defined in platform_*.go files

// showWelcome displays welcome message
func (ui *UI) showWelcome() {
	fmt.Println("=================================")
	fmt.Println("   TCP Chat Client")
	fmt.Println("=================================")
	fmt.Printf("Connected as: %s\n", ui.conn.nickname)
	fmt.Println("\nCommands:")
	fmt.Println("  /help                    - Show help")
	fmt.Println("  /users                   - List online users")
	fmt.Println("  /msg <nick> <message>    - Send private message")
	fmt.Println("  /file <nick> <filepath>  - Send file")
	fmt.Println("  /status <active|busy|invisible> - Change status")
	fmt.Println("  /room create <name>      - Create private room")
	fmt.Println("  /room invite <id> <nick> - Invite to room")
	fmt.Println("  /room accept <id>        - Accept room invitation")
	fmt.Println("  /room decline <id>       - Decline room invitation")
	fmt.Println("  /room msg <id> <message> - Message to room")
	fmt.Println("  /room list               - List your rooms")
	fmt.Println("  /room leave <id>         - Leave a room")
	fmt.Println("  /transfers               - Show file transfers")
	fmt.Println("  /quit                    - Exit")
	fmt.Println("\nType messages without '/' to broadcast to all users")
	fmt.Println("=================================\n")
}

// handleInput handles user input
func (ui *UI) handleInput() {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() && ui.running {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "/") {
			ui.handleCommand(input)
		} else {
			// Send broadcast message
			ui.conn.SendBroadcastMessage(input)
		}
	}
}

// handleCommand handles slash commands
func (ui *UI) handleCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	command := strings.ToLower(parts[0])

	switch command {
	case "/help":
		ui.showWelcome()

	case "/users":
		ui.showUsers()

	case "/msg":
		if len(parts) < 3 {
			fmt.Println("Usage: /msg <nickname> <message>")
			return
		}
		recipient := parts[1]
		message := strings.Join(parts[2:], " ")
		ui.conn.SendTextMessage(recipient, message)

	case "/file":
		if len(parts) < 3 {
			fmt.Println("Usage: /file <nickname> <filepath>")
			return
		}
		recipient := parts[1]
		filepath := strings.Join(parts[2:], " ")

		if err := ui.fileTransfer.SendFile(recipient, filepath); err != nil {
			fmt.Printf("Error sending file: %v\n", err)
		} else {
			fmt.Printf("Sending file to %s...\n", recipient)
		}

	case "/status":
		if len(parts) < 2 {
			fmt.Println("Usage: /status <active|busy|invisible>")
			return
		}

		var status common.UserStatus
		switch strings.ToLower(parts[1]) {
		case "active":
			status = common.StatusActive
		case "busy":
			status = common.StatusBusy
		case "invisible":
			status = common.StatusInvisible
		default:
			fmt.Println("Invalid status. Use: active, busy, or invisible")
			return
		}

		ui.conn.ChangeStatus(status)
		fmt.Printf("Status changed to: %s\n", status)

	case "/room":
		ui.handleRoomCommand(parts[1:])

	case "/transfers":
		ui.showTransfers()

	case "/quit":
		ui.running = false
		ui.conn.Disconnect()
		fmt.Println("Goodbye!")
		os.Exit(0)

	default:
		fmt.Printf("Unknown command: %s\n", command)
	}
}

// handleRoomCommand handles room-related commands
func (ui *UI) handleRoomCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: /room <create|invite|accept|decline|msg|list|leave|members|kick|delete|topic> ...")
		return
	}

	subcommand := strings.ToLower(args[0])

	switch subcommand {
	case "create":
		if len(args) < 2 {
			fmt.Println("Usage: /room create <name>")
			return
		}
		roomName := strings.Join(args[1:], " ")
		ui.conn.CreateRoom(roomName)

	case "invite":
		if len(args) < 3 {
			fmt.Println("Usage: /room invite <room_id> <nickname>")
			return
		}
		roomID := args[1]
		nickname := args[2]
		ui.conn.InviteToRoom(roomID, nickname)

	case "accept":
		if len(args) < 2 {
			fmt.Println("Usage: /room accept <room_id>")
			return
		}
		roomID := args[1]
		ui.conn.RespondToInvite(roomID, true)
		fmt.Printf("Accepted invitation to room %s\n", roomID)

	case "decline":
		if len(args) < 2 {
			fmt.Println("Usage: /room decline <room_id>")
			return
		}
		roomID := args[1]
		ui.conn.RespondToInvite(roomID, false)
		fmt.Printf("Declined invitation to room %s\n", roomID)

	case "msg":
		if len(args) < 3 {
			fmt.Println("Usage: /room msg <room_id> <message>")
			return
		}
		roomID := args[1]
		message := strings.Join(args[2:], " ")
		ui.conn.SendRoomMessage(roomID, message)

	case "list":
		ui.showRooms()

	case "leave":
		if len(args) < 2 {
			fmt.Println("Usage: /room leave <room_id>")
			return
		}
		roomID := args[1]
		ui.conn.LeaveRoom(roomID)
		// Don't delete here - wait for server confirmation

	case "members":
		if len(args) < 2 {
			fmt.Println("Usage: /room members <room_id>")
			return
		}
		roomID := args[1]
		ui.conn.GetRoomMembers(roomID)

	case "kick":
		if len(args) < 3 {
			fmt.Println("Usage: /room kick <room_id> <nickname>")
			return
		}
		roomID := args[1]
		nickname := args[2]
		ui.conn.KickFromRoom(roomID, nickname)

	case "delete":
		if len(args) < 2 {
			fmt.Println("Usage: /room delete <room_id>")
			return
		}
		roomID := args[1]
		ui.conn.DeleteRoom(roomID)

	case "topic":
		if len(args) < 3 {
			fmt.Println("Usage: /room topic <room_id> <description>")
			return
		}
		roomID := args[1]
		description := strings.Join(args[2:], " ")
		ui.conn.SetRoomTopic(roomID, description)

	default:
		fmt.Printf("Unknown room command: %s\n", subcommand)
	}
}

// receiveMessages handles incoming messages
func (ui *UI) receiveMessages() {
	for msg := range ui.conn.GetMessages() {
		ui.handleMessage(msg)
	}
}

// handleMessage processes incoming messages
func (ui *UI) handleMessage(msg *common.Message) {
	timestamp := msg.Timestamp.Format("15:04:05")

	switch msg.Type {
	case common.TypeText:
		if msg.Room != "" {
			// Room message
			ui.mutex.RLock()
			roomName := ui.rooms[msg.Room]
			ui.mutex.RUnlock()
			if roomName == "" {
				roomName = msg.Room
			}
			fmt.Printf("[%s] [Room: %s] %s: %s\n", timestamp, roomName, msg.Sender, msg.Content)
		} else if msg.Recipient == ui.conn.nickname {
			// Private message
			fmt.Printf("[%s] [Private] %s: %s\n", timestamp, msg.Sender, msg.Content)
		} else if msg.Recipient == "*" || msg.Recipient == "" {
			// Broadcast message
			fmt.Printf("[%s] %s: %s\n", timestamp, msg.Sender, msg.Content)
		}

	case common.TypeUserList:
		ui.users = msg.Users

	case common.TypeStatus:
		fmt.Printf("[%s] %s changed status to %s\n", timestamp, msg.Sender, msg.Status)

	case common.TypeRoom:
		if msg.Action == common.RoomCreate {
			ui.mutex.Lock()
			ui.rooms[msg.Room] = msg.Content
			ui.mutex.Unlock()
			fmt.Printf("[%s] %s (ID: %s)\n", timestamp, msg.Content, msg.Room)
		} else if msg.Action == common.RoomJoin {
			// Add room to our list when we join
			ui.mutex.Lock()
			ui.rooms[msg.Room] = msg.Content
			ui.mutex.Unlock()
			fmt.Printf("[%s] Joined room '%s' (ID: %s)\n", timestamp, msg.Content, msg.Room)
		} else if msg.Action == common.RoomMembers {
			// Display room members
			fmt.Printf("[%s] %s\n", timestamp, msg.Content)
		} else if msg.Action == common.RoomLeaveConfirm {
			// Remove room from local state after confirmation
			ui.mutex.Lock()
			delete(ui.rooms, msg.Room)
			ui.mutex.Unlock()
			fmt.Printf("[%s] Left room '%s'\n", timestamp, msg.Content)
		}

	case common.TypeInvite:
		fmt.Printf("\n[%s] %s\n", timestamp, msg.Content)
		fmt.Printf("Type '/room accept %s' to accept or '/room decline %s' to decline\n", msg.Room, msg.Room)

	case common.TypeFile:
		fmt.Printf("[%s] %s is sending you file: %s (%s)\n",
			timestamp, msg.Sender, msg.Filename, formatFileSize(msg.Filesize))

	case common.TypeFileChunk:
		// Progress update
		fmt.Printf("\rFile transfer: %s - %s", msg.Filename, msg.Content)

	case common.TypeFileComplete:
		fmt.Printf("\n[%s] File received: %s\n", timestamp, msg.Filename)
		if err := ui.fileTransfer.ReceiveFile(msg.FileID); err != nil {
			fmt.Printf("Error saving file: %v\n", err)
		} else {
			fmt.Printf("File saved to downloads/%s\n", msg.Filename)
		}

	case common.TypeError:
		fmt.Printf("[%s] Error: %s\n", timestamp, msg.Error)

	default:
		// System messages
		if msg.Sender == "Server" {
			fmt.Printf("[%s] %s\n", timestamp, msg.Content)
		}
	}
}

// showUsers displays online users
func (ui *UI) showUsers() {
	fmt.Println("\n=== Online Users ===")
	for _, user := range ui.users {
		parts := strings.Split(user, ":")
		if len(parts) == 2 {
			fmt.Printf("  %s (%s)\n", parts[0], parts[1])
		} else {
			fmt.Printf("  %s\n", user)
		}
	}
	fmt.Println("==================\n")
}

// showRooms displays user's rooms
func (ui *UI) showRooms() {
	fmt.Println("\n=== Your Rooms ===")
	ui.mutex.RLock()
	defer ui.mutex.RUnlock()
	if len(ui.rooms) == 0 {
		fmt.Println("  No rooms joined")
	} else {
		for id, info := range ui.rooms {
			fmt.Printf("  %s: %s\n", id, info)
		}
	}
	fmt.Println("==================\n")
}

// showTransfers displays active file transfers
func (ui *UI) showTransfers() {
	transfers := ui.fileTransfer.GetTransferProgress()

	fmt.Println("\n=== File Transfers ===")
	if len(transfers) == 0 {
		fmt.Println("  No active transfers")
	} else {
		for _, transfer := range transfers {
			fmt.Printf("  %s\n", transfer)
		}
	}
	fmt.Println("===================\n")
}
