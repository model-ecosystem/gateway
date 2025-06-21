package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	httpAdapter "gateway/internal/adapter/http"
	"gateway/internal/core"
)

// Simple handler that echoes the request ID
func echoHandler(ctx context.Context, req core.Request) (core.Response, error) {
	fmt.Printf("Received request with ID: %s\n", req.ID())
	fmt.Printf("  Path: %s\n", req.Path())
	fmt.Printf("  Method: %s\n", req.Method())
	fmt.Printf("  Timestamp: %s\n", time.Now().Format(time.RFC3339))
	fmt.Println()
	
	responseBody := fmt.Sprintf("Response with request ID: %s", req.ID())
	
	return &mockResponse{
		statusCode: 200,
		headers: map[string][]string{
			"X-Request-ID": {req.ID()},
			"Content-Type": {"text/plain"},
		},
		body: io.NopCloser(strings.NewReader(responseBody)),
	}, nil
}

type mockResponse struct {
	statusCode int
	headers    map[string][]string
	body       io.ReadCloser
}

func (m *mockResponse) StatusCode() int               { return m.statusCode }
func (m *mockResponse) Headers() map[string][]string { return m.headers }
func (m *mockResponse) Body() io.ReadCloser          { return m.body }


func main() {
	// Create HTTP adapter with echo handler
	cfg := httpAdapter.Config{
		Host:         "127.0.0.1",
		Port:         8080,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	
	adapter := httpAdapter.New(cfg, echoHandler)
	
	// Start the server
	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		log.Fatal(err)
	}
	
	fmt.Println("Server started on http://127.0.0.1:8080")
	fmt.Println("Request ID format: timestamp-randomhex (e.g., 1737039600123-a2b3c4d5)")
	fmt.Println()
	fmt.Println("Try making requests:")
	fmt.Println("  curl -i http://127.0.0.1:8080/test")
	fmt.Println()
	fmt.Println("Each request will have a unique ID with:")
	fmt.Println("  - Timestamp component for chronological ordering")
	fmt.Println("  - Random component for uniqueness")
	fmt.Println()
	
	// Keep running
	select {}
}