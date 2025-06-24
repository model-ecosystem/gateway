package config

import (
	"time"

	"gateway/internal/core"
)

// Config holds gateway configuration
type Config struct {
	Gateway Gateway `yaml:"gateway"`
}

// Gateway configuration
type Gateway struct {
	Frontend         Frontend          `yaml:"frontend"`
	Backend          Backend           `yaml:"backend"`
	Registry         Registry          `yaml:"registry"`
	Router           Router            `yaml:"router"`
	Auth             *Auth             `yaml:"auth,omitempty"`
	Health           *Health           `yaml:"health,omitempty"`
	Metrics          *Metrics          `yaml:"metrics,omitempty"`
	CircuitBreaker   *CircuitBreaker   `yaml:"circuitBreaker,omitempty"`
	Retry            *Retry            `yaml:"retry,omitempty"`
	CORS             *CORS             `yaml:"cors,omitempty"`
	Redis            *Redis            `yaml:"redis,omitempty"`
	RateLimitStorage *RateLimitStorage `yaml:"rateLimitStorage,omitempty"`
	Telemetry        *Telemetry        `yaml:"telemetry,omitempty"`
	Management       *Management       `yaml:"management,omitempty"`
	Middleware       *Middleware       `yaml:"middleware,omitempty"`
	OpenAPI          *OpenAPIConfig    `yaml:"openapi,omitempty"`
	Versioning       *VersioningConfig `yaml:"versioning,omitempty"`
}

// Frontend configuration
type Frontend struct {
	HTTP      HTTP       `yaml:"http"`
	WebSocket *WebSocket `yaml:"websocket,omitempty"`
	SSE       *SSE       `yaml:"sse,omitempty"`
}

// HTTP configuration
type HTTP struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	ReadTimeout    int    `yaml:"readTimeout"`
	WriteTimeout   int    `yaml:"writeTimeout"`
	MaxRequestSize int64  `yaml:"maxRequestSize"` // Maximum request body size in bytes (0 = no limit)
	TLS            *TLS   `yaml:"tls,omitempty"`
}

// TLS configuration
type TLS struct {
	Enabled            bool   `yaml:"enabled"`
	CertFile           string `yaml:"certFile"`
	KeyFile            string `yaml:"keyFile"`
	MinVersion         string `yaml:"minVersion"`
	MaxVersion         string `yaml:"maxVersion"`
	CipherSuites       []int  `yaml:"cipherSuites"`
	PreferServerCipher bool   `yaml:"preferServerCipher"`
}

// Backend configuration
type Backend struct {
	HTTP      HTTPBackend       `yaml:"http"`
	WebSocket *WebSocketBackend `yaml:"websocket,omitempty"`
	SSE       *SSEBackend       `yaml:"sse,omitempty"`
}

// HTTPBackend configuration
type HTTPBackend struct {
	// Connection pool settings
	MaxIdleConns        int `yaml:"maxIdleConns"`
	MaxIdleConnsPerHost int `yaml:"maxIdleConnsPerHost"`
	MaxConnsPerHost     int `yaml:"maxConnsPerHost"`
	IdleConnTimeout     int `yaml:"idleConnTimeout"`

	// Connection settings
	KeepAlive          bool `yaml:"keepAlive"`
	KeepAliveTimeout   int  `yaml:"keepAliveTimeout"`
	DisableCompression bool `yaml:"disableCompression"`
	DisableHTTP2       bool `yaml:"disableHTTP2"`

	// Timeout settings
	DialTimeout           int `yaml:"dialTimeout"`
	ResponseHeaderTimeout int `yaml:"responseHeaderTimeout"`
	ExpectContinueTimeout int `yaml:"expectContinueTimeout"`
	TLSHandshakeTimeout   int `yaml:"tlsHandshakeTimeout"`

	// TLS settings
	TLS *BackendTLS `yaml:"tls,omitempty"`
}

