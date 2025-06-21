package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

var (
	addr = flag.String("addr", ":3010", "server address")
)

// sseHandler handles SSE connections and sends periodic events
func sseHandler(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	log.Printf("SSE connection from %s", r.RemoteAddr)

	// Send initial event
	fmt.Fprintf(w, "event: connected\n")
	fmt.Fprintf(w, "data: Connected to SSE server at %s\n\n", time.Now().Format(time.RFC3339))
	flusher.Flush()

	// Create ticker for periodic events
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Create done channel
	done := r.Context().Done()

	eventCount := 0
	for {
		select {
		case <-done:
			log.Printf("SSE connection closed from %s", r.RemoteAddr)
			return

		case t := <-ticker.C:
			eventCount++

			// Send different types of events
			switch eventCount % 3 {
			case 0:
				// Regular event with ID
				fmt.Fprintf(w, "id: %d\n", eventCount)
				fmt.Fprintf(w, "event: tick\n")
				fmt.Fprintf(w, "data: Event %d at %s\n\n", eventCount, t.Format(time.RFC3339))

			case 1:
				// Multi-line data event
				fmt.Fprintf(w, "id: %d\n", eventCount)
				fmt.Fprintf(w, "event: status\n")
				fmt.Fprintf(w, "data: {\n")
				fmt.Fprintf(w, "data:   \"count\": %d,\n", eventCount)
				fmt.Fprintf(w, "data:   \"time\": \"%s\",\n", t.Format(time.RFC3339))
				fmt.Fprintf(w, "data:   \"server\": \"%s\"\n", *addr)
				fmt.Fprintf(w, "data: }\n\n")

			case 2:
				// Simple data-only event
				fmt.Fprintf(w, "data: Heartbeat %d\n\n", eventCount)
			}

			flusher.Flush()

			// Send comment as keepalive every 10 events
			if eventCount%10 == 0 {
				fmt.Fprintf(w, ": keepalive\n")
				flusher.Flush()
			}
		}
	}
}

// notificationHandler sends user-specific notifications
func notificationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Extract user from path or header
	user := r.Header.Get("X-User-ID")
	if user == "" {
		user = "anonymous"
	}

	log.Printf("Notification stream for user %s from %s", user, r.RemoteAddr)

	// Send personalized welcome
	fmt.Fprintf(w, "event: welcome\n")
	fmt.Fprintf(w, "data: Welcome %s! You are connected to server %s\n\n", user, *addr)
	flusher.Flush()

	// Send periodic notifications
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	done := r.Context().Done()
	notificationCount := 0

	for {
		select {
		case <-done:
			return

		case <-ticker.C:
			notificationCount++
			fmt.Fprintf(w, "id: notif-%d\n", notificationCount)
			fmt.Fprintf(w, "event: notification\n")
			fmt.Fprintf(w, "data: Notification %d for %s from server %s\n\n",
				notificationCount, user, *addr)
			flusher.Flush()
		}
	}
}

// healthHandler returns health status
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/events", sseHandler)
	mux.HandleFunc("/notifications/", notificationHandler)
	mux.HandleFunc("/health", healthHandler)

	server := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // No write timeout for SSE
	}

	fmt.Printf("SSE Server listening on %s\n", *addr)
	fmt.Println("Endpoints:")
	fmt.Printf("  - http://localhost%s/events (SSE event stream)\n", *addr)
	fmt.Printf("  - http://localhost%s/notifications/[user] (User notifications)\n", *addr)
	fmt.Printf("  - http://localhost%s/health (Health check)\n", *addr)

	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Server error:", err)
	}
}
