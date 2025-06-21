package recovery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"gateway/internal/core"
	gwerrors "gateway/pkg/errors"
)

// Mock request implementation
type mockRequest struct {
	method  string
	path    string
	body    io.ReadCloser
	headers map[string][]string
}

func (r *mockRequest) ID() string                   { return "test-request-id" }
func (r *mockRequest) Method() string               { return r.method }
func (r *mockRequest) Path() string                 { return r.path }
func (r *mockRequest) URL() string                  { return "http://example.com" + r.path }
func (r *mockRequest) RemoteAddr() string           { return "127.0.0.1:12345" }
func (r *mockRequest) Headers() map[string][]string { return r.headers }
func (r *mockRequest) Body() io.ReadCloser          { return r.body }
func (r *mockRequest) Context() context.Context     { return context.Background() }

// Mock response implementation
type mockResponse struct {
	statusCode int
	body       io.ReadCloser
	headers    map[string][]string
}

func (r *mockResponse) StatusCode() int              { return r.statusCode }
func (r *mockResponse) Body() io.ReadCloser          { return r.body }
func (r *mockResponse) Headers() map[string][]string { return r.headers }

func TestRecoveryMiddleware_RecoversPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	config := Config{
		StackTrace: false,
	}

	middleware := Middleware(config, logger)

	// Handler that panics
	panicHandler := core.Handler(func(ctx context.Context, req core.Request) (core.Response, error) {
		panic("test panic")
	})

	wrapped := middleware(panicHandler)

	req := &mockRequest{
		method: "GET",
		path:   "/test",
	}

	resp, err := wrapped(context.Background(), req)

	// Should return error instead of panic
	if err == nil {
		t.Error("Expected error from panic recovery")
	}

	// Should be internal error
	var gwErr *gwerrors.Error
	if !errors.As(err, &gwErr) {
		t.Error("Expected gateway error")
	} else if gwErr.Type != gwerrors.ErrorTypeInternal {
		t.Errorf("Expected internal error type, got %v", gwErr.Type)
	}

	// Response should be nil
	if resp != nil {
		t.Error("Expected nil response")
	}
}

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	middleware := Default(logger)

	// Handler that doesn't panic
	normalHandler := core.Handler(func(ctx context.Context, req core.Request) (core.Response, error) {
		return &mockResponse{
			statusCode: 200,
			headers:    make(map[string][]string),
		}, nil
	})

	wrapped := middleware(normalHandler)

	req := &mockRequest{
		method: "GET",
		path:   "/test",
	}

	resp, err := wrapped(context.Background(), req)

	// Should pass through normally
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if resp == nil {
		t.Error("Expected response")
	} else if resp.StatusCode() != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode())
	}
}

func TestRecoveryMiddleware_PanicHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var capturedPanic interface{}
	var capturedStack []byte

	config := Config{
		StackTrace: true,
		PanicHandler: func(ctx context.Context, recovered interface{}, stack []byte) {
			capturedPanic = recovered
			capturedStack = stack
		},
	}

	middleware := Middleware(config, logger)

	// Handler that panics
	panicHandler := core.Handler(func(ctx context.Context, req core.Request) (core.Response, error) {
		panic("custom panic message")
	})

	wrapped := middleware(panicHandler)

	req := &mockRequest{
		method: "POST",
		path:   "/api/test",
	}

	_, err := wrapped(context.Background(), req)

	// Verify panic handler was called
	if capturedPanic == nil {
		t.Error("Panic handler not called")
	}

	if panicMsg, ok := capturedPanic.(string); !ok || panicMsg != "custom panic message" {
		t.Errorf("Expected panic message 'custom panic message', got %v", capturedPanic)
	}

	if len(capturedStack) == 0 {
		t.Error("Expected stack trace")
	}

	// Check error details
	var gwErr *gwerrors.Error
	if errors.As(err, &gwErr) {
		if details, ok := gwErr.Details["panic"].(string); !ok || details != "custom panic message" {
			t.Errorf("Expected panic details in error, got %v", gwErr.Details)
		}
	}
}

func TestRecoveryMiddleware_ErrorPassthrough(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	middleware := Default(logger)

	expectedErr := errors.New("handler error")

	// Handler that returns error
	errorHandler := core.Handler(func(ctx context.Context, req core.Request) (core.Response, error) {
		return nil, expectedErr
	})

	wrapped := middleware(errorHandler)

	req := &mockRequest{
		method: "GET",
		path:   "/test",
	}

	resp, err := wrapped(context.Background(), req)

	// Should pass through error unchanged
	if err != expectedErr {
		t.Errorf("Expected original error, got %v", err)
	}

	if resp != nil {
		t.Error("Expected nil response")
	}
}

func TestRecoveryMiddleware_PanicTypes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name       string
		panicValue interface{}
	}{
		{"string panic", "string value"},
		{"error panic", errors.New("error value")},
		{"int panic", 42},
		{"nil panic", nil},
		{"struct panic", struct{ msg string }{msg: "struct value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := Default(logger)

			panicHandler := core.Handler(func(ctx context.Context, req core.Request) (core.Response, error) {
				panic(tt.panicValue)
			})

			wrapped := middleware(panicHandler)

			req := &mockRequest{
				method: "GET",
				path:   "/test",
			}

			_, err := wrapped(context.Background(), req)

			if err == nil {
				t.Error("Expected error from panic recovery")
			}

			// Verify error contains panic value
			var gwErr *gwerrors.Error
			if errors.As(err, &gwErr) {
				panicDetail := gwErr.Details["panic"].(string)
				expectedDetail := fmt.Sprintf("%v", tt.panicValue)

				// Special case for nil panic - Go runtime converts it to a specific message
				if tt.panicValue == nil && strings.Contains(panicDetail, "nil") {
					// Accept any message that mentions nil (e.g., "panic called with nil argument" or "<nil>")
					return
				}

				if !strings.Contains(panicDetail, expectedDetail) {
					t.Errorf("Expected panic detail to contain %v, got %s", expectedDetail, panicDetail)
				}
			}
		})
	}
}