// BackendTLS configuration
type BackendTLS struct {
	Enabled            bool   `yaml:"enabled"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
	ServerName         string `yaml:"serverName"`
	ClientCertFile     string `yaml:"clientCertFile"`
	ClientKeyFile      string `yaml:"clientKeyFile"`
	RootCAFile         string `yaml:"rootCAFile"`
	MinVersion         string `yaml:"minVersion"`
	MaxVersion         string `yaml:"maxVersion"`
	PreferServerCipher bool   `yaml:"preferServerCipher"`
	Renegotiation      bool   `yaml:"renegotiation"`
}

// WebSocket adapter configuration
type WebSocket struct {
	Enabled           bool     `yaml:"enabled"`
	Host              string   `yaml:"host"`
	Port              int      `yaml:"port"`
	ReadTimeout       int      `yaml:"readTimeout"`
	WriteTimeout      int      `yaml:"writeTimeout"`
	HandshakeTimeout  int      `yaml:"handshakeTimeout"`
	MaxMessageSize    int64    `yaml:"maxMessageSize"`
	ReadBufferSize    int      `yaml:"readBufferSize"`
	WriteBufferSize   int      `yaml:"writeBufferSize"`
	CheckOrigin       bool     `yaml:"checkOrigin"`
	AllowedOrigins    []string `yaml:"allowedOrigins"`
	EnableCompression bool     `yaml:"enableCompression"`
	CompressionLevel  int      `yaml:"compressionLevel"`
	Subprotocols      []string `yaml:"subprotocols"`
	WriteDeadline     int      `yaml:"writeDeadline"`
	PongWait          int      `yaml:"pongWait"`
	PingPeriod        int      `yaml:"pingPeriod"`
	CloseGracePeriod  int      `yaml:"closeGracePeriod"`
	// Token validation for long-lived connections
	TokenValidation    bool `yaml:"tokenValidation"`    // Enable token validation
	TokenCheckInterval int  `yaml:"tokenCheckInterval"` // Check interval in seconds (default: 60)
}

// WebSocketBackend configuration
type WebSocketBackend struct {
	// Connection settings
	HandshakeTimeout int `yaml:"handshakeTimeout"`
	ReadTimeout      int `yaml:"readTimeout"`
	WriteTimeout     int `yaml:"writeTimeout"`

	// Buffer settings
	ReadBufferSize  int `yaml:"readBufferSize"`
	WriteBufferSize int `yaml:"writeBufferSize"`

	// Message settings
	MaxMessageSize int64 `yaml:"maxMessageSize"`

	// Connection pool settings
	MaxConnections        int `yaml:"maxConnections"`
	ConnectionTimeout     int `yaml:"connectionTimeout"`
	IdleConnectionTimeout int `yaml:"idleConnectionTimeout"`

	// Keepalive settings
	PingInterval int `yaml:"pingInterval"`
	PongTimeout  int `yaml:"pongTimeout"`
	CloseTimeout int `yaml:"closeTimeout"`

	// Compression
	EnableCompression bool `yaml:"enableCompression"`
	CompressionLevel  int  `yaml:"compressionLevel"`
}

// SSE adapter configuration
type SSE struct {
	Enabled          bool `yaml:"enabled"`
	WriteTimeout     int  `yaml:"writeTimeout"`
	KeepaliveTimeout int  `yaml:"keepaliveTimeout"`
	// Token validation for long-lived connections
	TokenValidation    bool `yaml:"tokenValidation"`    // Enable token validation
	TokenCheckInterval int  `yaml:"tokenCheckInterval"` // Check interval in seconds (default: 60)
}

// SSEBackend configuration
type SSEBackend struct {
	// Connection settings
	ConnectTimeout int `yaml:"connectTimeout"`
	ReadTimeout    int `yaml:"readTimeout"`

	// Buffering settings
	BufferSize int `yaml:"bufferSize"`

	// Retry settings
	RetryInterval int `yaml:"retryInterval"`
	MaxRetries    int `yaml:"maxRetries"`

	// Event settings
	MaxEventSize int `yaml:"maxEventSize"`
}

// Registry configuration
type Registry struct {
	Type          string                   `yaml:"type"`
	Static        *StaticRegistry          `yaml:"static,omitempty"`
	Docker        *DockerRegistry          `yaml:"docker,omitempty"`
	DockerCompose *DockerComposeRegistry   `yaml:"dockerCompose,omitempty"`
}

// StaticRegistry configuration
type StaticRegistry struct {
	Services []Service `yaml:"services"`
}

// Service represents a service definition
type Service struct {
	Name      string     `yaml:"name"`
	Instances []Instance `yaml:"instances"`
}

// Instance represents a service instance
type Instance struct {
	ID      string   `yaml:"id"`
	Address string   `yaml:"address"`
	Port    int      `yaml:"port"`
	Weight  int      `yaml:"weight"`
	Health  string   `yaml:"health"`
	Tags    []string `yaml:"tags"`
}

// DockerRegistry configuration
type DockerRegistry struct {
	// Docker connection settings
	Host     string `yaml:"host"`     // Docker daemon host
	Version  string `yaml:"version"`  // Docker API version
	CertPath string `yaml:"certPath"` // Path to certificates for TLS

	// Service discovery settings
	LabelPrefix     string `yaml:"labelPrefix"`     // Label prefix for gateway config
	Network         string `yaml:"network"`         // Docker network to use
	RefreshInterval int    `yaml:"refreshInterval"` // Service refresh interval in seconds
}

// DockerComposeRegistry configuration
type DockerComposeRegistry struct {
	// Project name to filter containers
	ProjectName string `yaml:"projectName"`
	// Label prefix for gateway configuration
	LabelPrefix string `yaml:"labelPrefix"`
	// Service refresh interval in seconds
	RefreshInterval int `yaml:"refreshInterval"`
	// Docker connection settings
	DockerHost string `yaml:"dockerHost"`
	APIVersion string `yaml:"apiVersion"`
}

// Router configuration
type Router struct {
	Rules []RouteRule `yaml:"rules"`
}

// RouteRule represents a single routing rule
type RouteRule struct {
	ID                    string                 `yaml:"id"`
	Path                  string                 `yaml:"path"`
	ServiceName           string                 `yaml:"serviceName"`
	LoadBalance           string                 `yaml:"loadBalance"`
	Timeout               int                    `yaml:"timeout"`
	Protocol              string                 `yaml:"protocol"` // http, grpc, websocket, sse
	SessionAffinityConfig *SessionAffinityConfig `yaml:"sessionAffinity"`
	// Authentication
	AuthRequired bool   `yaml:"authRequired"`
	AuthType     string `yaml:"authType"`
	// Rate limiting
	RateLimit           int    `yaml:"rateLimit"`
	RateLimitBurst      int    `yaml:"rateLimitBurst"`
	RateLimitExpiration int    `yaml:"rateLimitExpiration"`
	RateLimitStorage    string `yaml:"rateLimitStorage"` // Storage name to use
	// gRPC configuration
	GRPC *GRPCConfig `yaml:"grpc,omitempty"`
}

// SessionAffinityConfig represents session affinity configuration
type SessionAffinityConfig struct {
	Enabled    bool   `yaml:"enabled"`
	TTL        int    `yaml:"ttl"`
	Source     string `yaml:"source"`     // cookie, header, query
	CookieName string `yaml:"cookieName"` // for cookie source
	HeaderName string `yaml:"headerName"` // for header source
	QueryParam string `yaml:"queryParam"` // for query source
	MaxEntries int    `yaml:"maxEntries"` // max number of sessions to track
}

// Auth configuration
type Auth struct {
	Required       bool          `yaml:"required"`
	Providers      []string      `yaml:"providers"`
	SkipPaths      []string      `yaml:"skipPaths"`
	RequiredScopes []string      `yaml:"requiredScopes"`
	JWT            *JWTConfig    `yaml:"jwt,omitempty"`
	APIKey         *APIKeyConfig `yaml:"apikey,omitempty"`
}

// JWTConfig represents JWT authentication configuration
type JWTConfig struct {
	Enabled           bool              `yaml:"enabled"`
	Issuer            string            `yaml:"issuer"`
	Audience          []string          `yaml:"audience"`
	SigningMethod     string            `yaml:"signingMethod"`
	PublicKey         string            `yaml:"publicKey"`
	Secret            string            `yaml:"secret"`
	JWKSEndpoint      string            `yaml:"jwksEndpoint"`
	JWKSCacheDuration int               `yaml:"jwksCacheDuration"` // seconds
	ClaimsMapping     map[string]string `yaml:"claimsMapping"`
	ScopeClaim        string            `yaml:"scopeClaim"`
	SubjectClaim      string            `yaml:"subjectClaim"`
	HeaderName        string            `yaml:"headerName"`
	CookieName        string            `yaml:"cookieName"`
}

// APIKeyConfig represents API key authentication configuration
type APIKeyConfig struct {
	Enabled       bool                      `yaml:"enabled"`
	Keys          map[string]*APIKeyDetails `yaml:"keys"`
	HashKeys      bool                      `yaml:"hashKeys"`
	DefaultScopes []string                  `yaml:"defaultScopes"`
	HeaderName    string                    `yaml:"headerName"`
	QueryParam    string                    `yaml:"queryParam"`
	Scheme        string                    `yaml:"scheme"`
}

// APIKeyDetails represents configuration for a single API key
type APIKeyDetails struct {
	Key       string                 `yaml:"key"`
	Subject   string                 `yaml:"subject"`
	Type      string                 `yaml:"type"`
	Scopes    []string               `yaml:"scopes"`
	ExpiresAt string                 `yaml:"expiresAt"`
	Metadata  map[string]interface{} `yaml:"metadata"`
	Disabled  bool                   `yaml:"disabled"`
}

// ToServiceInstance converts to core.ServiceInstance
func (i *Instance) ToServiceInstance(name string) core.ServiceInstance {
	return core.ServiceInstance{
		ID:       i.ID,
		Name:     name,
		Address:  i.Address,
		Port:     i.Port,
		Healthy:  i.Health == "healthy",
		Metadata: nil, // Static registry doesn't have metadata yet
	}
}

// ToRouteRule converts to core.RouteRule
func (r *RouteRule) ToRouteRule() core.RouteRule {
	rule := core.RouteRule{
		ID:          r.ID,
		Path:        r.Path,
		Methods:     nil, // Config doesn't have methods yet
		ServiceName: r.ServiceName,
		LoadBalance: core.LoadBalanceStrategy(r.LoadBalance),
		Timeout:     time.Duration(r.Timeout) * time.Second,
		Protocol:    r.Protocol,
		Metadata:    make(map[string]interface{}),
	}

	// Default to HTTP if protocol not specified
	if rule.Protocol == "" {
		rule.Protocol = "http"
	}

	// Convert session affinity config
	if r.SessionAffinityConfig != nil && r.SessionAffinityConfig.Enabled {
		rule.SessionAffinity = &core.SessionAffinityConfig{
			Enabled:    r.SessionAffinityConfig.Enabled,
			TTL:        time.Duration(r.SessionAffinityConfig.TTL) * time.Second,
			Source:     core.SessionSource(r.SessionAffinityConfig.Source),
			CookieName: r.SessionAffinityConfig.CookieName,
			HeaderName: r.SessionAffinityConfig.HeaderName,
			QueryParam: r.SessionAffinityConfig.QueryParam,
			MaxEntries: r.SessionAffinityConfig.MaxEntries,
		}
	}

	// Add gRPC configuration if present
	if r.GRPC != nil {
		// Override protocol if GRPC config is present (backward compatibility)
		rule.Protocol = "grpc"
		rule.Metadata["grpc"] = r.GRPC
	}

	// Add authentication configuration
	rule.Metadata["authRequired"] = r.AuthRequired
	if r.AuthType != "" {
		rule.Metadata["authType"] = r.AuthType
	}

	// Add rate limiting configuration
	if r.RateLimit > 0 {
		rule.Metadata["rateLimit"] = r.RateLimit
		rule.Metadata["rateLimitBurst"] = r.RateLimitBurst
		rule.Metadata["rateLimitExpiration"] = r.RateLimitExpiration
		if r.RateLimitStorage != "" {
			rule.Metadata["rateLimitStorage"] = r.RateLimitStorage
		}
	}

	return rule
}

// Health configuration
type Health struct {
	Enabled    bool             `yaml:"enabled"`
	HealthPath string           `yaml:"healthPath"`
	ReadyPath  string           `yaml:"readyPath"`
	LivePath   string           `yaml:"livePath"`
	Checks     map[string]Check `yaml:"checks"`
}

// Check represents a health check configuration
type Check struct {
	Type     string            `yaml:"type"`     // http, tcp, exec, grpc
	Interval int               `yaml:"interval"` // Check interval in seconds
	Timeout  int               `yaml:"timeout"`  // Timeout in seconds
	Config   map[string]string `yaml:"config"`   // Check-specific configuration
}

// Metrics configuration
type Metrics struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"` // Path to expose metrics (e.g., /metrics)
	Port    int    `yaml:"port"` // Port to expose metrics (0 = same as main port)
}

