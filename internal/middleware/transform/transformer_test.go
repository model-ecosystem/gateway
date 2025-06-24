package transform

import (
	"encoding/json"
	"testing"
)

func TestJSONTransformer(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		operations []Operation
		expected   string
	}{
		{
			name: "add field",
			input: `{"name":"John","age":30}`,
			operations: []Operation{
				{Type: "add", Path: "email", Value: "john@example.com"},
			},
			expected: `{"age":30,"email":"john@example.com","name":"John"}`,
		},
		{
			name: "remove field",
			input: `{"name":"John","age":30,"sensitive":"data"}`,
			operations: []Operation{
				{Type: "remove", Path: "sensitive"},
			},
			expected: `{"age":30,"name":"John"}`,
		},
		{
			name: "rename field",
			input: `{"firstName":"John","age":30}`,
			operations: []Operation{
				{Type: "rename", From: "firstName", To: "name"},
			},
			expected: `{"age":30,"name":"John"}`,
		},
		{
			name: "modify field",
			input: `{"name":"John","age":30}`,
			operations: []Operation{
				{Type: "modify", Path: "age", Value: 31},
			},
			expected: `{"age":31,"name":"John"}`,
		},
		{
			name: "nested field operations",
			input: `{"user":{"name":"John","age":30}}`,
			operations: []Operation{
				{Type: "add", Path: "user.email", Value: "john@example.com"},
				{Type: "modify", Path: "user.age", Value: 31},
			},
			expected: `{"user":{"age":31,"email":"john@example.com","name":"John"}}`,
		},
		{
			name: "multiple operations",
			input: `{"firstName":"John","lastName":"Doe","age":30,"private":"info"}`,
			operations: []Operation{
				{Type: "rename", From: "firstName", To: "first_name"},
				{Type: "rename", From: "lastName", To: "last_name"},
				{Type: "remove", Path: "private"},
				{Type: "add", Path: "full_name", Value: "John Doe"},
			},
			expected: `{"age":30,"first_name":"John","full_name":"John Doe","last_name":"Doe"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := NewJSONTransformer(tt.operations, nil)
			result, err := transformer.Transform([]byte(tt.input), "application/json")
			if err != nil {
				t.Fatalf("Transform failed: %v", err)
			}

			// Compare as JSON objects to ignore formatting differences
			var resultObj, expectedObj interface{}
			if err := json.Unmarshal(result, &resultObj); err != nil {
				t.Fatalf("Failed to parse result: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expectedObj); err != nil {
				t.Fatalf("Failed to parse expected: %v", err)
			}

			resultJSON, _ := json.Marshal(resultObj)
			expectedJSON, _ := json.Marshal(expectedObj)

			if string(resultJSON) != string(expectedJSON) {
				t.Errorf("Transform result mismatch\nGot:      %s\nExpected: %s", resultJSON, expectedJSON)
			}
		})
	}
}

func TestHeaderTransformer(t *testing.T) {
	config := HeaderConfig{
		Add: map[string]string{
			"X-Custom-Header": "custom-value",
			"X-Request-ID":    "123456",
		},
		Remove: []string{"X-Internal-Secret"},
		Rename: map[string]string{
			"X-Old-Header": "X-New-Header",
		},
		Modify: map[string]string{
			"Authorization": "Bearer ",
		},
	}

	headers := map[string][]string{
		"Content-Type":       {"application/json"},
		"X-Internal-Secret":  {"secret-value"},
		"X-Old-Header":       {"old-value"},
		"Authorization":      {"Bearer token123"},
	}

	transformer := NewHeaderTransformer(config, nil)
	result := transformer.TransformHeaders(headers)

	// Check additions
	if v, ok := result["X-Custom-Header"]; !ok || len(v) != 1 || v[0] != "custom-value" {
		t.Errorf("Expected X-Custom-Header to be added with value 'custom-value', got %v", v)
	}

	// Check removals
	if _, ok := result["X-Internal-Secret"]; ok {
		t.Error("Expected X-Internal-Secret to be removed")
	}

	// Check renames
	if _, ok := result["X-Old-Header"]; ok {
		t.Error("Expected X-Old-Header to be renamed")
	}
	if v, ok := result["X-New-Header"]; !ok || len(v) != 1 || v[0] != "old-value" {
		t.Errorf("Expected X-New-Header with value 'old-value', got %v", v)
	}

	// Check modifications
	if v, ok := result["Authorization"]; !ok || len(v) != 1 || v[0] != "token123" {
		t.Errorf("Expected Authorization to be modified to 'token123', got %v", v)
	}

	// Check untouched headers
	if v, ok := result["Content-Type"]; !ok || len(v) != 1 || v[0] != "application/json" {
		t.Errorf("Expected Content-Type to remain unchanged, got %v", v)
	}
}

func TestPathOperations(t *testing.T) {
	transformer := &JSONTransformer{logger: nil}

	// Test getValueAtPath
	data := map[string]interface{}{
		"user": map[string]interface{}{
			"name": "John",
			"details": map[string]interface{}{
				"age":  30,
				"city": "NYC",
			},
		},
	}

	val, exists := transformer.getValueAtPath(data, []string{"user", "name"})
	if !exists || val != "John" {
		t.Errorf("Expected to get 'John', got %v (exists: %v)", val, exists)
	}

	val, exists = transformer.getValueAtPath(data, []string{"user", "details", "age"})
	if !exists || val != 30 {
		t.Errorf("Expected to get 30, got %v (exists: %v)", val, exists)
	}

	_, exists = transformer.getValueAtPath(data, []string{"user", "nonexistent"})
	if exists {
		t.Error("Expected nonexistent path to return false")
	}
}
