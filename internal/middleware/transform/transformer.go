package transform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
)

// Transformer defines the interface for request/response transformations
type Transformer interface {
	Transform(data []byte, contentType string) ([]byte, error)
	GetContentType() string
}

// JSONTransformer transforms JSON data
type JSONTransformer struct {
	operations []Operation
	logger     *slog.Logger
}

// Operation represents a transformation operation
type Operation struct {
	Type   string                 `yaml:"type"` // add, remove, rename, modify, filter
	Path   string                 `yaml:"path"` // JSON path
	Value  interface{}            `yaml:"value,omitempty"`
	From   string                 `yaml:"from,omitempty"`  // For rename
	To     string                 `yaml:"to,omitempty"`    // For rename
	Filter func(interface{}) bool `yaml:"-"`               // For filter operations
	Script string                 `yaml:"script,omitempty"` // For script-based transforms
}

// NewJSONTransformer creates a new JSON transformer
func NewJSONTransformer(operations []Operation, logger *slog.Logger) *JSONTransformer {
	if logger == nil {
		logger = slog.Default()
	}

	return &JSONTransformer{
		operations: operations,
		logger:     logger.With("transformer", "json"),
	}
}

// Transform applies transformations to JSON data
func (t *JSONTransformer) Transform(data []byte, contentType string) ([]byte, error) {
	if !strings.Contains(contentType, "json") {
		return data, nil
	}

	// Parse JSON
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Apply operations
	for _, op := range t.operations {
		var err error
		jsonData, err = t.applyOperation(jsonData, op)
		if err != nil {
			t.logger.Error("Failed to apply operation",
				"type", op.Type,
				"path", op.Path,
				"error", err,
			)
			// Continue with other operations
		}
	}

	// Marshal back to JSON
	transformed, err := json.Marshal(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return transformed, nil
}

// GetContentType returns the content type this transformer produces
func (t *JSONTransformer) GetContentType() string {
	return "application/json"
}

// applyOperation applies a single operation
func (t *JSONTransformer) applyOperation(data interface{}, op Operation) (interface{}, error) {
	switch op.Type {
	case "add":
		return t.addField(data, op.Path, op.Value)
	case "remove":
		return t.removeField(data, op.Path)
	case "rename":
		return t.renameField(data, op.From, op.To)
	case "modify":
		return t.modifyField(data, op.Path, op.Value)
	case "filter":
		return t.filterField(data, op.Path, op.Filter)
	default:
		return data, fmt.Errorf("unknown operation type: %s", op.Type)
	}
}

// addField adds a field at the specified path
func (t *JSONTransformer) addField(data interface{}, path string, value interface{}) (interface{}, error) {
	parts := strings.Split(path, ".")
	return t.setValueAtPath(data, parts, value, true)
}

// removeField removes a field at the specified path
func (t *JSONTransformer) removeField(data interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	return t.removeValueAtPath(data, parts)
}

// renameField renames a field
func (t *JSONTransformer) renameField(data interface{}, from, to string) (interface{}, error) {
	// Get value at 'from' path
	fromParts := strings.Split(from, ".")
	value, exists := t.getValueAtPath(data, fromParts)
	if !exists {
		return data, nil // Field doesn't exist, nothing to rename
	}

	// Remove from old path
	data, err := t.removeValueAtPath(data, fromParts)
	if err != nil {
		return data, err
	}

	// Add to new path
	toParts := strings.Split(to, ".")
	return t.setValueAtPath(data, toParts, value, true)
}

// modifyField modifies a field value
func (t *JSONTransformer) modifyField(data interface{}, path string, value interface{}) (interface{}, error) {
	parts := strings.Split(path, ".")
	return t.setValueAtPath(data, parts, value, false)
}

// filterField filters array elements
func (t *JSONTransformer) filterField(data interface{}, path string, filter func(interface{}) bool) (interface{}, error) {
	if filter == nil {
		return data, nil
	}

	parts := strings.Split(path, ".")
	return t.filterValueAtPath(data, parts, filter)
}

// Helper methods for path-based operations

func (t *JSONTransformer) getValueAtPath(data interface{}, parts []string) (interface{}, bool) {
	if len(parts) == 0 {
		return data, true
	}

	switch v := data.(type) {
	case map[string]interface{}:
		if val, ok := v[parts[0]]; ok {
			return t.getValueAtPath(val, parts[1:])
		}
	case []interface{}:
		// Handle array indices
		if parts[0] == "*" {
			// Return entire array for wildcard
			return v, true
		}
	}

	return nil, false
}

func (t *JSONTransformer) setValueAtPath(data interface{}, parts []string, value interface{}, create bool) (interface{}, error) {
	if len(parts) == 0 {
		return value, nil
	}

	switch v := data.(type) {
	case map[string]interface{}:
		if len(parts) == 1 {
			v[parts[0]] = value
			return v, nil
		}

		// Recurse deeper
		if next, ok := v[parts[0]]; ok {
			updated, err := t.setValueAtPath(next, parts[1:], value, create)
			if err != nil {
				return v, err
			}
			v[parts[0]] = updated
		} else if create {
			// Create intermediate structure
			newMap := make(map[string]interface{})
			updated, err := t.setValueAtPath(newMap, parts[1:], value, create)
			if err != nil {
				return v, err
			}
			v[parts[0]] = updated
		}
		return v, nil
	}

	return data, fmt.Errorf("cannot set value at path: %s", strings.Join(parts, "."))
}

func (t *JSONTransformer) removeValueAtPath(data interface{}, parts []string) (interface{}, error) {
	if len(parts) == 0 {
		return nil, nil
	}

	switch v := data.(type) {
	case map[string]interface{}:
		if len(parts) == 1 {
			delete(v, parts[0])
			return v, nil
		}

		// Recurse deeper
		if next, ok := v[parts[0]]; ok {
			updated, err := t.removeValueAtPath(next, parts[1:])
			if err != nil {
				return v, err
			}
			v[parts[0]] = updated
		}
		return v, nil
	}

	return data, fmt.Errorf("cannot remove value at path: %s", strings.Join(parts, "."))
}

func (t *JSONTransformer) filterValueAtPath(data interface{}, parts []string, filter func(interface{}) bool) (interface{}, error) {
	if len(parts) == 0 {
		// Apply filter to array
		if arr, ok := data.([]interface{}); ok {
			filtered := make([]interface{}, 0)
			for _, item := range arr {
				if filter(item) {
					filtered = append(filtered, item)
				}
			}
			return filtered, nil
		}
		return data, nil
	}

	switch v := data.(type) {
	case map[string]interface{}:
		if next, ok := v[parts[0]]; ok {
			updated, err := t.filterValueAtPath(next, parts[1:], filter)
			if err != nil {
				return v, err
			}
			v[parts[0]] = updated
		}
		return v, nil
	}

	return data, fmt.Errorf("cannot filter value at path: %s", strings.Join(parts, "."))
}

// HeaderTransformer transforms HTTP headers
type HeaderTransformer struct {
	add    map[string]string
	remove []string
	rename map[string]string
	modify map[string]*regexp.Regexp
	logger *slog.Logger
}

// NewHeaderTransformer creates a new header transformer
func NewHeaderTransformer(config HeaderConfig, logger *slog.Logger) *HeaderTransformer {
	if logger == nil {
		logger = slog.Default()
	}

	t := &HeaderTransformer{
		add:    config.Add,
		remove: config.Remove,
		rename: config.Rename,
		modify: make(map[string]*regexp.Regexp),
		logger: logger.With("transformer", "header"),
	}

	// Compile regex patterns for modify operations
	for header, pattern := range config.Modify {
		if regex, err := regexp.Compile(pattern); err == nil {
			t.modify[header] = regex
		} else {
			t.logger.Error("Failed to compile regex",
				"header", header,
				"pattern", pattern,
				"error", err,
			)
		}
	}

	return t
}

// TransformHeaders transforms HTTP headers
func (t *HeaderTransformer) TransformHeaders(headers map[string][]string) map[string][]string {
	result := make(map[string][]string)

	// Copy existing headers
	for k, v := range headers {
		result[k] = v
	}

	// Add headers
	for k, v := range t.add {
		result[k] = []string{v}
	}

	// Remove headers
	for _, h := range t.remove {
		delete(result, h)
	}

	// Rename headers
	for from, to := range t.rename {
		if val, exists := result[from]; exists {
			result[to] = val
			delete(result, from)
		}
	}

	// Modify headers
	for header, regex := range t.modify {
		if vals, exists := result[header]; exists {
			for i, val := range vals {
				vals[i] = regex.ReplaceAllString(val, "")
			}
			result[header] = vals
		}
	}

	return result
}

// HeaderConfig represents header transformation configuration
type HeaderConfig struct {
	Add    map[string]string `yaml:"add"`
	Remove []string          `yaml:"remove"`
	Rename map[string]string `yaml:"rename"`
	Modify map[string]string `yaml:"modify"` // Header -> regex pattern
}

// BodyTransformer wraps a ReadCloser with transformation
type BodyTransformer struct {
	original    io.ReadCloser
	transformed io.Reader
	buffer      *bytes.Buffer
}

// NewBodyTransformer creates a new body transformer
func NewBodyTransformer(body io.ReadCloser, transformer Transformer, contentType string) (*BodyTransformer, error) {
	// Read original body
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	body.Close()

	// Transform data
	transformed, err := transformer.Transform(data, contentType)
	if err != nil {
		// Return original on error
		return &BodyTransformer{
			original:    io.NopCloser(bytes.NewReader(data)),
			transformed: bytes.NewReader(data),
			buffer:      bytes.NewBuffer(data),
		}, nil
	}

	return &BodyTransformer{
		original:    io.NopCloser(bytes.NewReader(data)),
		transformed: bytes.NewReader(transformed),
		buffer:      bytes.NewBuffer(transformed),
	}, nil
}

// Read implements io.Reader
func (t *BodyTransformer) Read(p []byte) (n int, err error) {
	return t.transformed.Read(p)
}

// Close implements io.Closer
func (t *BodyTransformer) Close() error {
	return nil
}