// CircuitBreaker configuration
type CircuitBreaker struct {
	Enabled  bool                            `yaml:"enabled"`
	Default  CircuitBreakerConfig            `yaml:"default"`
	Routes   map[string]CircuitBreakerConfig `yaml:"routes,omitempty"`
	Services map[string]CircuitBreakerConfig `yaml:"services,omitempty"`
}

// CircuitBreakerConfig holds circuit breaker settings
type CircuitBreakerConfig struct {
	MaxFailures      int     `yaml:"maxFailures"`      // Max failures before opening
	FailureThreshold float64 `yaml:"failureThreshold"` // Failure percentage threshold (0-1)
	Timeout          int     `yaml:"timeout"`          // Open state timeout in seconds
	MaxRequests      int     `yaml:"maxRequests"`      // Max requests in half-open state
	Interval         int     `yaml:"interval"`         // Reset interval in seconds
}

// Retry configuration
type Retry struct {
	Enabled  bool                   `yaml:"enabled"`
	Default  RetryConfig            `yaml:"default"`
	Routes   map[string]RetryConfig `yaml:"routes,omitempty"`
	Services map[string]RetryConfig `yaml:"services,omitempty"`
}

// RetryConfig holds retry settings
type RetryConfig struct {
	MaxAttempts  int     `yaml:"maxAttempts"`  // Maximum retry attempts (0 = no retry)
	InitialDelay int     `yaml:"initialDelay"` // Initial delay in milliseconds
	MaxDelay     int     `yaml:"maxDelay"`     // Maximum delay in milliseconds
	Multiplier   float64 `yaml:"multiplier"`   // Backoff multiplier
	Jitter       bool    `yaml:"jitter"`       // Add jitter to delays
	BudgetRatio  float64 `yaml:"budgetRatio"`  // Retry budget ratio (0-1, default 0.1)
}

