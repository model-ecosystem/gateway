package grpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gateway/internal/core"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// Transcoder handles HTTP to gRPC transcoding
//
// Current Implementation Status:
// This transcoder currently operates in JSON pass-through mode, which means:
// - HTTP requests with JSON bodies are passed directly to gRPC services
// - gRPC responses are assumed to be JSON and passed back to HTTP clients
// - No actual protobuf encoding/decoding is performed
//
// This approach works when:
// - The gRPC service accepts JSON encoding (gRPC-JSON transcoding)
// - The backend service handles JSON serialization internally
// - You're using gRPC services that support JSON codec
//
// Limitations:
// - Does not perform true protobuf message conversion
// - Requires gRPC services to accept JSON payloads
// - No validation against protobuf schemas
// - No support for streaming RPCs
// - No support for binary protobuf format
//
// Future Implementation (Planned):
// - Full protobuf descriptor support via ProtoRegistry
// - Automatic JSON to protobuf conversion based on service definitions
// - Support for streaming RPCs (server-streaming, client-streaming, bidirectional)
// - Field mapping and validation according to proto schemas
// - Support for gRPC-Web protocol
//
// To use the current implementation:
// 1. Ensure your gRPC services accept JSON encoding
// 2. Configure the gateway to route to gRPC backends
// 3. Send HTTP requests with JSON bodies to the appropriate paths
type Transcoder struct {
	logger   *slog.Logger
	registry *ProtoRegistry
}

// NewTranscoder creates a new transcoder
func NewTranscoder(logger *slog.Logger) *Transcoder {
	return &Transcoder{
		logger:   logger,
		registry: NewProtoRegistry(),
	}
}

// WithProtoRegistry sets a custom proto registry
func (t *Transcoder) WithProtoRegistry(registry *ProtoRegistry) *Transcoder {
	t.registry = registry
	return t
}

// HTTPToGRPC converts HTTP request to gRPC format
//
// Current behavior:
// - Extracts service and method from URL path (expects /package.Service/Method format)
// - Reads the request body as-is (assumes JSON format)
// - Does NOT perform JSON to protobuf conversion
// - Passes through the JSON body directly to the gRPC connector
//
// Note: This requires the gRPC backend to accept JSON-encoded requests,
// which is supported by services using the gRPC-JSON transcoding feature
// or custom JSON codecs.
func (t *Transcoder) HTTPToGRPC(req core.Request) (*GRPCRequest, error) {
	// Parse path to extract service and method
	// Expected format: /package.Service/Method
	parts := strings.Split(strings.TrimPrefix(req.Path(), "/"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid gRPC path format: %s", req.Path())
	}

	service := parts[0]
	method := parts[1]

	// Read HTTP body as JSON
	body, err := io.ReadAll(req.Body())
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Pass through JSON directly without protobuf conversion
	// This assumes the gRPC service accepts JSON encoding
	grpcReq := &GRPCRequest{
		Service: service,
		Method:  method,
		Body:    body,
		Headers: req.Headers(),
	}

	t.logger.Debug("transcoded HTTP to gRPC",
		"service", service,
		"method", method,
		"bodySize", len(body),
	)

	return grpcReq, nil
}

// GRPCToHTTP converts gRPC response to HTTP format
func (t *Transcoder) GRPCToHTTP(resp []byte, headers map[string][]string) *HTTPResponse {
	// For now, pass through response directly
	// In a real implementation, this would convert from protobuf to JSON
	httpResp := &HTTPResponse{
		StatusCode: http.StatusOK,
		Headers:    make(map[string][]string),
		Body:       resp,
	}

	// Copy headers
	for k, v := range headers {
		httpResp.Headers[k] = v
	}

	// Set content type if not present
	if _, ok := httpResp.Headers["Content-Type"]; !ok {
		httpResp.Headers["Content-Type"] = []string{"application/json"}
	}

	return httpResp
}

// GRPCRequest represents a transcoded gRPC request
type GRPCRequest struct {
	Service string
	Method  string
	Body    []byte
	Headers map[string][]string
}

// HTTPResponse represents a transcoded HTTP response
type HTTPResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}

// ToResponse converts to core.Response
func (r *HTTPResponse) ToResponse() core.Response {
	return &httpResponse{
		statusCode: r.StatusCode,
		headers:    r.Headers,
		body:       r.Body,
	}
}

// httpResponse implements core.Response
type httpResponse struct {
	statusCode int
	headers    map[string][]string
	body       []byte
}

func (r *httpResponse) StatusCode() int {
	return r.statusCode
}

func (r *httpResponse) Headers() map[string][]string {
	return r.headers
}

