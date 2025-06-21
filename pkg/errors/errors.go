package errors

import (
	"errors"
	"fmt"
)

// ErrorType represents the type of error
type ErrorType string

const (
	// ErrorTypeInternal represents internal server errors (HTTP 500)
	ErrorTypeInternal ErrorType = "internal"
	// ErrorTypeNotFound represents resource not found errors (HTTP 404)
	ErrorTypeNotFound ErrorType = "not_found"
	// ErrorTypeUnavailable represents service unavailable errors (HTTP 503)
	ErrorTypeUnavailable ErrorType = "unavailable"
	// ErrorTypeBadRequest represents bad request errors (HTTP 400)
	ErrorTypeBadRequest ErrorType = "bad_request"
	// ErrorTypeTimeout represents request timeout errors (HTTP 408)
	ErrorTypeTimeout ErrorType = "timeout"
	// ErrorTypeRateLimit represents rate limit exceeded errors (HTTP 429)
	ErrorTypeRateLimit ErrorType = "rate_limit"
	// ErrorTypeUnauthorized represents unauthorized errors (HTTP 401)
	ErrorTypeUnauthorized ErrorType = "unauthorized"
	// ErrorTypeForbidden represents forbidden errors (HTTP 403)
	ErrorTypeForbidden ErrorType = "forbidden"
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

// Wrap wraps an error with additional context, preserving structured error types
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}

	// If it's already a structured error, preserve its type
	var structuredErr *Error
	if As(err, &structuredErr) {
		// Create a new error with the same type but updated message
		return NewError(structuredErr.Type, message).WithCause(err)
	}

	// For non-structured errors, use simple wrapping
	return fmt.Errorf("%s: %w", message, err)
}

// WrapWithType wraps an error with a specific error type
func WrapWithType(err error, errType ErrorType, message string) error {
	if err == nil {
		return nil
	}
	return NewError(errType, message).WithCause(err)
}

// As is a convenience wrapper around errors.As
func As(err error, target any) bool {
	return errors.As(err, target)
}