// CORS configuration
type CORS struct {
	Enabled              bool     `yaml:"enabled"`
	AllowedOrigins       []string `yaml:"allowedOrigins"`
	AllowedMethods       []string `yaml:"allowedMethods"`
	AllowedHeaders       []string `yaml:"allowedHeaders"`
	ExposedHeaders       []string `yaml:"exposedHeaders"`
	AllowCredentials     bool     `yaml:"allowCredentials"`
	MaxAge               int      `yaml:"maxAge"`
	OptionsPassthrough   bool     `yaml:"optionsPassthrough"`
	OptionsSuccessStatus int      `yaml:"optionsSuccessStatus"`
}

// GRPCConfig holds gRPC-specific configuration for a route
type GRPCConfig struct {
	// ProtoDescriptor is the path to the proto descriptor file
	ProtoDescriptor string `yaml:"protoDescriptor"`
	// ProtoDescriptorBase64 is the base64 encoded proto descriptor
	ProtoDescriptorBase64 string `yaml:"protoDescriptorBase64"`
	// Service is the fully qualified gRPC service name
	Service string `yaml:"service"`
	// EnableTranscoding enables HTTP to gRPC transcoding
	EnableTranscoding bool `yaml:"enableTranscoding"`
	// TranscodingRules defines custom transcoding rules
	TranscodingRules map[string]string `yaml:"transcodingRules"`
	// DynamicDescriptors configuration for loading .desc files
	DynamicDescriptors *GRPCDescriptorConfig `yaml:"dynamicDescriptors,omitempty"`
}

