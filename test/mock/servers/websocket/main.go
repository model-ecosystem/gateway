package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var (
	addr = flag.String("addr", ":3001", "server address")
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for testing
	},
}

// echoHandler handles WebSocket connections and echoes messages back
func echoHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("New connection from %s", r.RemoteAddr)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Set read deadline
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	// Handle ping/pong
	conn.SetPingHandler(func(data string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
	})

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Read error: %v", err)
			}
			break
		}

		log.Printf("Received: %s", message)

		// Echo the message back
		if err := conn.WriteMessage(messageType, message); err != nil {
			log.Printf("Write error: %v", err)
			break
		}

		// Reset read deadline
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	}

	log.Printf("Connection closed from %s", r.RemoteAddr)
}

// healthHandler returns health status
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func main() {
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/echo", echoHandler)
	mux.HandleFunc("/health", healthHandler)

	server := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	fmt.Printf("WebSocket Echo Server listening on %s\n", *addr)
	fmt.Println("Endpoints:")
	fmt.Printf("  - ws://localhost%s/ws/echo (WebSocket echo)\n", *addr)
	fmt.Printf("  - http://localhost%s/health (Health check)\n", *addr)

	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Server error:", err)
	}
}
