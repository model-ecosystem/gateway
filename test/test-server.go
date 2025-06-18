package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

var (
	port = flag.Int("port", 3000, "Server port")
	name = flag.String("name", "test-server", "Server name")
)

type Response struct {
	Server    string    `json:"server"`
	Timestamp time.Time `json:"timestamp"`
	Path      string    `json:"path"`
	Method    string    `json:"method"`
	Headers   map[string][]string `json:"headers"`
}

func main() {
	flag.Parse()

	mux := http.NewServeMux()
	
	// 健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	// 通用处理器
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		resp := Response{
			Server:    fmt.Sprintf("%s:%d", *name, *port),
			Timestamp: time.Now(),
			Path:      r.URL.Path,
			Method:    r.Method,
			Headers:   r.Header,
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Server-Name", *name)
		w.Header().Set("X-Server-Port", fmt.Sprintf("%d", *port))

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting %s on %s", *name, addr)
	
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

// 使用方法：
// go run test/test-server.go -port 3000 -name example-1
// go run test/test-server.go -port 3001 -name example-2