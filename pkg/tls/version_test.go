package tls

import (
	"crypto/tls"
	"testing"
)

func TestParseTLSVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected uint16
	}{
		{
			name:     "TLS 1.0",
			input:    "1.0",
			expected: tls.VersionTLS10,
		},
		{
			name:     "TLS 1.1",
			input:    "1.1",
			expected: tls.VersionTLS11,
		},
		{
			name:     "TLS 1.2",
			input:    "1.2",
			expected: tls.VersionTLS12,
		},
		{
			name:     "TLS 1.3",
			input:    "1.3",
			expected: tls.VersionTLS13,
		},
		{
			name:     "invalid version defaults to 1.2",
			input:    "invalid",
			expected: tls.VersionTLS12,
		},
		{
			name:     "empty string defaults to 1.2",
			input:    "",
			expected: tls.VersionTLS12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTLSVersion(tt.input)
			if result != tt.expected {
				t.Errorf("ParseTLSVersion(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}