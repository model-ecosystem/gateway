package transcoding

import (
	"encoding/base64"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestNewProtoRegistry(t *testing.T) {
	registry := NewProtoRegistry()
	
	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}
	if registry.files == nil {
		t.Error("Expected initialized files registry")
	}
	if registry.types == nil {
		t.Error("Expected initialized types registry")
	}
	if registry.services == nil {
		t.Error("Expected initialized services map")
	}
	if registry.methods == nil {
		t.Error("Expected initialized methods map")
	}
}

func TestProtoRegistry_LoadDescriptorSet(t *testing.T) {
	registry := NewProtoRegistry()
	
	// Create a minimal FileDescriptorSet
	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    strPtr("test.proto"),
				Package: strPtr("test"),
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: strPtr("TestMessage"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   strPtr("id"),
								Number: int32Ptr(1),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
								Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							},
						},
					},
				},
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name: strPtr("TestService"),
						Method: []*descriptorpb.MethodDescriptorProto{
							{
								Name:       strPtr("GetTest"),
								InputType:  strPtr(".test.TestMessage"),
								OutputType: strPtr(".test.TestMessage"),
							},
						},
					},
				},
			},
		},
	}
	
	// Test loading from proto
	err := registry.LoadDescriptorSetProto(fds)
	if err != nil {
		t.Fatalf("Failed to load descriptor set: %v", err)
	}
	
	// Verify service was registered
	service, ok := registry.services["test.TestService"]
	if !ok {
		t.Error("Expected service to be registered")
	}
	if service == nil {
		t.Error("Expected non-nil service")
	}
	
	// Verify method was registered
	method, ok := registry.methods["/test.TestService/GetTest"]
	if !ok {
		t.Error("Expected method to be registered")
	}
	if method == nil {
		t.Error("Expected non-nil method")
	}
}

func TestProtoRegistry_GetMethod(t *testing.T) {
	registry := createTestRegistry(t)
	
	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name: "existing method",
			path: "/test.TestService/GetTest",
		},
		{
			name:        "non-existent method",
			path:        "/test.TestService/NonExistent",
			expectError: true,
		},
		{
			name:        "invalid path",
			path:        "invalid",
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, err := registry.GetMethod(tt.path)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if method == nil {
					t.Error("Expected non-nil method")
				}
			}
		})
	}
}

func TestProtoRegistry_GetMessageType(t *testing.T) {
	registry := createTestRegistry(t)
	
	tests := []struct {
		name        string
		typeName    string
		expectError bool
	}{
		{
			name:     "existing message type",
			typeName: "test.TestMessage",
		},
		{
			name:        "non-existent message type",
			typeName:    "test.NonExistent",
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgType, err := registry.GetMessageType(tt.typeName)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if msgType == nil {
					t.Error("Expected non-nil message type")
				}
			}
		})
	}
}

func TestProtoRegistry_JSONToProto(t *testing.T) {
	registry := createTestRegistry(t)
	
	tests := []struct {
		name        string
		json        string
		messageName string
		expectError bool
	}{
		{
			name:        "valid JSON",
			json:        `{"id": "test123"}`,
			messageName: "test.TestMessage",
		},
		{
			name:        "invalid JSON",
			json:        `{invalid json}`,
			messageName: "test.TestMessage",
			expectError: true,
		},
		{
			name:        "non-existent message type",
			json:        `{"id": "test123"}`,
			messageName: "test.NonExistent",
			expectError: true,
		},
		{
			name:        "JSON with unknown fields",
			json:        `{"id": "test123", "unknown": "field"}`,
			messageName: "test.TestMessage",
			// Should not error due to DiscardUnknown option
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protoData, err := registry.JSONToProto([]byte(tt.json), tt.messageName)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(protoData) == 0 {
					t.Error("Expected non-empty proto data")
				}
			}
		})
	}
}

func TestProtoRegistry_ProtoToJSON(t *testing.T) {
	registry := createTestRegistry(t)
	
	// First convert JSON to Proto to get valid proto data
	validProto, err := registry.JSONToProto([]byte(`{"id": "test123"}`), "test.TestMessage")
	if err != nil {
		t.Fatalf("Failed to create test proto data: %v", err)
	}
	
	tests := []struct {
		name        string
		protoData   []byte
		messageName string
		expectError bool
	}{
		{
			name:        "valid proto",
			protoData:   validProto,
			messageName: "test.TestMessage",
		},
		{
			name:        "invalid proto",
			protoData:   []byte("invalid proto data"),
			messageName: "test.TestMessage",
			expectError: true,
		},
		{
			name:        "non-existent message type",
			protoData:   validProto,
			messageName: "test.NonExistent",
			expectError: true,
		},
		{
			name:        "empty proto",
			protoData:   []byte{},
			messageName: "test.TestMessage",
			// Empty proto should work (all fields will be default)
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := registry.ProtoToJSON(tt.protoData, tt.messageName)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(jsonData) == 0 {
					t.Error("Expected non-empty JSON data")
				}
			}
		})
	}
}

