package common

import "fmt"

// ErrorType represents the type of error
type ErrorType string

const (
	ErrValidation   ErrorType = "VALIDATION"
	ErrRateLimit    ErrorType = "RATE_LIMIT"
	ErrNotFound     ErrorType = "NOT_FOUND"
	ErrUnauthorized ErrorType = "UNAUTHORIZED"
	ErrInternal     ErrorType = "INTERNAL"
	ErrTimeout      ErrorType = "TIMEOUT"
	ErrDuplicate    ErrorType = "DUPLICATE"
)

// ChatError represents a custom error with context
type ChatError struct {
	Type    ErrorType
	Message string
	Details map[string]interface{}
}

// Error implements the error interface
func (e *ChatError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// NewChatError creates a new chat error
func NewChatError(errType ErrorType, message string) *ChatError {
	return &ChatError{
		Type:    errType,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// WithDetail adds a detail to the error
func (e *ChatError) WithDetail(key string, value interface{}) *ChatError {
	e.Details[key] = value
	return e
}

// IsType checks if error is of specific type
func IsType(err error, errType ErrorType) bool {
	if chatErr, ok := err.(*ChatError); ok {
		return chatErr.Type == errType
	}
	return false
}
