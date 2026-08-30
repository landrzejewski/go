package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"training.pl/go/examples/chat/common"
)

// ChunkSize is defined in common/constants.go as FileChunkSize

// FileTransfer manages file transfers
type FileTransfer struct {
	conn *Connection
}

// NewFileTransfer creates a new file transfer manager
func NewFileTransfer(conn *Connection) *FileTransfer {
	return &FileTransfer{
		conn: conn,
	}
}

// SendFile sends a file to a recipient
func (ft *FileTransfer) SendFile(recipient, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	// NOTE: there is deliberately NO `defer file.Close()` here. SendFile returns
	// immediately after starting the sendFileChunks goroutine, so a defer would close the file
	// before that goroutine had read anything ("file already closed") and Close
	// would run twice. The goroutine owns the descriptor; on the error paths below
	// we close it explicitly.

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to get file info: %v", err)
	}

	// Check if it's a directory
	if fileInfo.IsDir() {
		file.Close()
		return fmt.Errorf("cannot send directory as file")
	}

	// Generate file ID
	fileID := generateFileID()
	filename := filepath.Base(filePath)
	filesize := fileInfo.Size()

	// Validate file size
	if filesize > common.MaxFileSize {
		file.Close()
		return fmt.Errorf("file size exceeds maximum allowed size of %d bytes", common.MaxFileSize)
	}

	totalChunks := int(filesize / common.FileChunkSize)
	if filesize%common.FileChunkSize != 0 {
		totalChunks++
	}

	// Create file transfer record
	transfer := &FileTransferProgress{
		FileID:      fileID,
		Filename:    filename,
		Filesize:    filesize,
		IsIncoming:  false,
		StartTime:   time.Now(),
		TotalChunks: totalChunks,
	}

	ft.conn.mutex.Lock()
	ft.conn.fileTransfers[fileID] = transfer
	ft.conn.mutex.Unlock()

	// Send file init message
	initMsg := &common.Message{
		Type:        common.TypeFile,
		Recipient:   recipient,
		FileID:      fileID,
		Filename:    filename,
		Filesize:    filesize,
		TotalChunks: totalChunks,
		Timestamp:   time.Now(),
	}

	ft.conn.sendChan <- initMsg

	// Start sending chunks
	go ft.sendFileChunks(file, fileID, recipient, totalChunks)

	return nil
}

// sendFileChunks sends file chunks
// sendFileChunks sends file chunks. This goroutine is the sole owner of the file
// descriptor, so it is the one that closes it.
func (ft *FileTransfer) sendFileChunks(file *os.File, fileID, recipient string, totalChunks int) {
	defer file.Close()

	buffer := make([]byte, common.FileChunkSize)
	chunkNum := 0

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			ft.notifyError(fileID, fmt.Sprintf("Read error: %v", err))
			return
		}

		if n == 0 {
			break
		}

		// Copy the chunk. The message is only QUEUED - writePump serialises it
		// later, in another goroutine, by which time the loop has overwritten
		// `buffer` with the next read. Sharing the buffer was a data race and
		// corrupted the file on the receiving side (the time.Sleep below merely
		// masked it).
		chunk := make([]byte, n)
		copy(chunk, buffer[:n])

		// Send chunk
		chunkMsg := &common.Message{
			Type:        common.TypeFileChunk,
			Recipient:   recipient,
			FileID:      fileID,
			ChunkNum:    chunkNum,
			TotalChunks: totalChunks,
			Data:        chunk,
			Timestamp:   time.Now(),
		}

		ft.conn.sendChan <- chunkMsg

		// Update progress
		ft.updateProgress(fileID, chunkNum, totalChunks)

		chunkNum++

		// Small delay to avoid overwhelming the connection
		time.Sleep(10 * time.Millisecond)
	}

	// File transfer complete
	ft.notifyComplete(fileID)
}

// IsIncoming reports whether a transfer is an INCOMING one. The server sends
// FileComplete to both sides, but only the receiver has anything to write.
func (ft *FileTransfer) IsIncoming(fileID string) bool {
	ft.conn.mutex.RLock()
	defer ft.conn.mutex.RUnlock()
	transfer, exists := ft.conn.fileTransfers[fileID]
	return exists && transfer.IsIncoming
}