// GRPCDescriptorConfig holds configuration for dynamic descriptor loading
type GRPCDescriptorConfig struct {
	// DescriptorFiles is a list of .desc files to load
	DescriptorFiles []string `yaml:"files"`
	// DescriptorDirs is a list of directories to scan for .desc files
	DescriptorDirs []string `yaml:"directories"`
	// AutoReload enables automatic reloading of descriptors
	AutoReload bool `yaml:"autoReload"`
	// ReloadInterval is the interval for checking descriptor changes (in seconds)
	ReloadInterval int `yaml:"reloadInterval"`
}

// Redis configuration
type Redis struct {
	// Connection settings
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`

	// Connection pool settings
	MaxActive      int `yaml:"maxActive"`      // Maximum number of active connections
	MaxIdle        int `yaml:"maxIdle"`        // Maximum number of idle connections
	IdleTimeout    int `yaml:"idleTimeout"`    // Idle timeout in seconds
	ConnectTimeout int `yaml:"connectTimeout"` // Connection timeout in seconds
	ReadTimeout    int `yaml:"readTimeout"`    // Read timeout in seconds
	WriteTimeout   int `yaml:"writeTimeout"`   // Write timeout in seconds

	// TLS settings
	TLS *RedisTLS `yaml:"tls,omitempty"`

	// Cluster settings
	Cluster      bool     `yaml:"cluster"`
	ClusterNodes []string `yaml:"clusterNodes"`

	// Sentinel settings
	Sentinel      bool     `yaml:"sentinel"`
	MasterName    string   `yaml:"masterName"`
	SentinelNodes []string `yaml:"sentinelNodes"`
}

// RedisTLS configuration
type RedisTLS struct {
	Enabled            bool   `yaml:"enabled"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
	CertFile           string `yaml:"certFile"`
	KeyFile            string `yaml:"keyFile"`
	CAFile             string `yaml:"caFile"`
}

