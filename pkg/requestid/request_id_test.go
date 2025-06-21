package requestid

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateRequestID(t *testing.T) {
	// Generate multiple request IDs
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateRequestID()

		// Check format: timestamp-randomhex
		parts := strings.Split(id, "-")
		if len(parts) != 2 {
			t.Errorf("Invalid request ID format: %s", id)
		}

		// Check if ID is unique
		if ids[id] {
			t.Errorf("Duplicate request ID generated: %s", id)
		}
		ids[id] = true

		// Check timestamp part is reasonable
		timestamp := parts[0]
		if len(timestamp) < 13 { // Unix millisecond timestamp should be at least 13 digits
			t.Errorf("Timestamp part too short: %s", timestamp)
		}

		// Check random part is 8 hex characters
		randomPart := parts[1]
		if len(randomPart) != 8 {
			t.Errorf("Random part should be 8 characters, got %d: %s", len(randomPart), randomPart)
		}

		// Verify it's hex
		for _, char := range randomPart {
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
				t.Errorf("Random part contains non-hex character: %c in %s", char, randomPart)
			}
		}
	}
}

func TestGenerateRequestIDTiming(t *testing.T) {
	// Generate IDs quickly and ensure timestamps make sense
	id1 := GenerateRequestID()
	time.Sleep(1 * time.Millisecond)
	id2 := GenerateRequestID()

	parts1 := strings.Split(id1, "-")
	parts2 := strings.Split(id2, "-")

	// The second ID should have a timestamp >= the first
	// (allowing for same millisecond due to system precision)
	if parts2[0] < parts1[0] {
		t.Errorf("Second ID has earlier timestamp: %s vs %s", parts2[0], parts1[0])
	}
}