// ReceiveFile saves a received file
func (ft *FileTransfer) ReceiveFile(fileID string) error {
	ft.conn.mutex.RLock()
	transfer, exists := ft.conn.fileTransfers[fileID]
	ft.conn.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("file transfer not found")
	}

	// Create downloads directory
	downloadDir := filepath.Join(".", "downloads")
	if err := os.MkdirAll(downloadDir, common.GetDirMode()); err != nil {
		return fmt.Errorf("failed to create download directory: %v", err)
	}

	// Sanitize filename to prevent path traversal attacks
	filename := filepath.Base(transfer.Filename)
	if filename == "." || filename == ".." || filename == "/" || filename == "" {
		return fmt.Errorf("invalid filename: %s", transfer.Filename)
	}

	// Create file
	filePath := filepath.Join(downloadDir, filename)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	// Snapshot the chunks under the lock. The readPump goroutine
	// (Connection.handleFileChunk) writes to the same map while ReceiveFile is
	// called from the UI goroutine - reading and writing a map at once is a race,
	// a w skrajnym przypadku "fatal error: concurrent map read and map write".
	transfer.mutex.Lock()
	totalChunks := transfer.TotalChunks
	chunks := make([][]byte, totalChunks)
	missing := -1
	for i := 0; i < totalChunks; i++ {
		chunk, exists := transfer.Chunks[i]
		if !exists {
			missing = i
			break
		}
		chunks[i] = chunk
	}
	transfer.mutex.Unlock()

	if missing >= 0 {
		return fmt.Errorf("missing chunk %d", missing)
	}

	// Write chunks in order
	for _, chunk := range chunks {
		if _, err := file.Write(chunk); err != nil {
			return fmt.Errorf("failed to write chunk: %v", err)
		}
	}

	// Clean up
	ft.conn.mutex.Lock()
	delete(ft.conn.fileTransfers, fileID)
	ft.conn.mutex.Unlock()

	return nil
}

// updateProgress updates transfer progress
func (ft *FileTransfer) updateProgress(fileID string, chunkNum, totalChunks int) {
	ft.conn.mutex.Lock()
	defer ft.conn.mutex.Unlock()

	if transfer, exists := ft.conn.fileTransfers[fileID]; exists {
		transfer.Progress = float64(chunkNum+1) / float64(totalChunks) * 100
	}
}

// notifyComplete notifies completion
func (ft *FileTransfer) notifyComplete(fileID string) {
	ft.conn.mutex.Lock()
	defer ft.conn.mutex.Unlock()

	if transfer, exists := ft.conn.fileTransfers[fileID]; exists {
		duration := time.Since(transfer.StartTime)
		speed := float64(transfer.Filesize) / duration.Seconds() / 1024 / 1024 // MB/s

		fmt.Printf("\nFile transfer complete: %s (%.2f MB/s)\n", transfer.Filename, speed)
		delete(ft.conn.fileTransfers, fileID)
	}
}

// notifyError notifies transfer error
func (ft *FileTransfer) notifyError(fileID, error string) {
	ft.conn.mutex.Lock()
	defer ft.conn.mutex.Unlock()

	if transfer, exists := ft.conn.fileTransfers[fileID]; exists {
		fmt.Printf("\nFile transfer error: %s - %s\n", transfer.Filename, error)
		delete(ft.conn.fileTransfers, fileID)
	}
}

// GetTransferProgress returns current transfer progress
func (ft *FileTransfer) GetTransferProgress() []string {
	ft.conn.mutex.RLock()
	defer ft.conn.mutex.RUnlock()

	var progress []string
	for _, transfer := range ft.conn.fileTransfers {
		direction := "↓"
		if !transfer.IsIncoming {
			direction = "↑"
		}

		status := fmt.Sprintf("%s %s: %.1f%% (%s)",
			direction,
			transfer.Filename,
			transfer.Progress,
			formatFileSize(transfer.Filesize))

		progress = append(progress, status)
	}

	return progress
}

// generateFileID generates a unique file ID
func generateFileID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// formatFileSize formats file size in human readable format
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