// RateLimitStorage defines available rate limit storage backends
type RateLimitStorage struct {
	// Default storage to use if not specified in route
	Default string `yaml:"default"`
	// Available storage configurations
	Stores map[string]*RateLimitStore `yaml:"stores"`
}

// RateLimitStore defines a single rate limit storage configuration
type RateLimitStore struct {
	Type  string `yaml:"type"` // "memory" or "redis"
	Redis *Redis `yaml:"redis,omitempty"`
	// Memory storage doesn't need configuration
}

// Telemetry configuration
type Telemetry struct {
	Enabled bool            `yaml:"enabled"`
	Service string          `yaml:"service"` // Service name for telemetry
	Version string          `yaml:"version"` // Service version
	Tracing TracingConfig   `yaml:"tracing"`
	Metrics TelemetryMetrics `yaml:"metrics"`
}

// TracingConfig holds tracing configuration
type TracingConfig struct {
	Enabled      bool              `yaml:"enabled"`
	Endpoint     string            `yaml:"endpoint"`     // OTLP endpoint (e.g., localhost:4318)
	Headers      map[string]string `yaml:"headers"`      // Additional headers for OTLP
	SampleRate   float64           `yaml:"sampleRate"`   // Sampling rate (0-1)
	MaxBatchSize int               `yaml:"maxBatchSize"` // Max batch size for export
	BatchTimeout int               `yaml:"batchTimeout"` // Batch timeout in seconds
}

// TelemetryMetrics configuration (for OpenTelemetry metrics)
type TelemetryMetrics struct {
	Enabled bool `yaml:"enabled"`
	// Additional OTEL metrics configuration can be added here
}

// Management configuration for runtime management API
type Management struct {
	Enabled  bool              `yaml:"enabled"`
	Host     string            `yaml:"host"`
	Port     int               `yaml:"port"`
	BasePath string            `yaml:"basePath"`
	Auth     *ManagementAuth   `yaml:"auth,omitempty"`
}

// ManagementAuth configuration
type ManagementAuth struct {
	Type  string            `yaml:"type"`  // "basic", "token"
	Token string            `yaml:"token"` // For token auth
	Users map[string]string `yaml:"users"` // For basic auth: username -> password
}

// Middleware configuration
type Middleware struct {
	Auth      *MiddlewareAuth      `yaml:"auth,omitempty"`
	Authz     *MiddlewareAuthz     `yaml:"authz,omitempty"`
	Transform *TransformConfig     `yaml:"transform,omitempty"`
}

