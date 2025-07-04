package main

import (
	"fmt"
	"net"
	"sync"
	"tcp-chat/common"
	"time"
)

// RateLimiter manages rate limiting for the server
type RateLimiter struct {
	// Connection limits
	totalConnections int
	connectionsByIP  map[string]int
	connMutex        sync.RWMutex

	// Message rate limiting
	messageRates map[string]*userRateLimit
	rateMutex    sync.RWMutex

	// Room creation limiting
	roomsPerUser map[string]int
	roomMutex    sync.RWMutex

	// File transfer limiting
	transfersPerUser map[string]int
	transferMutex    sync.RWMutex

	// Cleanup ticker
	cleanupTicker *time.Ticker
}

type userRateLimit struct {
	messages  int
	lastReset time.Time
	mutex     sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		connectionsByIP:  make(map[string]int),
		messageRates:     make(map[string]*userRateLimit),
		roomsPerUser:     make(map[string]int),
		transfersPerUser: make(map[string]int),
		cleanupTicker:    time.NewTicker(1 * time.Minute),
	}

	// Start cleanup routine
	go rl.cleanup()

	return rl
}

// CanConnect checks if a new connection is allowed
func (rl *RateLimiter) CanConnect(addr net.Addr) error {
	rl.connMutex.Lock()
	defer rl.connMutex.Unlock()

	// Check total connections
	if rl.totalConnections >= common.MaxConnections {
		return fmt.Errorf("server has reached maximum connection limit (%d)", common.MaxConnections)
	}

	// Extract IP from address
	ip, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return fmt.Errorf("invalid address format")
	}

	// Check per-IP limit
	if rl.connectionsByIP[ip] >= common.MaxConnectionsPerIP {
		return fmt.Errorf("IP %s has reached maximum connection limit (%d)", ip, common.MaxConnectionsPerIP)
	}

	return nil
}

// AddConnection registers a new connection
func (rl *RateLimiter) AddConnection(addr net.Addr) {
	rl.connMutex.Lock()
	defer rl.connMutex.Unlock()

	rl.totalConnections++

	ip, _, _ := net.SplitHostPort(addr.String())
	rl.connectionsByIP[ip]++
}

// RemoveConnection removes a connection
func (rl *RateLimiter) RemoveConnection(addr net.Addr) {
	rl.connMutex.Lock()
	defer rl.connMutex.Unlock()

	if rl.totalConnections > 0 {
		rl.totalConnections--
	}

	ip, _, _ := net.SplitHostPort(addr.String())
	if count := rl.connectionsByIP[ip]; count > 0 {
		if count == 1 {
			delete(rl.connectionsByIP, ip)
		} else {
			rl.connectionsByIP[ip]--
		}
	}
}

// CanSendMessage checks if a user can send a message
func (rl *RateLimiter) CanSendMessage(nickname string) error {
	rl.rateMutex.Lock()
	userLimit, exists := rl.messageRates[nickname]
	if !exists {
		userLimit = &userRateLimit{
			lastReset: time.Now(),
		}
		rl.messageRates[nickname] = userLimit
	}
	rl.rateMutex.Unlock()

	userLimit.mutex.Lock()
	defer userLimit.mutex.Unlock()

	// Reset counter if a second has passed
	if time.Since(userLimit.lastReset) >= time.Second {
		userLimit.messages = 0
		userLimit.lastReset = time.Now()
	}

	// Check rate limit
	if userLimit.messages >= common.MessagesPerSecond {
		return fmt.Errorf("message rate limit exceeded (%d messages per second)", common.MessagesPerSecond)
	}

	userLimit.messages++
	return nil
}

// CanCreateRoom checks if a user can create a room
func (rl *RateLimiter) CanCreateRoom(nickname string) error {
	rl.roomMutex.Lock()
	defer rl.roomMutex.Unlock()

	if rl.roomsPerUser[nickname] >= common.RoomsPerUser {
		return fmt.Errorf("room creation limit exceeded (%d rooms per user)", common.RoomsPerUser)
	}

	return nil
}

// AddRoom registers a room creation
func (rl *RateLimiter) AddRoom(nickname string) {
	rl.roomMutex.Lock()
	defer rl.roomMutex.Unlock()

	rl.roomsPerUser[nickname]++
}

// RemoveRoom removes a room from user's count
func (rl *RateLimiter) RemoveRoom(nickname string) {
	rl.roomMutex.Lock()
	defer rl.roomMutex.Unlock()

	if count := rl.roomsPerUser[nickname]; count > 0 {
		if count == 1 {
			delete(rl.roomsPerUser, nickname)
		} else {
			rl.roomsPerUser[nickname]--
		}
	}
}

// CanStartFileTransfer checks if a user can start a file transfer
func (rl *RateLimiter) CanStartFileTransfer(nickname string) error {
	rl.transferMutex.Lock()
	defer rl.transferMutex.Unlock()

	if rl.transfersPerUser[nickname] >= common.FileTransfersPerUser {
		return fmt.Errorf("file transfer limit exceeded (%d concurrent transfers per user)", common.FileTransfersPerUser)
	}

	return nil
}

// AddFileTransfer registers a file transfer
func (rl *RateLimiter) AddFileTransfer(nickname string) {
	rl.transferMutex.Lock()
	defer rl.transferMutex.Unlock()

	rl.transfersPerUser[nickname]++
}

// RemoveFileTransfer removes a file transfer
func (rl *RateLimiter) RemoveFileTransfer(nickname string) {
	rl.transferMutex.Lock()
	defer rl.transferMutex.Unlock()

	if count := rl.transfersPerUser[nickname]; count > 0 {
		if count == 1 {
			delete(rl.transfersPerUser, nickname)
		} else {
			rl.transfersPerUser[nickname]--
		}
	}
}

// RemoveUser cleans up all rate limit data for a user
func (rl *RateLimiter) RemoveUser(nickname string) {
	rl.rateMutex.Lock()
	delete(rl.messageRates, nickname)
	rl.rateMutex.Unlock()

	rl.roomMutex.Lock()
	delete(rl.roomsPerUser, nickname)
	rl.roomMutex.Unlock()

	rl.transferMutex.Lock()
	delete(rl.transfersPerUser, nickname)
	rl.transferMutex.Unlock()
}

// cleanup periodically cleans up old rate limit data
func (rl *RateLimiter) cleanup() {
	for range rl.cleanupTicker.C {
		rl.rateMutex.Lock()
		for nick, userLimit := range rl.messageRates {
			userLimit.mutex.Lock()
			if time.Since(userLimit.lastReset) > 5*time.Minute {
				delete(rl.messageRates, nick)
			}
			userLimit.mutex.Unlock()
		}
		rl.rateMutex.Unlock()
	}
}

// Stop stops the rate limiter
func (rl *RateLimiter) Stop() {
	rl.cleanupTicker.Stop()
}
