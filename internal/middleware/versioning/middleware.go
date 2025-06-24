package versioning

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"gateway/internal/config"
	"gateway/internal/core"
)

// VersioningMiddleware handles API versioning
type VersioningMiddleware struct {
	config *config.VersioningConfig
	logger *slog.Logger
}

// NewVersioningMiddleware creates a new versioning middleware
func NewVersioningMiddleware(cfg *config.VersioningConfig, logger *slog.Logger) *VersioningMiddleware {
	if cfg == nil {
		cfg = &config.VersioningConfig{
			Enabled:        false,
			Strategy:       "path",
			DefaultVersion: "1.0",
		}
	}

	return &VersioningMiddleware{
		config: cfg,
		logger: logger.With("component", "versioning"),
	}
}

// Middleware returns the HTTP handler middleware
func (m *VersioningMiddleware) Middleware(next http.Handler) http.Handler {
	if !m.config.Enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract version based on strategy
		version := m.extractVersion(r)
		if version == "" {
			version = m.config.DefaultVersion
		}

		// Check if version is deprecated
		if deprecation, exists := m.config.DeprecatedVersions[version]; exists {
			m.addDeprecationHeaders(w, version, deprecation)
		}

		// Store version in context for router to use
		ctx := r.Context()
		ctx = context.WithValue(ctx, "api.version", version)
		
		// Apply version mapping if exists
		if mapping, exists := m.config.VersionMappings[version]; exists {
			// Store service override in context
			if mapping.Service != "" {
				ctx = context.WithValue(ctx, "version.service", mapping.Service)
			}
			
			// Apply path prefix if configured
			if mapping.PathPrefix != "" && !strings.HasPrefix(r.URL.Path, mapping.PathPrefix) {
				r.URL.Path = mapping.PathPrefix + r.URL.Path
			}
		}

		// Add version header to response
		w.Header().Set("X-API-Version", version)

		// Continue with modified request
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractVersion extracts the API version from the request based on strategy
func (m *VersioningMiddleware) extractVersion(r *http.Request) string {
	switch m.config.Strategy {
	case "path":
		return m.extractVersionFromPath(r.URL.Path)
	case "header":
		return r.Header.Get(m.config.VersionHeader)
	case "query":
		return r.URL.Query().Get(m.config.VersionQuery)
	case "accept":
		return m.extractVersionFromAccept(r.Header.Get("Accept"))
	default:
		return ""
	}
}

// extractVersionFromPath extracts version from URL path (e.g., /v2/users)
func (m *VersioningMiddleware) extractVersionFromPath(path string) string {
	// Match patterns like /v1/, /v2.0/, etc.
	re := regexp.MustCompile(`^/v(\d+(?:\.\d+)?)/`)
	matches := re.FindStringSubmatch(path)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractVersionFromAccept extracts version from Accept header
func (m *VersioningMiddleware) extractVersionFromAccept(accept string) string {
	if m.config.AcceptPattern == "" {
		// Default pattern for version in Accept header
		m.config.AcceptPattern = `version=(\d+(?:\.\d+)?)`
	}
	
	re := regexp.MustCompile(m.config.AcceptPattern)
	matches := re.FindStringSubmatch(accept)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// addDeprecationHeaders adds deprecation headers to the response
func (m *VersioningMiddleware) addDeprecationHeaders(w http.ResponseWriter, version string, deprecation *config.DeprecationInfo) {
	w.Header().Set("X-API-Deprecated", "true")
	
	if deprecation.Message != "" {
		w.Header().Set("X-API-Deprecation-Message", deprecation.Message)
	}
	
	if deprecation.SunsetDate != "" {
		// Parse and format sunset date
		if sunsetTime, err := time.Parse(time.RFC3339, deprecation.SunsetDate); err == nil {
			w.Header().Set("Sunset", sunsetTime.Format(time.RFC1123))
		}
	}
}

// GetVersionFromContext extracts the API version from the request context
func GetVersionFromContext(ctx context.Context) string {
	if version, ok := ctx.Value("api.version").(string); ok {
		return version
	}
	return ""
}

// GetServiceOverrideFromContext extracts service override from context
func GetServiceOverrideFromContext(ctx context.Context) string {
	if service, ok := ctx.Value("version.service").(string); ok {
		return service
	}
	return ""
}

// VersionRouteModifier implements route modification based on versioning
type VersionRouteModifier struct {
	config *config.VersioningConfig
}

// NewVersionRouteModifier creates a new version route modifier
func NewVersionRouteModifier(cfg *config.VersioningConfig) *VersionRouteModifier {
	return &VersionRouteModifier{
		config: cfg,
	}
}

// ModifyRoute modifies the route based on version information
func (m *VersionRouteModifier) ModifyRoute(ctx context.Context, route *core.RouteResult) (*core.RouteResult, error) {
	version := GetVersionFromContext(ctx)
	if version == "" {
		return route, nil
	}

	// Check for service override
	if serviceOverride := GetServiceOverrideFromContext(ctx); serviceOverride != "" {
		// Create a modified route with the version-specific service
		modifiedRoute := *route
		// The router will need to resolve this service name
		modifiedRoute.ServiceName = serviceOverride
		return &modifiedRoute, nil
	}

	return route, nil
}