func (r *httpResponse) Body() io.ReadCloser {
	return io.NopCloser(bytes.NewReader(r.body))
}

// TranscodingRules defines how to map HTTP paths to gRPC methods
type TranscodingRules struct {
	// PathPrefix to gRPC service mapping
	// Example: "/api/users" -> "users.UserService"
	PathMappings map[string]string

	// HTTP method to gRPC method mapping
	// Example: "GET /users/{id}" -> "GetUser"
	MethodMappings map[string]string
}

// ApplyRules applies transcoding rules to convert HTTP path to gRPC method
func (t *Transcoder) ApplyRules(req core.Request, rules *TranscodingRules) (string, error) {
	path := req.Path()
	method := req.Method()

	// Find matching path prefix
	var service string
	for prefix, svc := range rules.PathMappings {
		if strings.HasPrefix(path, prefix) {
			service = svc
			break
		}
	}

	if service == "" {
		return "", fmt.Errorf("no service mapping found for path: %s", path)
	}

	// Find matching method
	key := fmt.Sprintf("%s %s", method, path)
	grpcMethod, ok := rules.MethodMappings[key]
	if !ok {
		// Try with path pattern
		for pattern, gm := range rules.MethodMappings {
			if matchesPattern(key, pattern) {
				grpcMethod = gm
				break
			}
		}
	}

	if grpcMethod == "" {
		return "", fmt.Errorf("no method mapping found for: %s", key)
	}

	return fmt.Sprintf("/%s/%s", service, grpcMethod), nil
}

// matchesPattern checks if a request matches a pattern
// Simple implementation - in production would use proper pattern matching
func matchesPattern(request, pattern string) bool {
	// Basic wildcard matching
	if strings.Contains(pattern, "{") {
		// Extract the fixed parts
		parts := strings.Split(pattern, "{")
		if len(parts) > 0 && strings.HasPrefix(request, parts[0]) {
			return true
		}
	}
	return request == pattern
}

// JSONToProtobuf converts JSON to protobuf format
func (t *Transcoder) JSONToProtobuf(jsonData []byte, messageType string) ([]byte, error) {
	// If no registry is available, fall back to pass-through
	if t.registry == nil {
		// Validate JSON at least
		var v interface{}
		if err := json.Unmarshal(jsonData, &v); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		return jsonData, nil
	}

	// Use registry to convert
	protoData, err := t.registry.JSONToProto(jsonData, messageType)
	if err != nil {
		// If conversion fails, log and fall back to pass-through
		t.logger.Warn("failed to convert JSON to protobuf, using pass-through",
			"error", err,
			"messageType", messageType,
		)
		return jsonData, nil
	}

	return protoData, nil
}

// ProtobufToJSON converts protobuf to JSON format
func (t *Transcoder) ProtobufToJSON(protoData []byte, messageType string) ([]byte, error) {
	// If no registry is available, fall back to pass-through
	if t.registry == nil {
		return protoData, nil
	}

	// Use registry to convert
	jsonData, err := t.registry.ProtoToJSON(protoData, messageType)
	if err != nil {
		// If conversion fails, log and fall back to pass-through
		t.logger.Warn("failed to convert protobuf to JSON, using pass-through",
			"error", err,
			"messageType", messageType,
		)
		return protoData, nil
	}

	return jsonData, nil
}

// TranscodeRequestWithMethod transcodes HTTP request to gRPC using method info
func (t *Transcoder) TranscodeRequestWithMethod(jsonData []byte, methodPath string) ([]byte, error) {
	if t.registry == nil {
		return jsonData, nil
	}

	return t.registry.TranscodeRequest(jsonData, methodPath)
}

// TranscodeResponseWithMethod transcodes gRPC response to HTTP using method info
func (t *Transcoder) TranscodeResponseWithMethod(protoData []byte, methodPath string) ([]byte, error) {
	if t.registry == nil {
		return protoData, nil
	}

	return t.registry.TranscodeResponse(protoData, methodPath)
}

// LoadProtoDescriptor loads protobuf descriptors from file
func (t *Transcoder) LoadProtoDescriptor(descriptorData []byte) error {
	if t.registry == nil {
		return fmt.Errorf("no proto registry available")
	}

	return t.registry.LoadDescriptorSet(descriptorData)
}

// LoadProtoDescriptorBase64 loads protobuf descriptors from base64 string
func (t *Transcoder) LoadProtoDescriptorBase64(base64Data string) error {
	if t.registry == nil {
		return fmt.Errorf("no proto registry available")
	}

	return t.registry.LoadDescriptorFromBase64(base64Data)
}