// MiddlewareAuth configuration
type MiddlewareAuth struct {
	OAuth2 *OAuth2Config `yaml:"oauth2,omitempty"`
}

// MiddlewareAuthz configuration  
type MiddlewareAuthz struct {
	RBAC *RBACConfig `yaml:"rbac,omitempty"`
}

// OAuth2Config represents OAuth2/OIDC configuration
type OAuth2Config struct {
	Enabled         bool               `yaml:"enabled"`
	TokenHeader     string             `yaml:"tokenHeader"`
	TokenQuery      string             `yaml:"tokenQuery"`
	TokenCookie     string             `yaml:"tokenCookie"`
	BearerPrefix    string             `yaml:"bearerPrefix"`
	RequireScopes   []string           `yaml:"requireScopes"`
	RequireAudience []string           `yaml:"requireAudience"`
	ClaimsKey       string             `yaml:"claimsKey"`
	Providers       []OAuth2Provider   `yaml:"providers"`
}

// OAuth2Provider represents an OAuth2/OIDC provider configuration
type OAuth2Provider struct {
	Name             string            `yaml:"name"`
	ClientID         string            `yaml:"clientId"`
	ClientSecret     string            `yaml:"clientSecret"`
	AuthorizationURL string            `yaml:"authorizationUrl"`
	TokenURL         string            `yaml:"tokenUrl"`
	UserInfoURL      string            `yaml:"userInfoUrl"`
	JWKSEndpoint     string            `yaml:"jwksEndpoint"`
	IssuerURL        string            `yaml:"issuerUrl"`
	DiscoveryURL     string            `yaml:"discoveryUrl"`
	UseDiscovery     bool              `yaml:"useDiscovery"`
	ValidateIssuer   bool              `yaml:"validateIssuer"`
	ValidateAudience bool              `yaml:"validateAudience"`
	Audience         []string          `yaml:"audience"`
	Scopes           []string          `yaml:"scopes"`
	ClaimsMapping    map[string]string `yaml:"claimsMapping"`
}

// RBACConfig represents RBAC configuration
type RBACConfig struct {
	Enabled              bool          `yaml:"enabled"`
	EnforcementMode      string        `yaml:"enforcementMode"`
	DefaultAllow         bool          `yaml:"defaultAllow"`
	SubjectKey           string        `yaml:"subjectKey"`
	SkipPaths            []string      `yaml:"skipPaths"`
	CacheSize            int           `yaml:"cacheSize"`
	CacheTTL             int           `yaml:"cacheTTL"` // seconds
	PolicyRefreshInterval int          `yaml:"policyRefreshInterval"`
	Policies             []RBACPolicy  `yaml:"policies"`
}

// RBACPolicy represents an RBAC policy
type RBACPolicy struct {
	Name        string                      `yaml:"name"`
	Description string                      `yaml:"description"`
	Roles       map[string]RBACRole         `yaml:"roles"`
	Bindings    map[string][]string         `yaml:"bindings"`
}

// RBACRole represents an RBAC role
type RBACRole struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Permissions []string          `yaml:"permissions"`
	Inherits    []string          `yaml:"inherits"`
	Metadata    map[string]string `yaml:"metadata"`
}

// OpenAPIConfig represents OpenAPI configuration
type OpenAPIConfig struct {
	Enabled         bool                     `yaml:"enabled"`
	SpecsDirectory  string                   `yaml:"specsDirectory"`
	SpecURLs        []string                 `yaml:"specUrls"`
	DefaultService  string                   `yaml:"defaultService"`
	ReloadInterval  int                      `yaml:"reloadInterval"` // seconds
	WatchFiles      bool                     `yaml:"watchFiles"`
	ServiceMappings map[string]string        `yaml:"serviceMappings"`
	Descriptors     *OpenAPIDescriptorConfig `yaml:"descriptors,omitempty"`
	Manager         *OpenAPIManagerConfig    `yaml:"manager,omitempty"`
}

