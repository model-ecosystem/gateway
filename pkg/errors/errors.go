package errors

import (
	"fmt"
)

// ErrorType represents the type of error
type ErrorType string

const (
	ErrorTypeInternal    ErrorType = "internal"
	ErrorTypeNotFound    ErrorType = "not_found"
	ErrorTypeUnavailable ErrorType = "unavailable"
	ErrorTypeBadRequest  ErrorType = "bad_request"
	ErrorTypeTimeout     ErrorType = "timeout"
)

// Error represents a structured error with additional context
type Error struct {
	Type    ErrorType
	Message string
	Cause   error
	Details map[string]any
}

// NewError creates a new structured error
func NewError(errType ErrorType, message string) *Error {
	return &Error{
		Type:    errType,
		Message: message,
		Details: make(map[string]any),
	}
}

// WithCause adds the underlying cause to the error
func (e *Error) WithCause(cause error) *Error {
	e.Cause = cause
	return e
}

// WithDetail adds a detail to the error
func (e *Error) WithDetail(key string, value any) *Error {
	e.Details[key] = value
	return e
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying cause
func (e *Error) Unwrap() error {
	return e.Cause
}

// Is implements errors.Is
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Type == t.Type
}

// HTTPStatusCode returns the appropriate HTTP status code for the error type
func (e *Error) HTTPStatusCode() int {
	switch e.Type {
	case ErrorTypeNotFound:
		return 404
	case ErrorTypeBadRequest:
		return 400
	case ErrorTypeTimeout:
		return 408
	case ErrorTypeUnavailable:
		return 503
	default:
		return 500
	}
}

// Wrap wraps an error with additional context
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}