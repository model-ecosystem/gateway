package cors

import (
	"net/http"
	"strconv"
	"strings"
)

// Config holds CORS configuration
type Config struct {
	// AllowedOrigins is a list of allowed origins. Use ["*"] to allow all origins.
	AllowedOrigins []string
	// AllowedMethods is a list of allowed HTTP methods
	AllowedMethods []string
	// AllowedHeaders is a list of allowed headers
	AllowedHeaders []string
	// ExposedHeaders is a list of headers that browsers are allowed to access
	ExposedHeaders []string
	// AllowCredentials indicates whether the request can include user credentials
	AllowCredentials bool
	// MaxAge indicates how long (in seconds) the results of a preflight request can be cached
	MaxAge int
	// OptionsPassthrough instructs to pass OPTIONS requests to the next handler
	OptionsPassthrough bool
	// OptionsSuccessStatus is the status code for successful OPTIONS requests
	OptionsSuccessStatus int
}

// DefaultConfig returns a default CORS configuration
func DefaultConfig() Config {
	return Config{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodHead,
			http.MethodOptions,
		},
		AllowedHeaders:       []string{"*"},
		ExposedHeaders:       []string{},
		AllowCredentials:     false,
		MaxAge:               86400, // 24 hours
		OptionsPassthrough:   false,
		OptionsSuccessStatus: http.StatusNoContent,
	}
}

// CORS provides Cross-Origin Resource Sharing middleware
type CORS struct {
	config         Config
	allowedOrigins map[string]bool
	allowedHeaders map[string]bool
}

// New creates a new CORS middleware handler
func New(config Config) *CORS {
	// Normalize and validate configuration
	if len(config.AllowedOrigins) == 0 {
		config.AllowedOrigins = []string{"*"}
	}
	if len(config.AllowedMethods) == 0 {
		config.AllowedMethods = DefaultConfig().AllowedMethods
	}
	if config.OptionsSuccessStatus == 0 {
		config.OptionsSuccessStatus = http.StatusNoContent
	}
	
	// Pre-process allowed origins for faster lookup
	allowedOrigins := make(map[string]bool)
	for _, origin := range config.AllowedOrigins {
		allowedOrigins[strings.ToLower(origin)] = true
	}
	
	// Pre-process allowed headers for faster lookup
	allowedHeaders := make(map[string]bool)
	for _, header := range config.AllowedHeaders {
		allowedHeaders[strings.ToLower(header)] = true
	}
	
	return &CORS{
		config:         config,
		allowedOrigins: allowedOrigins,
		allowedHeaders: allowedHeaders,
	}
}

// Handler returns an HTTP handler that applies CORS headers
func (c *CORS) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		
		// Check if this is a preflight request
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			c.handlePreflight(w, r, origin)
			if !c.config.OptionsPassthrough {
				return
			}
		} else {
			c.handleActualRequest(w, r, origin)
		}
		
		next.ServeHTTP(w, r)
	})
}

// handlePreflight handles CORS preflight requests
func (c *CORS) handlePreflight(w http.ResponseWriter, r *http.Request, origin string) {
	headers := w.Header()
	
	// Check origin
	if c.isOriginAllowed(origin) {
		headers.Set("Access-Control-Allow-Origin", origin)
		headers.Add("Vary", "Origin")
		
		if c.config.AllowCredentials {
			headers.Set("Access-Control-Allow-Credentials", "true")
		}
	}
	
	// Handle request method
	reqMethod := r.Header.Get("Access-Control-Request-Method")
	if c.isMethodAllowed(reqMethod) {
		headers.Set("Access-Control-Allow-Methods", strings.Join(c.config.AllowedMethods, ", "))
	}
	
	// Handle request headers
	reqHeaders := r.Header.Get("Access-Control-Request-Headers")
	if reqHeaders != "" {
		if c.areHeadersAllowed(reqHeaders) {
			headers.Set("Access-Control-Allow-Headers", reqHeaders)
		}
	}
	
	// Set max age
	if c.config.MaxAge > 0 {
		headers.Set("Access-Control-Max-Age", strconv.Itoa(c.config.MaxAge))
	}
	
	// Write status for OPTIONS request
	if !c.config.OptionsPassthrough {
		w.WriteHeader(c.config.OptionsSuccessStatus)
	}
}

// handleActualRequest handles actual CORS requests (not preflight)
func (c *CORS) handleActualRequest(w http.ResponseWriter, r *http.Request, origin string) {
	headers := w.Header()
	
	// Check origin
	if c.isOriginAllowed(origin) {
		headers.Set("Access-Control-Allow-Origin", origin)
		headers.Add("Vary", "Origin")
		
		if c.config.AllowCredentials {
			headers.Set("Access-Control-Allow-Credentials", "true")
		}
	}
	
	// Expose headers
	if len(c.config.ExposedHeaders) > 0 {
		headers.Set("Access-Control-Expose-Headers", strings.Join(c.config.ExposedHeaders, ", "))
	}
}

// isOriginAllowed checks if the origin is allowed
func (c *CORS) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	
	// Check for wildcard
	if c.allowedOrigins["*"] {
		return true
	}
	
	// Check exact match
	return c.allowedOrigins[strings.ToLower(origin)]
}

// isMethodAllowed checks if the method is allowed
func (c *CORS) isMethodAllowed(method string) bool {
	for _, allowed := range c.config.AllowedMethods {
		if strings.EqualFold(allowed, method) {
			return true
		}
	}
	return false
}

// areHeadersAllowed checks if the requested headers are allowed
func (c *CORS) areHeadersAllowed(headers string) bool {
	// If wildcard is allowed, accept all headers
	if c.allowedHeaders["*"] {
		return true
	}
	
	// Check each requested header
	requested := strings.Split(headers, ",")
	for _, header := range requested {
		header = strings.TrimSpace(strings.ToLower(header))
		if !c.allowedHeaders[header] {
			return false
		}
	}
	
	return true
}