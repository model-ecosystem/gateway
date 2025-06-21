package integration

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestSSEBasic tests basic SSE functionality through the gateway
func TestSSEBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	skipIfServerNotRunning(t, "localhost:8081")

	// Gateway SSE URL
	url := "http://localhost:8081/events"

	// Create request with SSE headers
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	// Make request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to connect to gateway: %v", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", contentType)
	}

	// Read events
	scanner := bufio.NewScanner(resp.Body)
	eventCount := 0
	maxEvents := 3

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	eventChan := make(chan string, 10)
	errChan := make(chan error, 1)

	// Read events in goroutine
	go func() {
		var currentEvent strings.Builder

		for scanner.Scan() {
			line := scanner.Text()

			if line == "" && currentEvent.Len() > 0 {
				// End of event
				eventChan <- currentEvent.String()
				currentEvent.Reset()
			} else if line != "" {
				currentEvent.WriteString(line)
				currentEvent.WriteString("\n")
			}
		}

		if err := scanner.Err(); err != nil {
			errChan <- err
		}
	}()

	// Process events
	for eventCount < maxEvents {
		select {
		case event := <-eventChan:
			eventCount++
			t.Logf("Event %d:\n%s", eventCount, event)

			// Verify event structure
			if !strings.Contains(event, "event:") || !strings.Contains(event, "data:") {
				t.Errorf("Invalid event structure: %s", event)
			}

		case err := <-errChan:
			t.Fatalf("Error reading events: %v", err)

		case <-ctx.Done():
			t.Fatalf("Timeout waiting for events (got %d/%d)", eventCount, maxEvents)
		}
	}

	if eventCount < maxEvents {
		t.Errorf("Expected at least %d events, got %d", maxEvents, eventCount)
	}
}

// TestSSEStickySessions tests SSE with sticky sessions
func TestSSEStickySessions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	skipIfServerNotRunning(t, "localhost:8081")

	// Make multiple connections and verify they go to the same backend
	url := "http://localhost:8081/notifications/testuser"

	// First connection
	resp1, err := makeSSERequest(url, "user1")
	if err != nil {
		t.Fatalf("First connection failed: %v", err)
	}
	defer resp1.Body.Close()

	// Read server info from first event
	server1, err := readServerFromSSE(resp1.Body)
	if err != nil {
		t.Fatalf("Failed to read server from first connection: %v", err)
	}

	// Second connection with same session
	resp2, err := makeSSERequest(url, "user1")
	if err != nil {
		t.Fatalf("Second connection failed: %v", err)
	}
	defer resp2.Body.Close()

	server2, err := readServerFromSSE(resp2.Body)
	if err != nil {
		t.Fatalf("Failed to read server from second connection: %v", err)
	}

	// Should route to same backend
	if server1 != server2 {
		t.Errorf("Sticky session failed: first=%s, second=%s", server1, server2)
	}
}

// TestSSEKeepalive tests SSE keepalive functionality
func TestSSEKeepalive(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	skipIfServerNotRunning(t, "localhost:8081")

	url := "http://localhost:8081/events"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{
		Timeout: 0, // No timeout for long-running SSE
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer resp.Body.Close()

	// Read for 35 seconds to ensure we get keepalive
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	scanner := bufio.NewScanner(resp.Body)
	keepaliveFound := false

	done := make(chan bool)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, ":") {
				// This is a comment/keepalive
				keepaliveFound = true
				done <- true
				return
			}
		}
	}()

	select {
	case <-done:
		if !keepaliveFound {
			t.Error("No keepalive comment found")
		}
	case <-ctx.Done():
		t.Error("Timeout waiting for keepalive")
	}
}

// Helper functions

func makeSSERequest(url, userID string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("X-User-ID", userID)
	req.Header.Set("X-Session-Id", userID) // For sticky sessions

	client := &http.Client{Timeout: 0}
	return client.Do(req)
}

func readServerFromSSE(body io.Reader) (string, error) {
	scanner := bufio.NewScanner(body)

	for scanner.Scan() {
		line := scanner.Text()

		// Look for server info in data
		if strings.HasPrefix(line, "data:") && strings.Contains(line, "server") {
			// Extract server address from message
			parts := strings.Split(line, " ")
			for i, part := range parts {
				if part == "server" && i+1 < len(parts) {
					return parts[i+1], nil
				}
			}
		}
	}

	return "", fmt.Errorf("server info not found")
}
