package chat

import (
	"log"
	"net"
	"sync"
)

var messages = make(chan *message, 1000)
var connections = make([]net.Conn, 0)
var mutex = &sync.RWMutex{}

func Server(address string) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		panic(err)
	}
	defer func() {
		err := listener.Close()
		if err != nil {
			panic(err)
		}
	}()

	go messageHandler()

	log.Println("Listening on: " + address)

	for {
		connection, err := listener.Accept()
		if err != nil {
			log.Println("Connection accept error: " + err.Error())
			continue
		}
		log.Println("Client connected: ", connection.LocalAddr())
		mutex.Lock()
		connections = append(connections, connection)
		go connectionHandler(connection, messages)
		mutex.Unlock()
	}

	// close(messages)
}

func connectionHandler(connection net.Conn, messages chan<- *message) {
	defer func() {
		err := connection.Close()
		if err != nil {
			log.Println("Error closing connection: " + err.Error())
		}
	}()
	messageBytes := make([]byte, bufferSize)
	for {
		if _, err := connection.Read(messageBytes); err != nil {
			log.Println("Error reading message")
			break
		}
		messages <- &message{connection, messageBytes}
	}

}

func messageHandler() {
	for message := range messages {
		mutex.RLock()
		for _, connection := range connections {
			if connection == message.sender {
				continue
			}
			if _, err := connection.Write(message.bytes); err != nil {
				log.Println("Error sending message")
				continue
			}
		}
		mutex.RUnlock()
	}
}
