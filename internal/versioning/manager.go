package versioning

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gateway/internal/core"
)

// Strategy defines how versions are extracted and handled
type Strategy string

const (
	StrategyPath   Strategy = "path"   // Version in path: /v1/users
	StrategyHeader Strategy = "header" // Version in header: X-API-Version: 1
	StrategyQuery  Strategy = "query"  // Version in query: ?version=1
	StrategyAccept Strategy = "accept" // Version in Accept header: application/vnd.api+json;version=1
)

// Config represents versioning configuration
type Config struct {
	Enabled           bool                    `yaml:"enabled"`
	Strategy          Strategy                `yaml:"strategy"`
	DefaultVersion    string                  `yaml:"defaultVersion"`
	VersionHeader     string                  `yaml:"versionHeader"`     // For header strategy
	VersionQuery      string                  `yaml:"versionQuery"`      // For query strategy
	AcceptPattern     string                  `yaml:"acceptPattern"`     // For accept strategy
	DeprecatedVersions map[string]DeprecationInfo `yaml:"deprecatedVersions"`
	VersionMappings   map[string]VersionMapping   `yaml:"versionMappings"`
}

// DeprecationInfo contains deprecation details
type DeprecationInfo struct {
	Message    string    `yaml:"message"`
	SunsetDate time.Time `yaml:"sunsetDate"`
	RemovalDate time.Time `yaml:"removalDate"`
}

// VersionMapping maps versions to services or transformations
type VersionMapping struct {
	Service         string                 `yaml:"service"`
	PathPrefix      string                 `yaml:"pathPrefix"`
	Transformations map[string]interface{} `yaml:"transformations"`
	Deprecated      bool                   `yaml:"deprecated"`
}

// Manager manages API versioning
type Manager struct {
	config         *Config
	logger         *slog.Logger
	versionPattern *regexp.Regexp
	acceptPattern  *regexp.Regexp
	mu             sync.RWMutex
}

// NewManager creates a new versioning manager
func NewManager(config *Config, logger *slog.Logger) (*Manager, error) {
	if logger == nil {
		logger = slog.Default()
	}

	m := &Manager{
		config: config,
		logger: logger.With("component", "versioning"),
	}

	// Compile patterns
	if config.Strategy == StrategyPath {
		// Match version in path: /v1/, /v2.0/, etc.
		m.versionPattern = regexp.MustCompile(`^/v(\d+(?:\.\d+)?)/`)
	}

	if config.Strategy == StrategyAccept && config.AcceptPattern != "" {
		pattern, err := regexp.Compile(config.AcceptPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid accept pattern: %w", err)
		}
		m.acceptPattern = pattern
	}

	return m, nil
}

// ExtractVersion extracts the API version from a request
func (m *Manager) ExtractVersion(req core.Request) string {
	if !m.config.Enabled {
		return m.config.DefaultVersion
	}

	var version string

	switch m.config.Strategy {
	case StrategyPath:
		version = m.extractPathVersion(req.Path())
	case StrategyHeader:
		version = m.extractHeaderVersion(req.Headers())
	case StrategyQuery:
		version = m.extractQueryVersion(req.URL())
	case StrategyAccept:
		version = m.extractAcceptVersion(req.Headers())
	}

	if version == "" {
		version = m.config.DefaultVersion
	}

	return version
}

// TransformPath transforms the request path based on version
func (m *Manager) TransformPath(path string, version string) string {
	if !m.config.Enabled {
		return path
	}

	// Remove version from path if using path strategy
	if m.config.Strategy == StrategyPath && m.versionPattern != nil {
		path = m.versionPattern.ReplaceAllString(path, "/")
	}

	// Apply version-specific path transformations
	if mapping, ok := m.config.VersionMappings[version]; ok {
		if mapping.PathPrefix != "" {
			path = mapping.PathPrefix + path
		}
	}

	return path
}

// GetServiceForVersion returns the service name for a specific version
func (m *Manager) GetServiceForVersion(serviceName string, version string) string {
	if !m.config.Enabled {
		return serviceName
	}

	if mapping, ok := m.config.VersionMappings[version]; ok {
		if mapping.Service != "" {
			return mapping.Service
		}
	}

	// Default pattern: append version to service name
	if version != "" && version != m.config.DefaultVersion {
		return serviceName + "-" + version
	}

	return serviceName
}

