// Package requestid provides request ID generation utilities.
// It generates unique request IDs with timestamp and random components.
package requestid

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

// counter is used as fallback when random generation fails
var counter atomic.Uint64

// GenerateRequestID generates a unique request ID with format: timestamp-randomhex
// Example: 1737039600123-a2b3c4d5
func GenerateRequestID() string {
	// Get current timestamp in milliseconds
	timestamp := time.Now().UnixMilli()
	
	// Generate 4 bytes of random data
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		// Fallback to counter if random generation fails
		return fmt.Sprintf("%d-%d", timestamp, counter.Add(1))
	}
	
	// Format: timestamp-randomhex
	return fmt.Sprintf("%d-%s", timestamp, hex.EncodeToString(randomBytes))
}