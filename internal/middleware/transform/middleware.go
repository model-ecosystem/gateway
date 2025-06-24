package transform

import (
	"context"
	"io"
	"log/slog"
	"strings"

	"gateway/internal/core"
)

// Config represents transformation middleware configuration
type Config struct {
	Enabled             bool                          `yaml:"enabled"`
	RequestTransforms   map[string]TransformConfig    `yaml:"request"`  // Path pattern -> transform
	ResponseTransforms  map[string]TransformConfig    `yaml:"response"` // Path pattern -> transform
	GlobalRequest       *TransformConfig              `yaml:"globalRequest"`
	GlobalResponse      *TransformConfig              `yaml:"globalResponse"`
}

// TransformConfig represents transformation configuration
type TransformConfig struct {
	Headers    *HeaderConfig `yaml:"headers"`
	Body       *BodyConfig   `yaml:"body"`
	Conditions []Condition   `yaml:"conditions"` // Apply only if conditions match
}

// BodyConfig represents body transformation configuration
type BodyConfig struct {
	Operations []Operation `yaml:"operations"`
	Format     string      `yaml:"format"` // json, xml, etc.
}

// Condition represents a transformation condition
type Condition struct {
	Header      string `yaml:"header"`
	Value       string `yaml:"value"`
	ContentType string `yaml:"contentType"`
	Method      string `yaml:"method"`
}

// Middleware implements request/response transformation
type Middleware struct {
	config  *Config
	logger  *slog.Logger
	matchers map[string]func(string) bool
}

// NewMiddleware creates a new transformation middleware
func NewMiddleware(config *Config, logger *slog.Logger) *Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	m := &Middleware{
		config:   config,
		logger:   logger.With("middleware", "transform"),
		matchers: make(map[string]func(string) bool),
	}

	// Pre-compile path matchers
	for pattern := range config.RequestTransforms {
		m.matchers[pattern] = createMatcher(pattern)
	}
	for pattern := range config.ResponseTransforms {
		if _, exists := m.matchers[pattern]; !exists {
			m.matchers[pattern] = createMatcher(pattern)
		}
	}

	return m
}

// Middleware returns the core.Middleware function
func (m *Middleware) Middleware() core.Middleware {
	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			if !m.config.Enabled {
				return next(ctx, req)
			}

			// Transform request
			transformedReq, err := m.transformRequest(req)
			if err != nil {
				m.logger.Error("Request transformation failed",
					"path", req.Path(),
					"error", err,
				)
				// Continue with original request
				transformedReq = req
			}

			// Call next handler
			resp, err := next(ctx, transformedReq)
			if err != nil {
				return nil, err
			}

			// Transform response
			transformedResp, err := m.transformResponse(resp, req.Path())
			if err != nil {
				m.logger.Error("Response transformation failed",
					"path", req.Path(),
					"error", err,
				)
				// Return original response
				return resp, nil
			}

			return transformedResp, nil
		}
	}
}

// transformRequest applies request transformations
func (m *Middleware) transformRequest(req core.Request) (core.Request, error) {
	// Find matching transform config
	var transformConfig *TransformConfig

	// Check path-specific transforms
	for pattern, config := range m.config.RequestTransforms {
		if matcher, exists := m.matchers[pattern]; exists && matcher(req.Path()) {
			c := config // Create copy to avoid reference issues
			transformConfig = &c
			break
		}
	}

	// Use global if no specific match
	if transformConfig == nil && m.config.GlobalRequest != nil {
		transformConfig = m.config.GlobalRequest
	}

	if transformConfig == nil {
		return req, nil
	}

	// Check conditions
	if !m.checkConditions(transformConfig.Conditions, req) {
		return req, nil
	}

	// Create transformed request
	transformed := &transformedRequest{
		original: req,
		headers:  copyHeaders(req.Headers()),
	}

	// Transform headers
	if transformConfig.Headers != nil {
		headerTransformer := NewHeaderTransformer(*transformConfig.Headers, m.logger)
		transformed.headers = headerTransformer.TransformHeaders(transformed.headers)
	}

	// Transform body
	if transformConfig.Body != nil && req.Body() != nil {
		contentType := getContentType(req.Headers())
		bodyTransformer := NewJSONTransformer(transformConfig.Body.Operations, m.logger)
		
		transformedBody, err := NewBodyTransformer(req.Body(), bodyTransformer, contentType)
		if err != nil {
			return req, err
		}
		transformed.body = transformedBody
	} else {
		transformed.body = req.Body()
	}

	return transformed, nil
}

