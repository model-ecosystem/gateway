package grpc

import (
	"encoding/base64"
	"fmt"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// ProtoRegistry manages protobuf descriptors for transcoding
type ProtoRegistry struct {
	files    *protoregistry.Files
	types    *protoregistry.Types
	services map[string]protoreflect.ServiceDescriptor
	methods  map[string]protoreflect.MethodDescriptor
}

// NewProtoRegistry creates a new protobuf registry
func NewProtoRegistry() *ProtoRegistry {
	// Create isolated registries to avoid conflicts
	return &ProtoRegistry{
		files:    new(protoregistry.Files),
		types:    new(protoregistry.Types),
		services: make(map[string]protoreflect.ServiceDescriptor),
		methods:  make(map[string]protoreflect.MethodDescriptor),
	}
}

// LoadDescriptorSet loads a FileDescriptorSet from bytes
func (r *ProtoRegistry) LoadDescriptorSet(data []byte) error {
	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(data, &fds); err != nil {
		return fmt.Errorf("failed to unmarshal descriptor set: %w", err)
	}

	return r.LoadDescriptorSetProto(&fds)
}

// LoadDescriptorSetProto loads a FileDescriptorSet proto
func (r *ProtoRegistry) LoadDescriptorSetProto(fds *descriptorpb.FileDescriptorSet) error {
	// Register all files
	for _, fdp := range fds.File {
		fd, err := protodesc.NewFile(fdp, r.files)
		if err != nil {
			return fmt.Errorf("failed to create file descriptor: %w", err)
		}

		if err := r.files.RegisterFile(fd); err != nil {
			return fmt.Errorf("failed to register file: %w", err)
		}

		// Register services and methods
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			service := services.Get(i)
			serviceName := string(service.FullName())
			r.services[serviceName] = service

			methods := service.Methods()
			for j := 0; j < methods.Len(); j++ {
				method := methods.Get(j)
				methodPath := fmt.Sprintf("/%s/%s", serviceName, method.Name())
				r.methods[methodPath] = method
			}
		}

		// Register message types
		messages := fd.Messages()
		for i := 0; i < messages.Len(); i++ {
			msg := messages.Get(i)
			msgType := dynamicpb.NewMessageType(msg)
			if err := r.types.RegisterMessage(msgType); err != nil {
				// Ignore duplicate registration errors
				if !strings.Contains(err.Error(), "already registered") {
					return fmt.Errorf("failed to register message type: %w", err)
				}
			}
		}
	}

	return nil
}

// GetMethod returns the method descriptor for a gRPC method path
func (r *ProtoRegistry) GetMethod(path string) (protoreflect.MethodDescriptor, error) {
	method, ok := r.methods[path]
	if !ok {
		return nil, fmt.Errorf("method not found: %s", path)
	}
	return method, nil
}

// GetMessageType returns the message type for a fully qualified name
func (r *ProtoRegistry) GetMessageType(name string) (protoreflect.MessageType, error) {
	msgType, err := r.types.FindMessageByName(protoreflect.FullName(name))
	if err != nil {
		return nil, fmt.Errorf("message type not found: %s", name)
	}
	return msgType, nil
}

// JSONToProto converts JSON to protobuf using the message descriptor
func (r *ProtoRegistry) JSONToProto(jsonData []byte, messageName string) ([]byte, error) {
	// Get message type
	msgType, err := r.GetMessageType(messageName)
	if err != nil {
		return nil, err
	}

	// Create new message instance
	msg := msgType.New()

	// Unmarshal JSON
	unmarshaler := protojson.UnmarshalOptions{
		DiscardUnknown: true,
		AllowPartial:   true,
	}

	if err := unmarshaler.Unmarshal(jsonData, msg.Interface()); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Marshal to protobuf
	protoData, err := proto.Marshal(msg.Interface())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal protobuf: %w", err)
	}

	return protoData, nil
}

// ProtoToJSON converts protobuf to JSON using the message descriptor
func (r *ProtoRegistry) ProtoToJSON(protoData []byte, messageName string) ([]byte, error) {
	// Get message type
	msgType, err := r.GetMessageType(messageName)
	if err != nil {
		return nil, err
	}

	// Create new message instance
	msg := msgType.New()

	// Unmarshal protobuf
	if err := proto.Unmarshal(protoData, msg.Interface()); err != nil {
		return nil, fmt.Errorf("failed to unmarshal protobuf: %w", err)
	}

	// Marshal to JSON
	marshaler := protojson.MarshalOptions{
		Multiline:       false,
		Indent:          "",
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}

	jsonData, err := marshaler.Marshal(msg.Interface())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return jsonData, nil
}

// TranscodeRequest transcodes an HTTP request to gRPC format
func (r *ProtoRegistry) TranscodeRequest(jsonData []byte, methodPath string) ([]byte, error) {
	// Get method descriptor
	method, err := r.GetMethod(methodPath)
	if err != nil {
		return nil, err
	}

	// Get input message type name
	inputType := string(method.Input().FullName())

	// Convert JSON to protobuf
	return r.JSONToProto(jsonData, inputType)
}

// TranscodeResponse transcodes a gRPC response to HTTP format
func (r *ProtoRegistry) TranscodeResponse(protoData []byte, methodPath string) ([]byte, error) {
	// Get method descriptor
	method, err := r.GetMethod(methodPath)
	if err != nil {
		return nil, err
	}

	// Get output message type name
	outputType := string(method.Output().FullName())

	// Convert protobuf to JSON
	return r.ProtoToJSON(protoData, outputType)
}

// LoadDescriptorFromBase64 loads a descriptor set from base64 encoded string
func (r *ProtoRegistry) LoadDescriptorFromBase64(base64Data string) error {
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("failed to decode base64: %w", err)
	}

	return r.LoadDescriptorSet(data)
}

// GetServiceMethods returns all methods for a service
func (r *ProtoRegistry) GetServiceMethods(serviceName string) ([]string, error) {
	service, ok := r.services[serviceName]
	if !ok {
		return nil, fmt.Errorf("service not found: %s", serviceName)
	}

	methods := service.Methods()
	result := make([]string, 0, methods.Len())

	for i := 0; i < methods.Len(); i++ {
		method := methods.Get(i)
		result = append(result, string(method.Name()))
	}

	return result, nil
}

// GetMethodInfo returns information about a method
func (r *ProtoRegistry) GetMethodInfo(methodPath string) (*MethodInfo, error) {
	method, err := r.GetMethod(methodPath)
	if err != nil {
		return nil, err
	}

	return &MethodInfo{
		Name:            string(method.Name()),
		FullName:        string(method.FullName()),
		InputType:       string(method.Input().FullName()),
		OutputType:      string(method.Output().FullName()),
		IsStreaming:     method.IsStreamingClient() || method.IsStreamingServer(),
		ClientStreaming: method.IsStreamingClient(),
		ServerStreaming: method.IsStreamingServer(),
	}, nil
}

// MethodInfo contains information about a gRPC method
type MethodInfo struct {
	Name            string
	FullName        string
	InputType       string
	OutputType      string
	IsStreaming     bool
	ClientStreaming bool
	ServerStreaming bool
}
