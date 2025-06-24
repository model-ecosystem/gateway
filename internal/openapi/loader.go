package openapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gateway/internal/core"
	"gopkg.in/yaml.v3"
)

// Spec represents an OpenAPI specification
type Spec struct {
	OpenAPI string                 `json:"openapi" yaml:"openapi"`
	Info    Info                   `json:"info" yaml:"info"`
	Servers []Server               `json:"servers" yaml:"servers"`
	Paths   map[string]PathItem    `json:"paths" yaml:"paths"`
	Tags    []Tag                  `json:"tags" yaml:"tags"`
}

// Info represents API information
type Info struct {
	Title       string `json:"title" yaml:"title"`
	Description string `json:"description" yaml:"description"`
	Version     string `json:"version" yaml:"version"`
}

// Server represents a server definition
type Server struct {
	URL         string                    `json:"url" yaml:"url"`
	Description string                    `json:"description" yaml:"description"`
	Variables   map[string]ServerVariable `json:"variables" yaml:"variables"`
}

// ServerVariable represents a server variable
type ServerVariable struct {
	Default     string   `json:"default" yaml:"default"`
	Description string   `json:"description" yaml:"description"`
	Enum        []string `json:"enum" yaml:"enum"`
}

// PathItem represents operations on a path
type PathItem struct {
	Get        *Operation `json:"get,omitempty" yaml:"get,omitempty"`
	Post       *Operation `json:"post,omitempty" yaml:"post,omitempty"`
	Put        *Operation `json:"put,omitempty" yaml:"put,omitempty"`
	Delete     *Operation `json:"delete,omitempty" yaml:"delete,omitempty"`
	Patch      *Operation `json:"patch,omitempty" yaml:"patch,omitempty"`
	Options    *Operation `json:"options,omitempty" yaml:"options,omitempty"`
	Head       *Operation `json:"head,omitempty" yaml:"head,omitempty"`
	Trace      *Operation `json:"trace,omitempty" yaml:"trace,omitempty"`
	Parameters []Parameter `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// Operation represents an API operation
type Operation struct {
	OperationID string                `json:"operationId" yaml:"operationId"`
	Summary     string                `json:"summary" yaml:"summary"`
	Description string                `json:"description" yaml:"description"`
	Tags        []string              `json:"tags" yaml:"tags"`
	Parameters  []Parameter           `json:"parameters" yaml:"parameters"`
	Security    []map[string][]string `json:"security" yaml:"security"`
	Servers     []Server              `json:"servers" yaml:"servers"`
	// Extension fields for gateway configuration
	XGateway *GatewayExtension `json:"x-gateway,omitempty" yaml:"x-gateway,omitempty"`
}

// Parameter represents an operation parameter
type Parameter struct {
	Name        string `json:"name" yaml:"name"`
	In          string `json:"in" yaml:"in"` // query, header, path, cookie
	Description string `json:"description" yaml:"description"`
	Required    bool   `json:"required" yaml:"required"`
}

// Tag represents an API tag
type Tag struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	// Extension for service mapping
	XService string `json:"x-service,omitempty" yaml:"x-service,omitempty"`
}

// GatewayExtension contains gateway-specific configuration
type GatewayExtension struct {
	ServiceName     string                 `json:"serviceName" yaml:"serviceName"`
	LoadBalance     string                 `json:"loadBalance" yaml:"loadBalance"`
	Timeout         int                    `json:"timeout" yaml:"timeout"`
	RateLimit       int                    `json:"rateLimit" yaml:"rateLimit"`
	AuthRequired    bool                   `json:"authRequired" yaml:"authRequired"`
	RequiredScopes  []string               `json:"requiredScopes" yaml:"requiredScopes"`
	Transformations map[string]interface{} `json:"transformations" yaml:"transformations"`
}

// Loader loads OpenAPI specifications
type Loader struct {
	logger     *slog.Logger
	httpClient *http.Client
}

// NewLoader creates a new OpenAPI loader
func NewLoader(logger *slog.Logger) *Loader {
	if logger == nil {
		logger = slog.Default()
	}

	return &Loader{
		logger: logger.With("component", "openapi_loader"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Load loads an OpenAPI spec from a file or URL
func (l *Loader) Load(source string) (*Spec, error) {
	var data []byte
	var err error

	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		// Load from URL
		data, err = l.loadFromURL(source)
	} else {
		// Load from file
		data, err = l.loadFromFile(source)
	}

	if err != nil {
		return nil, err
	}

	// Parse spec
	var spec Spec
	if strings.HasSuffix(source, ".json") {
		err = json.Unmarshal(data, &spec)
	} else {
		err = yaml.Unmarshal(data, &spec)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Validate spec
	if err := l.validateSpec(&spec); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI spec: %w", err)
	}

	l.logger.Info("OpenAPI spec loaded",
		"source", source,
		"title", spec.Info.Title,
		"version", spec.Info.Version,
		"paths", len(spec.Paths),
	)

	return &spec, nil
}

// ParseBytes parses an OpenAPI spec from raw bytes
func (l *Loader) ParseBytes(data []byte) (*Spec, error) {
	var spec Spec
	
	// Try to parse as JSON first
	if err := json.Unmarshal(data, &spec); err != nil {
		// Try YAML if JSON fails
		if err := yaml.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("failed to parse OpenAPI spec as JSON or YAML: %w", err)
		}
	}

	// Validate spec
	if err := l.validateSpec(&spec); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI spec: %w", err)
	}

	l.logger.Debug("OpenAPI spec parsed from bytes",
		"title", spec.Info.Title,
		"version", spec.Info.Version,
		"paths", len(spec.Paths),
	)

	return &spec, nil
}

// LoadDirectory loads all OpenAPI specs from a directory
func (l *Loader) LoadDirectory(dir string) ([]*Spec, error) {
	var specs []*Spec

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Check if it's an OpenAPI file
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			return nil
		}

		// Check if filename indicates OpenAPI
		name := strings.ToLower(info.Name())
		if !strings.Contains(name, "openapi") && !strings.Contains(name, "swagger") {
			return nil
		}

		spec, err := l.Load(path)
		if err != nil {
			l.logger.Warn("Failed to load OpenAPI spec",
				"path", path,
				"error", err,
			)
			return nil // Continue with other files
		}

		specs = append(specs, spec)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return specs, nil
}

// ToRouteRules converts OpenAPI paths to gateway route rules
func (l *Loader) ToRouteRules(spec *Spec, defaultService string) []core.RouteRule {
	var rules []core.RouteRule

	// Create tag to service mapping
	tagServices := make(map[string]string)
	for _, tag := range spec.Tags {
		if tag.XService != "" {
			tagServices[tag.Name] = tag.XService
		}
	}

	// Convert paths to routes
	for path, pathItem := range spec.Paths {
		// Convert OpenAPI path to gateway path pattern
		gatewayPath := convertPath(path)

		// Process each operation
		operations := map[string]*Operation{
			"GET":     pathItem.Get,
			"POST":    pathItem.Post,
			"PUT":     pathItem.Put,
			"DELETE":  pathItem.Delete,
			"PATCH":   pathItem.Patch,
			"OPTIONS": pathItem.Options,
			"HEAD":    pathItem.Head,
			"TRACE":   pathItem.Trace,
		}

		for method, op := range operations {
			if op == nil {
				continue
			}

			// Determine service name
			serviceName := defaultService
			if op.XGateway != nil && op.XGateway.ServiceName != "" {
				serviceName = op.XGateway.ServiceName
			} else if len(op.Tags) > 0 {
				// Use first tag's service mapping
				if svc, ok := tagServices[op.Tags[0]]; ok {
					serviceName = svc
				}
			}

			// Create route rule
			rule := core.RouteRule{
				ID:          generateRouteID(op.OperationID, method, path),
				Path:        gatewayPath,
				Methods:     []string{method},
				ServiceName: serviceName,
				Metadata:    make(map[string]interface{}),
			}

			// Apply gateway extensions
			if op.XGateway != nil {
				if op.XGateway.LoadBalance != "" {
					rule.LoadBalance = core.LoadBalanceStrategy(op.XGateway.LoadBalance)
				}
				if op.XGateway.Timeout > 0 {
					rule.Timeout = time.Duration(op.XGateway.Timeout) * time.Second
				}
				if op.XGateway.RateLimit > 0 {
					rule.Metadata["rateLimit"] = op.XGateway.RateLimit
				}
				rule.Metadata["authRequired"] = op.XGateway.AuthRequired
				if len(op.XGateway.RequiredScopes) > 0 {
					rule.Metadata["requiredScopes"] = op.XGateway.RequiredScopes
				}
				if op.XGateway.Transformations != nil {
					rule.Metadata["transformations"] = op.XGateway.Transformations
				}
			}

			// Add operation metadata
			rule.Metadata["operationId"] = op.OperationID
			rule.Metadata["summary"] = op.Summary
			if len(op.Tags) > 0 {
				rule.Metadata["tags"] = op.Tags
			}

			rules = append(rules, rule)
		}
	}

	l.logger.Info("Converted OpenAPI to routes",
		"spec", spec.Info.Title,
		"routes", len(rules),
	)

	return rules
}

// Helper functions

func (l *Loader) loadFromFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (l *Loader) loadFromURL(url string) ([]byte, error) {
	resp, err := l.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func (l *Loader) validateSpec(spec *Spec) error {
	if spec.OpenAPI == "" {
		return fmt.Errorf("missing openapi version")
	}

	if !strings.HasPrefix(spec.OpenAPI, "3.") {
		return fmt.Errorf("unsupported OpenAPI version: %s (only 3.x supported)", spec.OpenAPI)
	}

	if spec.Info.Title == "" {
		return fmt.Errorf("missing info.title")
	}

	if spec.Info.Version == "" {
		return fmt.Errorf("missing info.version")
	}

	if len(spec.Paths) == 0 {
		return fmt.Errorf("no paths defined")
	}

	return nil
}

// convertPath converts OpenAPI path to gateway pattern
func convertPath(path string) string {
	// Convert {param} to * for simple wildcard matching
	// This is a simplified conversion - could be enhanced
	result := path
	for {
		start := strings.Index(result, "{")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		result = result[:start] + "*" + result[start+end+1:]
	}
	return result
}

// generateRouteID generates a unique route ID
func generateRouteID(operationID, method, path string) string {
	if operationID != "" {
		return operationID
	}

	// Generate from method and path
	id := strings.ToLower(method) + "_" + path
	id = strings.ReplaceAll(id, "/", "_")
	id = strings.ReplaceAll(id, "{", "")
	id = strings.ReplaceAll(id, "}", "")
	id = strings.ReplaceAll(id, "*", "param")
	id = strings.Trim(id, "_")

	return id
}
