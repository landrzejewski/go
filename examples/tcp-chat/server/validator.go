package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"tcp-chat/common"
)

var (
	nicknameRegex = regexp.MustCompile(common.NicknamePattern)
	roomNameRegex = regexp.MustCompile(common.RoomNamePattern)
)

// ValidateNickname validates a nickname according to the rules
func ValidateNickname(nickname string) error {
	if len(nickname) < common.MinNicknameLength {
		return fmt.Errorf("nickname must be at least %d characters long", common.MinNicknameLength)
	}
	if len(nickname) > common.MaxNicknameLength {
		return fmt.Errorf("nickname cannot exceed %d characters", common.MaxNicknameLength)
	}
	if !nicknameRegex.MatchString(nickname) {
		return errors.New("nickname can only contain letters, numbers, underscores, and hyphens")
	}
	return nil
}

// ValidateRoomName validates a room name according to the rules
func ValidateRoomName(roomName string) error {
	// Trim leading and trailing spaces
	roomName = strings.TrimSpace(roomName)

	if len(roomName) < common.MinRoomNameLength {
		return fmt.Errorf("room name must be at least %d characters long", common.MinRoomNameLength)
	}
	if len(roomName) > common.MaxRoomNameLength {
		return fmt.Errorf("room name cannot exceed %d characters", common.MaxRoomNameLength)
	}
	if !roomNameRegex.MatchString(roomName) {
		return errors.New("room name can only contain letters, numbers, underscores, hyphens, and spaces")
	}
	return nil
}

// ValidateMessage validates a message content
func ValidateMessage(content string) error {
	if len(content) == 0 {
		return errors.New("message cannot be empty")
	}
	if len(content) > common.MaxMessageSize {
		return fmt.Errorf("message cannot exceed %d characters", common.MaxMessageSize)
	}
	return nil
}

// ValidateFileName validates a file name for security
func ValidateFileName(filename string) error {
	if len(filename) == 0 {
		return errors.New("filename cannot be empty")
	}
	if len(filename) > common.MaxFileNameLength {
		return fmt.Errorf("filename cannot exceed %d characters", common.MaxFileNameLength)
	}

	// Check for path traversal attempts
	cleanPath := filepath.Clean(filename)
	if strings.Contains(cleanPath, "..") || strings.ContainsAny(cleanPath, `/\`) {
		return errors.New("filename cannot contain path separators or parent directory references")
	}

	// Check for hidden files
	if strings.HasPrefix(filename, ".") {
		return errors.New("hidden files are not allowed")
	}

	return nil
}

// ValidateFileSize validates file size is within limits
func ValidateFileSize(size int64) error {
	if size <= 0 {
		return errors.New("file size must be positive")
	}
	if size > common.MaxFileSize {
		return fmt.Errorf("file size cannot exceed %d bytes", common.MaxFileSize)
	}
	return nil
}
