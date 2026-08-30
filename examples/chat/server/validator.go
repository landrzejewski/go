package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"tcp-chat/common"
	"unicode/utf8"
)

var (
	nicknameRegex = regexp.MustCompile(common.NicknamePattern)
	roomNameRegex = regexp.MustCompile(common.RoomNamePattern)
)

// ValidateNickname validates a nickname according to the rules
func ValidateNickname(nickname string) error {
	// utf8.RuneCountInString, a nie len(): len liczy BAJTY, więc limit "20
	// znaków" oznaczałby w praktyce ~6 znaków CJK albo 10 polskich z ogonkami.
	length := utf8.RuneCountInString(nickname)
	if length < common.MinNicknameLength {
		return fmt.Errorf("nickname must be at least %d characters long", common.MinNicknameLength)
	}
	if length > common.MaxNicknameLength {
		return fmt.Errorf("nickname cannot exceed %d characters", common.MaxNicknameLength)
	}
	if !nicknameRegex.MatchString(nickname) {
		return errors.New("nickname can only contain letters, numbers, underscores, and hyphens")
	}
	// "Server" jest nadawcą wszystkich komunikatów systemowych - bez tego
	// zastrzeżenia dowolny użytkownik mógł się pod serwer podszyć.
	if strings.EqualFold(nickname, "Server") {
		return errors.New("nickname 'Server' is reserved")
	}
	return nil
}

// ValidateRoomName validates a room name according to the rules.
// UWAGA: TrimSpace działa tylko na lokalnej kopii - walidujemy tę samą postać,
// którą zapisuje wywołujący (handleRoomMessage też robi TrimSpace).
func ValidateRoomName(roomName string) error {
	roomName = strings.TrimSpace(roomName)

	length := utf8.RuneCountInString(roomName)
	if length < common.MinRoomNameLength {
		return fmt.Errorf("room name must be at least %d characters long", common.MinRoomNameLength)
	}
	if length > common.MaxRoomNameLength {
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
	// MaxMessageSize jest limitem w BAJTACH (chroni bufor odczytu), więc
	// komunikat też mówi o bajtach - inaczej wprowadzałby w błąd przy tekstach
	// spoza ASCII.
	if len(content) > common.MaxMessageSize {
		return fmt.Errorf("message cannot exceed %d bytes", common.MaxMessageSize)
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
