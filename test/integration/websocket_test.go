package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestWebSocketEcho tests WebSocket echo through the gateway
func TestWebSocketEcho(t *testing.T) {
	// Skip if not integration test
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	skipIfServerNotRunning(t, "localhost:8081")

	// Gateway WebSocket URL
	u := url.URL{Scheme: "ws", Host: "localhost:8081", Path: "/ws/echo"}

	// Connect to gateway
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	conn, resp, err := dialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to gateway: %v", err)
	}
	defer conn.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("Expected status %d, got %d", http.StatusSwitchingProtocols, resp.StatusCode)
	}

	// Test messages
	messages := []string{
		"Hello, WebSocket!",
		"Testing gateway proxy",
		"Unicode test: Hello World 🚀",
		"Large message: " + generateLargeMessage(1000),
	}

	for _, msg := range messages {
		// Send message
		err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
		if err != nil {
			t.Fatalf("Failed to write message: %v", err)
		}

		// Read echo
		messageType, reply, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		if messageType != websocket.TextMessage {
			t.Errorf("Expected text message, got type %d", messageType)
		}

		if string(reply) != msg {
			t.Errorf("Expected echo %q, got %q", msg, string(reply))
		}
	}

	// Test binary message
	binaryData := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	err = conn.WriteMessage(websocket.BinaryMessage, binaryData)
	if err != nil {
		t.Fatalf("Failed to write binary message: %v", err)
	}

	messageType, reply, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read binary message: %v", err)
	}

	if messageType != websocket.BinaryMessage {
		t.Errorf("Expected binary message, got type %d", messageType)
	}

	if string(reply) != string(binaryData) {
		t.Errorf("Binary data mismatch")
	}

	// Close connection
	err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		t.Errorf("Failed to send close message: %v", err)
	}
}

// TestWebSocketPingPong tests ping/pong through the gateway
func TestWebSocketPingPong(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	skipIfServerNotRunning(t, "localhost:8081")

	u := url.URL{Scheme: "ws", Host: "localhost:8081", Path: "/ws/echo"}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Set pong handler
	pongReceived := make(chan string, 1)
	conn.SetPongHandler(func(data string) error {
		pongReceived <- data
		return nil
	})

	// Send ping
	pingData := "ping-test"
	err = conn.WriteControl(websocket.PingMessage, []byte(pingData), time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("Failed to send ping: %v", err)
	}

	// Start read loop in goroutine
	go func() {
		for {
			if _, _, err := conn.NextReader(); err != nil {
				return
			}
		}
	}()

	// Wait for pong
	select {
	case data := <-pongReceived:
		if data != pingData {
			t.Errorf("Expected pong data %q, got %q", pingData, data)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for pong")
	}
}

// TestWebSocketConcurrent tests concurrent WebSocket connections
func TestWebSocketConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	skipIfServerNotRunning(t, "localhost:8081")

	numClients := 10
	numMessages := 5

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	errChan := make(chan error, numClients)

	for i := 0; i < numClients; i++ {
		clientID := i
		go func() {
			u := url.URL{Scheme: "ws", Host: "localhost:8081", Path: "/ws/echo"}

			conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
			if err != nil {
				errChan <- fmt.Errorf("client %d: failed to connect: %w", clientID, err)
				return
			}
			defer conn.Close()

			for j := 0; j < numMessages; j++ {
				msg := fmt.Sprintf("Client %d - Message %d", clientID, j)

				err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
				if err != nil {
					errChan <- fmt.Errorf("client %d: write error: %w", clientID, err)
					return
				}

				_, reply, err := conn.ReadMessage()
				if err != nil {
					errChan <- fmt.Errorf("client %d: read error: %w", clientID, err)
					return
				}

				if string(reply) != msg {
					errChan <- fmt.Errorf("client %d: expected %q, got %q", clientID, msg, string(reply))
					return
				}
			}

			errChan <- nil
		}()
	}

	// Wait for all clients
	for i := 0; i < numClients; i++ {
		select {
		case err := <-errChan:
			if err != nil {
				t.Error(err)
			}
		case <-ctx.Done():
			t.Fatal("Test timeout")
		}
	}
}

// generateLargeMessage generates a message of specified size
func generateLargeMessage(size int) string {
	msg := make([]byte, size)
	for i := range msg {
		msg[i] = byte('A' + (i % 26))
	}
	return string(msg)
}
