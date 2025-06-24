package versioning

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"gateway/internal/core"
	"gateway/pkg/errors"
)

// Middleware implements API versioning
type Middleware struct {
	manager *Manager
	logger  *slog.Logger
}

// NewMiddleware creates a new versioning middleware
func NewMiddleware(manager *Manager, logger *slog.Logger) *Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return &Middleware{
		manager: manager,
		logger:  logger.With("middleware", "versioning"),
	}
}

// Middleware returns the core.Middleware function
func (m *Middleware) Middleware() core.Middleware {
	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			// Extract version from request
			version := m.manager.ExtractVersion(req)
			
			// Check if version is allowed
			if !m.manager.IsVersionAllowed(version) {
				return nil, errors.NewError(errors.ErrorTypeBadRequest,
					fmt.Sprintf("API version %s is no longer supported", version))
			}
			
			// Log version usage
			m.logger.Debug("API version extracted",
				"version", version,
				"path", req.Path(),
				"method", req.Method(),
			)
			
			// Store version in context
			ctx = context.WithValue(ctx, versionContextKey{}, version)
			
			// Create versioned request
			versionedReq := &versionedRequest{
				original: req,
				version:  version,
				path:     m.manager.TransformPath(req.Path(), version),
			}
			
			// Call next handler
			resp, err := next(ctx, versionedReq)
			if err != nil {
				return nil, err
			}
			
			// Add version headers to response
			versionHeaders := m.manager.GetVersionHeaders(version)
			return &versionedResponse{
				original:       resp,
				versionHeaders: versionHeaders,
			}, nil
		}
	}
}

// RouteAwareMiddleware creates versioning middleware that modifies routing
func RouteAwareMiddleware(manager *Manager, logger *slog.Logger) core.Middleware {
	middleware := NewMiddleware(manager, logger)
	
	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			// Extract version
			version := manager.ExtractVersion(req)
			
			// Check if version is allowed
			if !manager.IsVersionAllowed(version) {
				return nil, errors.NewError(errors.ErrorTypeBadRequest,
					fmt.Sprintf("API version %s is no longer supported", version))
			}
			
			// Store version in context for route-aware handling
			ctx = context.WithValue(ctx, versionContextKey{}, version)
			
			// If route is already determined, modify it
			if route := getRouteFromContext(ctx); route != nil {
				// Transform service name based on version
				originalService := route.Rule.ServiceName
				versionedService := manager.GetServiceForVersion(originalService, version)
				
				if versionedService != originalService {
					// Create modified route with a copy of the rule
					modifiedRule := *route.Rule
					modifiedRule.ServiceName = versionedService
					
					// Add version metadata
					if modifiedRule.Metadata == nil {
						modifiedRule.Metadata = make(map[string]interface{})
					}
					modifiedRule.Metadata["apiVersion"] = version
					modifiedRule.Metadata["originalService"] = originalService
					
					// Add version transformations
					if transforms := manager.GetTransformations(version); transforms != nil {
						modifiedRule.Metadata["transformations"] = transforms
					}
					
					modifiedRoute := &core.RouteResult{
						Instance: route.Instance,
						Rule:     &modifiedRule,
					}
					
					// Update context with modified route
					ctx = context.WithValue(ctx, routeContextKey{}, modifiedRoute)
					
					middleware.logger.Debug("Route modified for version",
						"version", version,
						"originalService", originalService,
						"versionedService", versionedService,
					)
				}
			}
			
			// Use the base middleware for request/response handling
			return middleware.Middleware()(next)(ctx, req)
		}
	}
}

// GetVersion retrieves the API version from context
func GetVersion(ctx context.Context) (string, bool) {
	version, ok := ctx.Value(versionContextKey{}).(string)
	return version, ok
}

// Context keys
type versionContextKey struct{}
type routeContextKey struct{}

// Helper to get route from context
func getRouteFromContext(ctx context.Context) *core.RouteResult {
	if route, ok := ctx.Value(routeContextKey{}).(*core.RouteResult); ok {
		return route
	}
	return nil
}

// versionedRequest wraps a request with version information
type versionedRequest struct {
	original core.Request
	version  string
	path     string
}

func (r *versionedRequest) ID() string                    { return r.original.ID() }
func (r *versionedRequest) Method() string                { return r.original.Method() }
func (r *versionedRequest) Path() string                  { return r.path }
func (r *versionedRequest) URL() string {
	// Reconstruct URL with transformed path
	origURL := r.original.URL()
	if idx := strings.Index(origURL, "?"); idx != -1 {
		return r.path + origURL[idx:]
	}
	return r.path
}
func (r *versionedRequest) RemoteAddr() string            { return r.original.RemoteAddr() }
func (r *versionedRequest) Headers() map[string][]string  { return r.original.Headers() }
func (r *versionedRequest) Body() io.ReadCloser           { return r.original.Body() }
func (r *versionedRequest) Context() context.Context      { return r.original.Context() }

// versionedResponse wraps a response with version headers
type versionedResponse struct {
	original       core.Response
	versionHeaders map[string]string
}

func (r *versionedResponse) StatusCode() int { return r.original.StatusCode() }
func (r *versionedResponse) Headers() map[string][]string {
	headers := r.original.Headers()
	result := make(map[string][]string)
	
	// Copy original headers
	for k, v := range headers {
		result[k] = v
	}
	
	// Add version headers
	for k, v := range r.versionHeaders {
		result[k] = []string{v}
	}
	
	return result
}
func (r *versionedResponse) Body() io.ReadCloser { return r.original.Body() }
