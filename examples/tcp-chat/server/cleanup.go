package main

import (
	"log"
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

// cleanupEmptyRooms removes rooms that have been empty for too long
func (cm *CleanupManager) cleanupEmptyRooms() {
	now := time.Now()
	var toDelete []string

	cm.server.roomManager.mutex.RLock()
	for roomID, room := range cm.server.roomManager.rooms {
		room.mutex.RLock()
		memberCount := len(room.Members)
		createdAt := room.CreatedAt
		room.mutex.RUnlock()

		// Remove rooms that are empty and older than timeout
		if memberCount == 0 && now.Sub(createdAt) > common.EmptyRoomTimeout {
			toDelete = append(toDelete, roomID)
			log.Printf("Cleaning up empty room: %s (%s)", room.Name, roomID)
		}
	}
	cm.server.roomManager.mutex.RUnlock()

	// Delete empty rooms
	for _, roomID := range toDelete {
		cm.server.roomManager.RemoveRoom(roomID)
	}
}