// CheckDeprecation checks if a version is deprecated
func (m *Manager) CheckDeprecation(version string) *DeprecationInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if info, ok := m.config.DeprecatedVersions[version]; ok {
		return &info
	}

	if mapping, ok := m.config.VersionMappings[version]; ok {
		if mapping.Deprecated {
			return &DeprecationInfo{
				Message: fmt.Sprintf("API version %s is deprecated", version),
			}
		}
	}

	return nil
}

// IsVersionAllowed checks if a version is allowed
func (m *Manager) IsVersionAllowed(version string) bool {
	if !m.config.Enabled {
		return true
	}

	// Check removal date
	if info, ok := m.config.DeprecatedVersions[version]; ok {
		if !info.RemovalDate.IsZero() && time.Now().After(info.RemovalDate) {
			return false
		}
	}

	return true
}

// GetVersionHeaders returns headers to add for version information
func (m *Manager) GetVersionHeaders(version string) map[string]string {
	headers := make(map[string]string)

	// Add version header
	headers["X-API-Version"] = version

	// Add deprecation headers if applicable
	if deprecation := m.CheckDeprecation(version); deprecation != nil {
		headers["X-API-Deprecated"] = "true"
		if deprecation.Message != "" {
			headers["X-API-Deprecation-Message"] = deprecation.Message
		}
		if !deprecation.SunsetDate.IsZero() {
			headers["Sunset"] = deprecation.SunsetDate.Format(time.RFC1123)
		}
	}

	return headers
}

// GetTransformations returns transformations for a specific version
func (m *Manager) GetTransformations(version string) map[string]interface{} {
	if mapping, ok := m.config.VersionMappings[version]; ok {
		return mapping.Transformations
	}
	return nil
}

// Helper methods

func (m *Manager) extractPathVersion(path string) string {
	if m.versionPattern == nil {
		return ""
	}

	matches := m.versionPattern.FindStringSubmatch(path)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func (m *Manager) extractHeaderVersion(headers map[string][]string) string {
	headerName := m.config.VersionHeader
	if headerName == "" {
		headerName = "X-API-Version"
	}

	if values, ok := headers[headerName]; ok && len(values) > 0 {
		return values[0]
	}

	return ""
}

func (m *Manager) extractQueryVersion(url string) string {
	queryParam := m.config.VersionQuery
	if queryParam == "" {
		queryParam = "version"
	}

	// Simple query parameter extraction
	if idx := strings.Index(url, "?"); idx != -1 {
		query := url[idx+1:]
		params := strings.Split(query, "&")
		for _, param := range params {
			parts := strings.SplitN(param, "=", 2)
			if len(parts) == 2 && parts[0] == queryParam {
				return parts[1]
			}
		}
	}

	return ""
}

func (m *Manager) extractAcceptVersion(headers map[string][]string) string {
	if values, ok := headers["Accept"]; ok && len(values) > 0 {
		accept := values[0]
		
		// Use custom pattern if configured
		if m.acceptPattern != nil {
			matches := m.acceptPattern.FindStringSubmatch(accept)
			if len(matches) > 1 {
				return matches[1]
			}
		}
		
		// Default pattern: version=X
		if idx := strings.Index(accept, "version="); idx != -1 {
			start := idx + 8
			end := start
			for end < len(accept) && accept[end] != ';' && accept[end] != ',' {
				end++
			}
			return accept[start:end]
		}
	}

	return ""
}

// CompareVersions compares two version strings
func CompareVersions(v1, v2 string) int {
	// Parse version numbers
	parts1 := parseVersion(v1)
	parts2 := parseVersion(v2)

	// Compare each part
	for i := 0; i < len(parts1) || i < len(parts2); i++ {
		var p1, p2 int
		if i < len(parts1) {
			p1 = parts1[i]
		}
		if i < len(parts2) {
			p2 = parts2[i]
		}

		if p1 < p2 {
			return -1
		} else if p1 > p2 {
			return 1
		}
	}

	return 0
}

func parseVersion(v string) []int {
	// Remove 'v' prefix if present
	v = strings.TrimPrefix(v, "v")
	
	parts := strings.Split(v, ".")
	result := make([]int, 0, len(parts))
	
	for _, part := range parts {
		if num, err := strconv.Atoi(part); err == nil {
			result = append(result, num)
		}
	}
	
	return result
}