func TestProtoRegistry_TranscodeRequest(t *testing.T) {
	registry := createTestRegistry(t)
	
	tests := []struct {
		name        string
		json        string
		methodPath  string
		expectError bool
	}{
		{
			name:       "valid request",
			json:       `{"id": "test123"}`,
			methodPath: "/test.TestService/GetTest",
		},
		{
			name:        "invalid method",
			json:        `{"id": "test123"}`,
			methodPath:  "/test.TestService/NonExistent",
			expectError: true,
		},
		{
			name:        "invalid JSON",
			json:        `{invalid}`,
			methodPath:  "/test.TestService/GetTest",
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protoData, err := registry.TranscodeRequest([]byte(tt.json), tt.methodPath)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(protoData) == 0 {
					t.Error("Expected non-empty proto data")
				}
			}
		})
	}
}

func TestProtoRegistry_TranscodeResponse(t *testing.T) {
	registry := createTestRegistry(t)
	
	// Create valid proto response
	validProto, _ := registry.JSONToProto([]byte(`{"id": "response123"}`), "test.TestMessage")
	
	tests := []struct {
		name        string
		protoData   []byte
		methodPath  string
		expectError bool
	}{
		{
			name:       "valid response",
			protoData:  validProto,
			methodPath: "/test.TestService/GetTest",
		},
		{
			name:        "invalid method",
			protoData:   validProto,
			methodPath:  "/test.TestService/NonExistent",
			expectError: true,
		},
		{
			name:        "invalid proto",
			protoData:   []byte("invalid"),
			methodPath:  "/test.TestService/GetTest",
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := registry.TranscodeResponse(tt.protoData, tt.methodPath)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(jsonData) == 0 {
					t.Error("Expected non-empty JSON data")
				}
			}
		})
	}
}

func TestProtoRegistry_LoadDescriptorFromBase64(t *testing.T) {
	registry := NewProtoRegistry()
	
	// Create a descriptor set and encode to base64
	fds := createTestFileDescriptorSet()
	data, err := proto.Marshal(fds)
	if err != nil {
		t.Fatalf("Failed to marshal descriptor set: %v", err)
	}
	base64Data := base64.StdEncoding.EncodeToString(data)
	
	// Test valid base64
	err = registry.LoadDescriptorFromBase64(base64Data)
	if err != nil {
		t.Errorf("Failed to load from base64: %v", err)
	}
	
	// Verify loaded
	_, err = registry.GetMethod("/test.TestService/GetTest")
	if err != nil {
		t.Error("Expected method to be loaded")
	}
	
	// Test invalid base64
	registry2 := NewProtoRegistry()
	err = registry2.LoadDescriptorFromBase64("invalid base64!")
	if err == nil {
		t.Error("Expected error for invalid base64")
	}
}

func TestProtoRegistry_GetServiceMethods(t *testing.T) {
	registry := createTestRegistry(t)
	
	tests := []struct {
		name         string
		serviceName  string
		expectError  bool
		expectedMethods []string
	}{
		{
			name:         "existing service",
			serviceName:  "test.TestService",
			expectedMethods: []string{"GetTest"},
		},
		{
			name:         "non-existent service",
			serviceName:  "test.NonExistent",
			expectError:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			methods, err := registry.GetServiceMethods(tt.serviceName)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(methods) != len(tt.expectedMethods) {
					t.Errorf("Expected %d methods, got %d", len(tt.expectedMethods), len(methods))
				}
				for i, method := range methods {
					if method != tt.expectedMethods[i] {
						t.Errorf("Expected method %s, got %s", tt.expectedMethods[i], method)
					}
				}
			}
		})
	}
}