// transformResponse applies response transformations
func (m *Middleware) transformResponse(resp core.Response, path string) (core.Response, error) {
	// Find matching transform config
	var transformConfig *TransformConfig

	// Check path-specific transforms
	for pattern, config := range m.config.ResponseTransforms {
		if matcher, exists := m.matchers[pattern]; exists && matcher(path) {
			c := config // Create copy
			transformConfig = &c
			break
		}
	}

	// Use global if no specific match
	if transformConfig == nil && m.config.GlobalResponse != nil {
		transformConfig = m.config.GlobalResponse
	}

	if transformConfig == nil {
		return resp, nil
	}

	// Create transformed response
	transformed := &transformedResponse{
		original:   resp,
		statusCode: resp.StatusCode(),
		headers:    copyHeaders(resp.Headers()),
	}

	// Transform headers
	if transformConfig.Headers != nil {
		headerTransformer := NewHeaderTransformer(*transformConfig.Headers, m.logger)
		transformed.headers = headerTransformer.TransformHeaders(transformed.headers)
	}

	// Transform body
	if transformConfig.Body != nil && resp.Body() != nil {
		contentType := getContentType(resp.Headers())
		bodyTransformer := NewJSONTransformer(transformConfig.Body.Operations, m.logger)
		
		transformedBody, err := NewBodyTransformer(resp.Body(), bodyTransformer, contentType)
		if err != nil {
			return resp, err
		}
		transformed.body = transformedBody
	} else {
		transformed.body = resp.Body()
	}

	return transformed, nil
}

// checkConditions checks if all conditions match
func (m *Middleware) checkConditions(conditions []Condition, req core.Request) bool {
	for _, condition := range conditions {
		if !m.checkCondition(condition, req) {
			return false
		}
	}
	return true
}

// checkCondition checks a single condition
func (m *Middleware) checkCondition(condition Condition, req core.Request) bool {
	// Check method
	if condition.Method != "" && condition.Method != req.Method() {
		return false
	}

	// Check header
	if condition.Header != "" {
		headers := req.Headers()
		values, exists := headers[condition.Header]
		if !exists {
			return false
		}
		if condition.Value != "" {
			found := false
			for _, v := range values {
				if v == condition.Value {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// Check content type
	if condition.ContentType != "" {
		contentType := getContentType(req.Headers())
		if !strings.Contains(contentType, condition.ContentType) {
			return false
		}
	}

	return true
}

// Helper functions

func createMatcher(pattern string) func(string) bool {
	// Simple wildcard matching
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return func(path string) bool {
			return strings.HasPrefix(path, prefix)
		}
	}
	return func(path string) bool {
		return path == pattern
	}
}

func copyHeaders(headers map[string][]string) map[string][]string {
	result := make(map[string][]string)
	for k, v := range headers {
		result[k] = append([]string(nil), v...)
	}
	return result
}

func getContentType(headers map[string][]string) string {
	if values, ok := headers["Content-Type"]; ok && len(values) > 0 {
		return values[0]
	}
	return ""
}

// transformedRequest wraps a request with transformations
type transformedRequest struct {
	original core.Request
	headers  map[string][]string
	body     io.ReadCloser
}

func (r *transformedRequest) ID() string                    { return r.original.ID() }
func (r *transformedRequest) Method() string                { return r.original.Method() }
func (r *transformedRequest) Path() string                  { return r.original.Path() }
func (r *transformedRequest) URL() string                   { return r.original.URL() }
func (r *transformedRequest) RemoteAddr() string            { return r.original.RemoteAddr() }
func (r *transformedRequest) Headers() map[string][]string  { return r.headers }
func (r *transformedRequest) Body() io.ReadCloser           { return r.body }
func (r *transformedRequest) Context() context.Context      { return r.original.Context() }

// transformedResponse wraps a response with transformations
type transformedResponse struct {
	original   core.Response
	statusCode int
	headers    map[string][]string
	body       io.ReadCloser
}

func (r *transformedResponse) StatusCode() int               { return r.statusCode }
func (r *transformedResponse) Headers() map[string][]string  { return r.headers }
func (r *transformedResponse) Body() io.ReadCloser           { return r.body }

// RouteAwareMiddleware creates transformation middleware that uses route metadata
func RouteAwareMiddleware(config *Config, logger *slog.Logger) core.Middleware {
	middleware := NewMiddleware(config, logger)
	
	// Enhance with route-aware logic
	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			// Check if route has transformation metadata
			if route := getRouteFromContext(ctx); route != nil {
				if transforms, ok := route.Rule.Metadata["transformations"].(map[string]interface{}); ok {
					// Apply route-specific transformations
					// This would override or merge with global config
					middleware.logger.Debug("Route has transformations",
						"route", route.Rule.ID,
						"transforms", transforms,
					)
				}
			}
			
			return middleware.Middleware()(next)(ctx, req)
		}
	}
}

// Helper to get route from context
type routeContextKey struct{}

func getRouteFromContext(ctx context.Context) *core.RouteResult {
	if route, ok := ctx.Value(routeContextKey{}).(*core.RouteResult); ok {
		return route
	}
	return nil
}
