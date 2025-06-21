package errors

import (
	"fmt"
	"strings"
	"testing"
)

func TestNewError(t *testing.T) {
	tests := []struct {
		name        string
		errorType   ErrorType
		message     string
		wantMessage string
	}{
		{
			name:        "not found error",
			errorType:   ErrorTypeNotFound,
			message:     "resource not found",
			wantMessage: "resource not found",
		},
		{
			name:        "unavailable error",
			errorType:   ErrorTypeUnavailable,
			message:     "service unavailable",
			wantMessage: "service unavailable",
		},
		{
			name:        "timeout error",
			errorType:   ErrorTypeTimeout,
			message:     "request timeout",
			wantMessage: "request timeout",
		},
		{
			name:        "bad request error",
			errorType:   ErrorTypeBadRequest,
			message:     "invalid input",
			wantMessage: "invalid input",
		},
		{
			name:        "internal error",
			errorType:   ErrorTypeInternal,
			message:     "internal server error",
			wantMessage: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewError(tt.errorType, tt.message)
			
			if err.Type != tt.errorType {
				t.Errorf("NewError() type = %v, want %v", err.Type, tt.errorType)
			}
			
			if err.Message != tt.wantMessage {
				t.Errorf("NewError() message = %v, want %v", err.Message, tt.wantMessage)
			}
			
			if err.Details == nil {
				t.Error("NewError() details should be initialized")
			}
		})
	}
}

func TestErrorWithDetails(t *testing.T) {
	err := NewError(ErrorTypeNotFound, "service not found").
		WithDetail("service", "test-service").
		WithDetail("namespace", "default")

	if err.Details["service"] != "test-service" {
		t.Errorf("WithDetail() service = %v, want test-service", err.Details["service"])
	}

	if err.Details["namespace"] != "default" {
		t.Errorf("WithDetail() namespace = %v, want default", err.Details["namespace"])
	}

	// Test chaining
	err.WithDetail("version", "v1").WithDetail("region", "us-east-1")
	
	if len(err.Details) != 4 {
		t.Errorf("Expected 4 details, got %d", len(err.Details))
	}
}

func TestErrorWithCause(t *testing.T) {
	cause := fmt.Errorf("connection refused")
	err := NewError(ErrorTypeUnavailable, "backend unavailable").
		WithCause(cause)

	if err.Cause != cause {
		t.Errorf("WithCause() cause = %v, want %v", err.Cause, cause)
	}

	// Test Error() includes cause
	errorStr := err.Error()
	if !strings.Contains(errorStr, "connection refused") {
		t.Errorf("Error() should include cause, got: %v", errorStr)
	}
}


func TestErrorString(t *testing.T) {
	// Simple error without details or cause
	err1 := NewError(ErrorTypeNotFound, "user not found")
	str1 := err1.Error()
	
	expected := "not_found: user not found"
	if str1 != expected {
		t.Errorf("Error() = %v, want '%s'", str1, expected)
	}

	// Error with details (details are not included in Error() string)
	err2 := NewError(ErrorTypeNotFound, "user not found").
		WithDetail("id", "123").
		WithDetail("method", "GET")
	str2 := err2.Error()
	
	expected2 := "not_found: user not found"
	if str2 != expected2 {
		t.Errorf("Error() = %v, want '%s'", str2, expected2)
	}
	
	// Verify details are stored even if not in error string
	if err2.Details["id"] != "123" {
		t.Error("Detail 'id' not stored correctly")
	}
	if err2.Details["method"] != "GET" {
		t.Error("Detail 'method' not stored correctly")
	}

	// Error with cause
	cause := fmt.Errorf("database connection failed")
	err3 := NewError(ErrorTypeInternal, "failed to fetch user").
		WithCause(cause)
	str3 := err3.Error()
	
	expected3 := "internal: failed to fetch user: database connection failed"
	if str3 != expected3 {
		t.Errorf("Error() = %v, want '%s'", str3, expected3)
	}

	// Error with both details and cause
	err4 := NewError(ErrorTypeInternal, "operation failed").
		WithDetail("operation", "update").
		WithDetail("table", "users").
		WithCause(fmt.Errorf("timeout"))
	str4 := err4.Error()
	
	expected4 := "internal: operation failed: timeout"
	if str4 != expected4 {
		t.Errorf("Error() = %v, want '%s'", str4, expected4)
	}
	
	// Verify details are still stored
	if err4.Details["operation"] != "update" {
		t.Error("Detail 'operation' not stored correctly")
	}
	if err4.Details["table"] != "users" {
		t.Error("Detail 'table' not stored correctly")
	}
}

func TestErrorNilHandling(t *testing.T) {
	// Test WithDetail with nil value
	err := NewError(ErrorTypeNotFound, "test").
		WithDetail("key", nil)
	
	if err.Details["key"] != nil {
		t.Errorf("WithDetail() should accept nil value")
	}

	// Test WithCause with nil
	err2 := NewError(ErrorTypeNotFound, "test").
		WithCause(nil)
	
	if err2.Cause != nil {
		t.Errorf("WithCause() should accept nil")
	}
	
	// Error string should not include nil cause
	if strings.Contains(err2.Error(), "caused by") {
		t.Errorf("Error() should not include nil cause")
	}
}

func TestErrorTypeValidation(t *testing.T) {
	// Test with empty error type
	err := NewError("", "test message")
	if err.Type != "" {
		t.Errorf("Empty error type should be preserved")
	}
}