func TestProtoRegistry_GetMethodInfo(t *testing.T) {
	registry := createTestRegistry(t)
	
	// Add a streaming method to test
	streamingFds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    strPtr("streaming.proto"),
				Package: strPtr("streaming"),
				Dependency: []string{"test.proto"},
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: strPtr("StreamMessage"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   strPtr("data"),
								Number: int32Ptr(1),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
								Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							},
						},
					},
				},
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name: strPtr("StreamService"),
						Method: []*descriptorpb.MethodDescriptorProto{
							{
								Name:            strPtr("ClientStream"),
								InputType:       strPtr(".streaming.StreamMessage"),
								OutputType:      strPtr(".streaming.StreamMessage"),
								ClientStreaming: boolPtr(true),
							},
							{
								Name:            strPtr("ServerStream"),
								InputType:       strPtr(".streaming.StreamMessage"),
								OutputType:      strPtr(".streaming.StreamMessage"),
								ServerStreaming: boolPtr(true),
							},
							{
								Name:            strPtr("BidiStream"),
								InputType:       strPtr(".streaming.StreamMessage"),
								OutputType:      strPtr(".streaming.StreamMessage"),
								ClientStreaming: boolPtr(true),
								ServerStreaming: boolPtr(true),
							},
						},
					},
				},
			},
		},
	}
	
	registry.LoadDescriptorSetProto(streamingFds)
	
	tests := []struct {
		name               string
		methodPath         string
		expectError        bool
		expectedName       string
		expectedStreaming  bool
		expectedClientStream bool
		expectedServerStream bool
	}{
		{
			name:         "unary method",
			methodPath:   "/test.TestService/GetTest",
			expectedName: "GetTest",
		},
		{
			name:                 "client streaming",
			methodPath:           "/streaming.StreamService/ClientStream",
			expectedName:         "ClientStream",
			expectedStreaming:    true,
			expectedClientStream: true,
		},
		{
			name:                 "server streaming",
			methodPath:           "/streaming.StreamService/ServerStream",
			expectedName:         "ServerStream",
			expectedStreaming:    true,
			expectedServerStream: true,
		},
		{
			name:                 "bidi streaming",
			methodPath:           "/streaming.StreamService/BidiStream",
			expectedName:         "BidiStream",
			expectedStreaming:    true,
			expectedClientStream: true,
			expectedServerStream: true,
		},
		{
			name:        "non-existent method",
			methodPath:  "/test.TestService/NonExistent",
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := registry.GetMethodInfo(tt.methodPath)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if info.Name != tt.expectedName {
					t.Errorf("Expected name %s, got %s", tt.expectedName, info.Name)
				}
				if info.IsStreaming != tt.expectedStreaming {
					t.Errorf("Expected IsStreaming %v, got %v", tt.expectedStreaming, info.IsStreaming)
				}
				if info.ClientStreaming != tt.expectedClientStream {
					t.Errorf("Expected ClientStreaming %v, got %v", tt.expectedClientStream, info.ClientStreaming)
				}
				if info.ServerStreaming != tt.expectedServerStream {
					t.Errorf("Expected ServerStreaming %v, got %v", tt.expectedServerStream, info.ServerStreaming)
				}
			}
		})
	}
}

func TestProtoRegistry_DuplicateRegistration(t *testing.T) {
	registry := NewProtoRegistry()
	
	fds := createTestFileDescriptorSet()
	
	// Load once
	err := registry.LoadDescriptorSetProto(fds)
	if err != nil {
		t.Fatalf("First load failed: %v", err)
	}
	
	// Load again - duplicate registration is expected to fail
	err = registry.LoadDescriptorSetProto(fds)
	if err == nil {
		t.Fatal("Expected error for duplicate registration")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("Expected 'already registered' error, got: %v", err)
	}
	
	// Verify still works
	_, err = registry.GetMethod("/test.TestService/GetTest")
	if err != nil {
		t.Error("Method should still be accessible after duplicate registration")
	}
}

func TestProtoRegistry_EmptyInput(t *testing.T) {
	registry := NewProtoRegistry()
	
	// Test with empty descriptor set
	err := registry.LoadDescriptorSetProto(&descriptorpb.FileDescriptorSet{})
	if err != nil {
		t.Errorf("Loading empty descriptor set should not error: %v", err)
	}
	
	// Test with invalid marshal data
	err = registry.LoadDescriptorSet([]byte("invalid proto data"))
	if err == nil {
		t.Error("Expected error for invalid proto data")
	}
}

// Helper functions
func createTestRegistry(t *testing.T) *ProtoRegistry {
	registry := NewProtoRegistry()
	fds := createTestFileDescriptorSet()
	
	err := registry.LoadDescriptorSetProto(fds)
	if err != nil {
		t.Fatalf("Failed to create test registry: %v", err)
	}
	
	return registry
}

func createTestFileDescriptorSet() *descriptorpb.FileDescriptorSet {
	return &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    strPtr("test.proto"),
				Package: strPtr("test"),
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: strPtr("TestMessage"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   strPtr("id"),
								Number: int32Ptr(1),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
								Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							},
						},
					},
				},
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name: strPtr("TestService"),
						Method: []*descriptorpb.MethodDescriptorProto{
							{
								Name:       strPtr("GetTest"),
								InputType:  strPtr(".test.TestMessage"),
								OutputType: strPtr(".test.TestMessage"),
							},
						},
					},
				},
			},
		},
	}
}

func strPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}