package rbac

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	
	"gateway/internal/core"
	"gateway/pkg/errors"
)

// MiddlewareConfig represents RBAC middleware configuration
type MiddlewareConfig struct {
	Enabled              bool              `yaml:"enabled"`
	SubjectExtractor     SubjectExtractor  `yaml:"-"` // Function to extract subject from context
	SubjectKey           string            `yaml:"subjectKey"` // Context key for subject
	ResourceExtractor    ResourceExtractor `yaml:"-"` // Function to extract resource from request
	ActionExtractor      ActionExtractor   `yaml:"-"` // Function to extract action from request
	EnforcementMode      string            `yaml:"enforcementMode"` // "enforce" or "permissive"
	DefaultAllow         bool              `yaml:"defaultAllow"`    // Default decision when no policy matches
	SkipPaths            []string          `yaml:"skipPaths"`       // Paths to skip authorization
	PolicyRefreshInterval int              `yaml:"policyRefreshInterval"` // Seconds between policy refresh
}

// SubjectExtractor extracts the subject from context
type SubjectExtractor func(ctx context.Context) (string, error)

// ResourceExtractor extracts the resource from request
type ResourceExtractor func(req core.Request) string

// ActionExtractor extracts the action from request
type ActionExtractor func(req core.Request) string

// Middleware implements RBAC authorization middleware
type Middleware struct {
	rbac   *RBAC
	config *MiddlewareConfig
	logger *slog.Logger
}

// NewMiddleware creates a new RBAC middleware
func NewMiddleware(rbac *RBAC, config *MiddlewareConfig, logger *slog.Logger) *Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	
	// Set default extractors if not provided
	if config.SubjectExtractor == nil {
		config.SubjectExtractor = defaultSubjectExtractor(config.SubjectKey)
	}
	if config.ResourceExtractor == nil {
		config.ResourceExtractor = defaultResourceExtractor
	}
	if config.ActionExtractor == nil {
		config.ActionExtractor = defaultActionExtractor
	}
	
	return &Middleware{
		rbac:   rbac,
		config: config,
		logger: logger.With("middleware", "rbac"),
	}
}

// Middleware returns the core.Middleware function
func (m *Middleware) Middleware() core.Middleware {
	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			// Skip if disabled
			if !m.config.Enabled {
				return next(ctx, req)
			}
			
			// Check if path should be skipped
			if m.shouldSkipPath(req.Path()) {
				return next(ctx, req)
			}
			
			// Extract subject
			subject, err := m.config.SubjectExtractor(ctx)
			if err != nil {
				m.logger.Error("Failed to extract subject", "error", err)
				if m.config.EnforcementMode == "permissive" {
					return next(ctx, req)
				}
				return nil, errors.NewError(errors.ErrorTypeUnauthorized, "failed to extract subject")
			}
			
			// Extract resource and action
			resource := m.config.ResourceExtractor(req)
			action := m.config.ActionExtractor(req)
			
			// Check permission
			allowed := m.rbac.HasPermission(ctx, subject, resource, action)
			
			// Log decision
			m.logger.Debug("Authorization decision",
				"subject", subject,
				"resource", resource,
				"action", action,
				"allowed", allowed,
				"path", req.Path(),
			)
			
			if !allowed {
				// Check default policy
				if m.config.DefaultAllow {
					m.logger.Warn("No matching policy, allowing by default",
						"subject", subject,
						"resource", resource,
						"action", action,
					)
					return next(ctx, req)
				}
				
				if m.config.EnforcementMode == "permissive" {
					m.logger.Warn("Authorization denied in permissive mode",
						"subject", subject,
						"resource", resource,
						"action", action,
					)
					return next(ctx, req)
				}
				
				return nil, errors.NewError(errors.ErrorTypeForbidden, 
					fmt.Sprintf("permission denied: %s cannot %s %s", subject, action, resource))
			}
			
			return next(ctx, req)
		}
	}
}

// shouldSkipPath checks if a path should skip authorization
func (m *Middleware) shouldSkipPath(path string) bool {
	for _, skipPath := range m.config.SkipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}
	return false
}

// Default extractors

func defaultSubjectExtractor(subjectKey string) SubjectExtractor {
	if subjectKey == "" {
		subjectKey = "auth_subject"
	}
	
	return func(ctx context.Context) (string, error) {
		subject, ok := ctx.Value(subjectKey).(string)
		if !ok || subject == "" {
			return "", fmt.Errorf("subject not found in context")
		}
		return subject, nil
	}
}

func defaultResourceExtractor(req core.Request) string {
	// Use the request path as resource
	return req.Path()
}

func defaultActionExtractor(req core.Request) string {
	// Map HTTP methods to actions
	method := req.Method()
	switch method {
	case "GET", "HEAD":
		return "read"
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	default:
		return strings.ToLower(method)
	}
}

// RouteAwareMiddleware creates RBAC middleware that uses route metadata
func RouteAwareMiddleware(rbac *RBAC, config *MiddlewareConfig, logger *slog.Logger) core.Middleware {
	middleware := NewMiddleware(rbac, config, logger)
	
	// Override resource extractor to use route info
	config.ResourceExtractor = func(req core.Request) string {
		// Try to get route from context
		if route := getRouteFromContext(req.Context()); route != nil {
			// Use service name as resource if available
			if route.Rule.ServiceName != "" {
				return "service:" + route.Rule.ServiceName
			}
			// Fall back to route ID
			if route.Rule.ID != "" {
				return "route:" + route.Rule.ID
			}
		}
		// Fall back to path
		return req.Path()
	}
	
	return middleware.Middleware()
}

// Helper to get route from context (must match the key used in route-aware handler)
type routeContextKey struct{}

func getRouteFromContext(ctx context.Context) *core.RouteResult {
	if route, ok := ctx.Value(routeContextKey{}).(*core.RouteResult); ok {
		return route
	}
	return nil
}