// OpenAPIDescriptorConfig holds configuration for dynamic OpenAPI descriptor loading
type OpenAPIDescriptorConfig struct {
	SpecFiles      []string      `yaml:"specFiles"`
	SpecDirs       []string      `yaml:"specDirs"`
	SpecURLs       []string      `yaml:"specUrls"`
	AutoReload     bool          `yaml:"autoReload"`
	ReloadInterval time.Duration `yaml:"reloadInterval"`
	FailOnError    bool          `yaml:"failOnError"`
	FileExtensions []string      `yaml:"fileExtensions"`
	DefaultService string        `yaml:"defaultService"`
}

// OpenAPIManagerConfig holds configuration for the OpenAPI manager
type OpenAPIManagerConfig struct {
	UpdateStrategy     string            `yaml:"updateStrategy"`     // merge, replace, append
	ConflictResolution string            `yaml:"conflictResolution"` // newest, error, skip
	RoutePrefix        string            `yaml:"routePrefix"`
	RouteGeneration    *RouteGeneration  `yaml:"routeGeneration"`
}

// RouteGeneration holds route generation options
type RouteGeneration struct {
	IncludeOptions       bool   `yaml:"includeOptions"`
	IncludeCORS          bool   `yaml:"includeCORS"`
	PathStyle            string `yaml:"pathStyle"` // exact, prefix, wildcard
	OperationIDAsRouteID bool   `yaml:"operationIdAsRouteId"`
}

// TransformConfig represents transformation middleware configuration
type TransformConfig struct {
	Enabled            bool                             `yaml:"enabled"`
	RequestTransforms  map[string]TransformRule         `yaml:"request"`
	ResponseTransforms map[string]TransformRule         `yaml:"response"`
	GlobalRequest      *TransformRule                   `yaml:"globalRequest"`
	GlobalResponse     *TransformRule                   `yaml:"globalResponse"`
}

// TransformRule represents a transformation rule
type TransformRule struct {
	Headers    *HeaderTransform    `yaml:"headers"`
	Body       *BodyTransform      `yaml:"body"`
	Conditions []TransformCondition `yaml:"conditions"`
}

// HeaderTransform represents header transformations
type HeaderTransform struct {
	Add    map[string]string `yaml:"add"`
	Remove []string          `yaml:"remove"`
	Rename map[string]string `yaml:"rename"`
	Modify map[string]string `yaml:"modify"`
}

// BodyTransform represents body transformations
type BodyTransform struct {
	Operations []TransformOperation `yaml:"operations"`
	Format     string               `yaml:"format"`
}

// TransformOperation represents a transformation operation
type TransformOperation struct {
	Type   string      `yaml:"type"`
	Path   string      `yaml:"path"`
	Value  interface{} `yaml:"value"`
	From   string      `yaml:"from"`
	To     string      `yaml:"to"`
	Script string      `yaml:"script"`
}

// TransformCondition represents a transformation condition
type TransformCondition struct {
	Header      string `yaml:"header"`
	Value       string `yaml:"value"`
	ContentType string `yaml:"contentType"`
	Method      string `yaml:"method"`
}

// VersioningConfig represents API versioning configuration
type VersioningConfig struct {
	Enabled            bool                               `yaml:"enabled"`
	Strategy           string                             `yaml:"strategy"`           // path, header, query, accept
	DefaultVersion     string                             `yaml:"defaultVersion"`
	VersionHeader      string                             `yaml:"versionHeader"`      // For header strategy
	VersionQuery       string                             `yaml:"versionQuery"`       // For query strategy
	AcceptPattern      string                             `yaml:"acceptPattern"`      // For accept strategy
	DeprecatedVersions map[string]*DeprecationInfo        `yaml:"deprecatedVersions"`
	VersionMappings    map[string]*VersionMapping         `yaml:"versionMappings"`
}

// DeprecationInfo represents deprecation details for a version
type DeprecationInfo struct {
	Message     string `yaml:"message"`
	SunsetDate  string `yaml:"sunsetDate"`
	RemovalDate string `yaml:"removalDate"`
}

// VersionMapping represents version-specific configuration
type VersionMapping struct {
	Service         string                 `yaml:"service"`
	PathPrefix      string                 `yaml:"pathPrefix"`
	Transformations map[string]interface{} `yaml:"transformations"`
	Deprecated      bool                   `yaml:"deprecated"`
}
