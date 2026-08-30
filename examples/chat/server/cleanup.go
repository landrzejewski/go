package main

import (
	"log"
	"time"

	"tcp-chat/common"
)

// CleanupManager handles periodic cleanup of resources
type CleanupManager struct {
	server   *Server
	ticker   *time.Ticker
	stopChan chan bool
}

// NewCleanupManager creates a new cleanup manager
func NewCleanupManager(server *Server) *CleanupManager {
	return &CleanupManager{
		server:   server,
		ticker:   time.NewTicker(1 * time.Minute),
		stopChan: make(chan bool),
	}
}

// Start begins the cleanup routine
func (cm *CleanupManager) Start() {
	go cm.run()
}

// Stop stops the cleanup routine
func (cm *CleanupManager) Stop() {
	cm.ticker.Stop()
	close(cm.stopChan)
}

// run executes periodic cleanup tasks
func (cm *CleanupManager) run() {
	for {
		select {
		case <-cm.ticker.C:
			cm.cleanupFileTransfers()
			cm.cleanupEmptyRooms()
		case <-cm.stopChan:
			return
		}
	}
}

// cleanupFileTransfers removes stale file transfers
func (cm *CleanupManager) cleanupFileTransfers() {
	now := time.Now()
	var toDelete []string

	// Find stale transfers
	cm.server.fileTransfers.Range(func(key, value interface{}) bool {
		fileID := key.(string)
		ft := value.(*common.FileTransfer)

		// Check if transfer is older than timeout
		if now.Sub(ft.StartTime) > common.FileTransferTimeout {
			toDelete = append(toDelete, fileID)
			log.Printf("Cleaning up stale file transfer: %s", fileID)

			// Notify sender about timeout
			if sender, ok := cm.server.GetClient(ft.Sender); ok {
				errMsg := common.NewErrorMessage("Server", ft.Sender,
					"File transfer timed out: "+ft.Filename)
				sender.SendMessage(errMsg)
			}

			// Notify recipient about timeout
			if recipient, ok := cm.server.GetClient(ft.Recipient); ok {
				errMsg := common.NewErrorMessage("Server", ft.Recipient,
					"File transfer timed out: "+ft.Filename)
				recipient.SendMessage(errMsg)
			}

			// Clean up rate limiter
			cm.server.rateLimiter.RemoveFileTransfer(ft.Sender)
		}
		return true
	})

	// Delete stale transfers
	for _, fileID := range toDelete {
		cm.server.fileTransfers.Delete(fileID)
	}
}

// cleanupEmptyRooms usuwa pokoje, które stoją puste dłużej niż EmptyRoomTimeout.
//
// Liczy się czas OD OPUSTOSZENIA (Room.LastEmptyAt), a nie od utworzenia.
// Poprzednia wersja porównywała CreatedAt, więc pokój sprzed 31 minut znikał
// w chwili wyjścia ostatniego członka, a pokój sprzed minuty przeżywał pusty -
// mimo że komentarz obiecywał coś zupełnie innego.
func (cm *CleanupManager) cleanupEmptyRooms() {
	now := time.Now()
	var toDelete []string
	creators := make(map[string]string)

	cm.server.roomManager.mutex.RLock()
	for roomID, room := range cm.server.roomManager.rooms {
		room.mutex.RLock()
		memberCount := len(room.Members)
		lastEmptyAt := room.LastEmptyAt
		creator := room.Creator
		room.mutex.RUnlock()

		if memberCount == 0 && !lastEmptyAt.IsZero() && now.Sub(lastEmptyAt) > common.EmptyRoomTimeout {
			toDelete = append(toDelete, roomID)
			creators[roomID] = creator
			log.Printf("Cleaning up empty room: %s (%s)", room.Name, roomID)
		}
	}
	cm.server.roomManager.mutex.RUnlock()

	// Delete empty rooms
	for _, roomID := range toDelete {
		// Zwolnienie limitu pokoi twórcy - patrz komentarz przy RoomDelete.
		cm.server.rateLimiter.RemoveRoom(creators[roomID])
		cm.server.roomManager.RemoveRoom(roomID)
	}
}
