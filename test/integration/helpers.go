package integration

import (
	"net"
	"testing"
	"time"
)

// isServerRunning checks if a server is running at the given address
func isServerRunning(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// skipIfServerNotRunning skips the test if the server is not running
func skipIfServerNotRunning(t *testing.T, addr string) {
	if !isServerRunning(addr) {
		t.Skipf("Skipping integration test: server not running at %s", addr)
	}
}