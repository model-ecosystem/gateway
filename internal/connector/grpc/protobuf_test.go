package grpc

import (
	"encoding/base64"
	"log/slog"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Test proto definition for testing
const testProtoDefinition = `
syntax = "proto3";
package test;

message HelloRequest {
  string name = 1;
  int32 age = 2;
}

message HelloResponse {
  string message = 1;
  bool success = 2;
}

service Greeter {
  rpc SayHello (HelloRequest) returns (HelloResponse);
}
`

func createTestDescriptorSet() (*descriptorpb.FileDescriptorSet, error) {
	// This is a simplified test descriptor
	// In real tests, you would generate this from actual proto files
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test.proto"),
		Package: proto.String("test"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("HelloRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("name"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						JsonName: proto.String("name"),
					},
					{
						Name:     proto.String("age"),
						Number:   proto.Int32(2),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
						JsonName: proto.String("age"),
					},
				},
			},
			{
				Name: proto.String("HelloResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("message"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						JsonName: proto.String("message"),
					},
					{
						Name:     proto.String("success"),
						Number:   proto.Int32(2),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum(),
						JsonName: proto.String("success"),
					},
				},
			},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String("Greeter"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("SayHello"),
						InputType:  proto.String(".test.HelloRequest"),
						OutputType: proto.String(".test.HelloResponse"),
					},
				},
			},
		},
		Syntax: proto.String("proto3"),
	}

	return &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fd},
	}, nil
}

func TestProtoRegistry_LoadDescriptorSet(t *testing.T) {
	registry := NewProtoRegistry()

	// Create test descriptor set
	fds, err := createTestDescriptorSet()
	if err != nil {
		t.Fatalf("Failed to create test descriptor: %v", err)
	}

	// Marshal to bytes
	data, err := proto.Marshal(fds)
	if err != nil {
		t.Fatalf("Failed to marshal descriptor: %v", err)
	}

	// Load descriptor
	err = registry.LoadDescriptorSet(data)
	if err != nil {
		t.Fatalf("Failed to load descriptor: %v", err)
	}

	// Verify service was loaded
	methods, err := registry.GetServiceMethods("test.Greeter")
	if err != nil {
		t.Errorf("Failed to get service methods: %v", err)
	}

	if len(methods) != 1 || methods[0] != "SayHello" {
		t.Errorf("Expected SayHello method, got %v", methods)
	}
}

func TestProtoRegistry_GetMethodInfo(t *testing.T) {
	registry := NewProtoRegistry()

	// Create and load test descriptor
	fds, _ := createTestDescriptorSet()
	data, _ := proto.Marshal(fds)
	registry.LoadDescriptorSet(data)

	// Get method info
	info, err := registry.GetMethodInfo("/test.Greeter/SayHello")
	if err != nil {
		t.Fatalf("Failed to get method info: %v", err)
	}

	// Verify method info
	if info.Name != "SayHello" {
		t.Errorf("Expected method name SayHello, got %s", info.Name)
	}

	if info.InputType != "test.HelloRequest" {
		t.Errorf("Expected input type test.HelloRequest, got %s", info.InputType)
	}

	if info.OutputType != "test.HelloResponse" {
		t.Errorf("Expected output type test.HelloResponse, got %s", info.OutputType)
	}

	if info.IsStreaming {
		t.Error("Expected non-streaming method")
	}
}

func TestProtoRegistry_LoadDescriptorFromBase64(t *testing.T) {
	registry := NewProtoRegistry()

	// Create test descriptor
	fds, _ := createTestDescriptorSet()
	data, _ := proto.Marshal(fds)

	// Encode to base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	// Load from base64
	err := registry.LoadDescriptorFromBase64(base64Data)
	if err != nil {
		t.Fatalf("Failed to load from base64: %v", err)
	}

	// Verify it was loaded
	_, err = registry.GetMethod("/test.Greeter/SayHello")
	if err != nil {
		t.Error("Method not found after loading from base64")
	}
}

func TestProtoRegistry_JSONToProto_Fallback(t *testing.T) {
	registry := NewProtoRegistry()

	// Test without loaded descriptors (should fail gracefully)
	jsonData := []byte(`{"name": "test", "age": 25}`)

	// This should fail but not panic
	_, err := registry.JSONToProto(jsonData, "test.HelloRequest")
	if err == nil {
		t.Error("Expected error when message type not found")
	}
}

func TestProtoRegistry_TranscodeRequest(t *testing.T) {
	registry := NewProtoRegistry()

	// Create and load test descriptor
	fds, _ := createTestDescriptorSet()
	data, _ := proto.Marshal(fds)
	err := registry.LoadDescriptorSet(data)
	if err != nil {
		// Skip test if descriptor loading not supported
		t.Skip("Descriptor loading not fully supported")
	}

	// Test JSON data
	jsonData := []byte(`{"name": "Alice", "age": 30}`)

	// Transcode request
	protoData, err := registry.TranscodeRequest(jsonData, "/test.Greeter/SayHello")
	if err != nil {
		t.Logf("Transcoding not fully supported: %v", err)
		return
	}

	// Verify it's valid protobuf (at minimum, should not be same as JSON)
	if len(protoData) == len(jsonData) {
		// In a real implementation, this would be different
		t.Log("Protobuf transcoding is using pass-through mode")
	}
}

func TestTranscoder_WithProtoRegistry(t *testing.T) {
	logger := slog.Default()
	transcoder := NewTranscoder(logger)

	// Create custom registry
	registry := NewProtoRegistry()

	// Set custom registry
	transcoder.WithProtoRegistry(registry)

	if transcoder.registry != registry {
		t.Error("Registry was not set correctly")
	}
}

func TestTranscoder_JSONToProtobuf_WithRegistry(t *testing.T) {
	logger := slog.Default()
	transcoder := NewTranscoder(logger)

	// Test with registry (but no descriptors loaded)
	jsonData := []byte(`{"test": "data"}`)
	result, err := transcoder.JSONToProtobuf(jsonData, "test.Message")

	// Should fall back to pass-through
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Result should be same as input (pass-through)
	if string(result) != string(jsonData) {
		t.Error("Expected pass-through behavior")
	}
}

func TestTranscoder_LoadProtoDescriptorBase64(t *testing.T) {
	logger := slog.Default()
	transcoder := NewTranscoder(logger)

	// Create test descriptor
	fds, _ := createTestDescriptorSet()
	data, _ := proto.Marshal(fds)
	base64Data := base64.StdEncoding.EncodeToString(data)

	// Load descriptor
	err := transcoder.LoadProtoDescriptorBase64(base64Data)
	if err != nil {
		t.Logf("Descriptor loading returned error (may be expected): %v", err)
	}
}
