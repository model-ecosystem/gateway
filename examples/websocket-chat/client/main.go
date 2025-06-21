package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

var (
	addr = flag.String("addr", "localhost:8081", "gateway address")
	path = flag.String("path", "/ws/echo", "WebSocket path")
)

func main() {
	flag.Parse()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: *addr, Path: *path}
	fmt.Printf("Connecting to %s...\n", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer conn.Close()

	fmt.Println("Connected! Type messages to send (or 'quit' to exit)")

	done := make(chan struct{})

	// Read messages from server
	go func() {
		defer close(done)
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Println("Read error:", err)
				}
				return
			}

			switch messageType {
			case websocket.TextMessage:
				fmt.Printf("< %s\n", message)
			case websocket.BinaryMessage:
				fmt.Printf("< [Binary: %d bytes]\n", len(message))
			}
		}
	}()

	// Send ping periodically
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Read from stdin and send
	scanner := bufio.NewScanner(os.Stdin)

	for {
		select {
		case <-done:
			return

		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second)); err != nil {
				log.Println("Ping error:", err)
				return
			}

		case <-interrupt:
			fmt.Println("\nInterrupt received, closing connection...")

			// Send close message
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("Close error:", err)
			}

			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return

		default:
			fmt.Print("> ")
			if scanner.Scan() {
				text := scanner.Text()
				if text == "quit" {
					fmt.Println("Closing connection...")
					return
				}

				err := conn.WriteMessage(websocket.TextMessage, []byte(text))
				if err != nil {
					log.Println("Write error:", err)
					return
				}
			}

			if err := scanner.Err(); err != nil {
				log.Println("Scanner error:", err)
				return
			}
		}
	}
}
