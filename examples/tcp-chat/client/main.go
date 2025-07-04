package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"tcp-chat/common"
	"time"
)

var logFile *os.File

func main() {
	// Parse command line arguments
	serverAddr := flag.String("server", "localhost:8080", "Server address")
	nickname := flag.String("nick", "", "Your nickname")
	flag.Parse()

	// Validate nickname
	if *nickname == "" {
		fmt.Println("Error: Nickname is required")
		fmt.Println("Usage: ./client -nick <your_nickname> [-server <address>]")
		os.Exit(1)
	}

	// Create connection
	conn := NewConnection(*nickname)

	// Create file transfer manager
	ft := NewFileTransfer(conn)

	// Create UI
	ui := NewUI(conn, ft)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")

		// Send disconnect message if connected
		if conn.IsConnected() {
			disconnectMsg := &common.Message{
				Type:    common.TypeDisconnect,
				Sender:  *nickname,
				Content: "Client shutting down",
			}
			conn.sendChan <- disconnectMsg

			// Give message time to send
			time.Sleep(100 * time.Millisecond)
		}

		conn.Disconnect()

		// Close log file
		if logFile != nil {
			logFile.Close()
		}

		fmt.Println("Goodbye!")
		os.Exit(0)
	}()

	// Connect to server with retry
	go conn.ConnectWithRetry(*serverAddr)

	// Wait for connection
	fmt.Printf("Connecting to %s...\n", *serverAddr)
	conn.WaitForConnection()

	// Start UI
	ui.Start()
}

// Initialize logging
func init() {
	// Set up logging to file
	var err error
	logFile, err = os.OpenFile("client.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, common.GetFileMode())
	if err == nil {
		log.SetOutput(logFile)
	} else {
		log.Printf("Failed to open log file: %v", err)
	}
}
