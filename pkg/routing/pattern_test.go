package routing

import "testing"

func TestConvertToServeMuxPattern(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple path",
			input:    "/api/users",
			expected: "/api/users",
		},
		{
			name:     "path with parameter",
			input:    "/api/users/:id",
			expected: "/api/users/{id}",
		},
		{
			name:     "path with multiple parameters",
			input:    "/api/users/:userId/posts/:postId",
			expected: "/api/users/{userId}/posts/{postId}",
		},
		{
			name:     "path with wildcard /*",
			input:    "/api/users/*",
			expected: "/api/users/{path...}",
		},
		{
			name:     "path with wildcard *",
			input:    "/api/users*",
			expected: "/api/users{path...}",
		},
		{
			name:     "complex path with params and wildcard",
			input:    "/api/users/:id/files/*",
			expected: "/api/users/{id}/files/{path...}",
		},
		{
			name:     "root wildcard",
			input:    "/*",
			expected: "/{path...}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToServeMuxPattern(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertToServeMuxPattern(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
