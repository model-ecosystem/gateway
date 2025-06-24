package management

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"gateway/internal/config"
	"gateway/internal/core"
)

// API provides runtime management endpoints
type API struct {
	config       *config.Management
	logger       *slog.Logger
	server       *http.Server
	mux          *http.ServeMux
	mu           sync.RWMutex
	
	// References to managed components
	registry     core.ServiceRegistry
	router       interface{ GetRoutes() []core.RouteRule }
	healthChecker interface{ GetHealthStatus() map[string]bool }
	circuitBreaker interface{ GetStatus() map[string]string }
	rateLimiter   interface{ GetStats() map[string]interface{} }
	
	// Stats
	startTime    time.Time
	requestCount uint64
}

// NewAPI creates a new management API
func NewAPI(cfg *config.Management, logger *slog.Logger) *API {
	if cfg == nil {
		cfg = &config.Management{
			Enabled:  false,
			Host:     "127.0.0.1",
			Port:     9090,
			BasePath: "/management",
		}
	}

	api := &API{
		config:    cfg,
		logger:    logger.With("component", "management-api"),
		mux:       http.NewServeMux(),
		startTime: time.Now(),
	}

	// Setup routes
	api.setupRoutes()

	return api
}

// SetRegistry sets the service registry reference
func (api *API) SetRegistry(registry core.ServiceRegistry) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.registry = registry
}

// SetRouter sets the router reference
func (api *API) SetRouter(router interface{ GetRoutes() []core.RouteRule }) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.router = router
}

// SetHealthChecker sets the health checker reference
func (api *API) SetHealthChecker(hc interface{ GetHealthStatus() map[string]bool }) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.healthChecker = hc
}

// SetCircuitBreaker sets the circuit breaker reference
func (api *API) SetCircuitBreaker(cb interface{ GetStatus() map[string]string }) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.circuitBreaker = cb
}

// SetRateLimiter sets the rate limiter reference
func (api *API) SetRateLimiter(rl interface{ GetStats() map[string]interface{} }) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.rateLimiter = rl
}

// setupRoutes configures all management endpoints
func (api *API) setupRoutes() {
	basePath := api.config.BasePath
	if basePath == "" {
		basePath = "/management"
	}

	// Apply auth middleware if configured
	handler := http.Handler(api.mux)
	if api.config.Auth != nil {
		handler = api.authMiddleware(handler)
	}

	// Health endpoints
	api.mux.HandleFunc(basePath+"/health", api.handleHealth)
	api.mux.HandleFunc(basePath+"/health/live", api.handleLiveness)
	api.mux.HandleFunc(basePath+"/health/ready", api.handleReadiness)
	
	// Info endpoints
	api.mux.HandleFunc(basePath+"/info", api.handleInfo)
	api.mux.HandleFunc(basePath+"/stats", api.handleStats)
	
	// Service management
	api.mux.HandleFunc(basePath+"/services", api.handleServices)
	api.mux.HandleFunc(basePath+"/services/", api.handleServiceDetail)
	
	// Route management
	api.mux.HandleFunc(basePath+"/routes", api.handleRoutes)
	api.mux.HandleFunc(basePath+"/routes/reload", api.handleRouteReload)
	
	// Circuit breaker management
	api.mux.HandleFunc(basePath+"/circuit-breakers", api.handleCircuitBreakers)
	api.mux.HandleFunc(basePath+"/circuit-breakers/reset", api.handleCircuitBreakerReset)
	
	// Rate limiter management
	api.mux.HandleFunc(basePath+"/rate-limits", api.handleRateLimits)
	
	// Config endpoints
	api.mux.HandleFunc(basePath+"/config", api.handleConfig)
	api.mux.HandleFunc(basePath+"/config/reload", api.handleConfigReload)
}

// Start starts the management API server
func (api *API) Start(ctx context.Context) error {
	if !api.config.Enabled {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", api.config.Host, api.config.Port)
	api.server = &http.Server{
		Addr:    addr,
		Handler: api.mux,
	}

	go func() {
		api.logger.Info("Starting management API", "address", addr)
		if err := api.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			api.logger.Error("Management API error", "error", err)
		}
	}()

	return nil
}

// Stop stops the management API server
func (api *API) Stop(ctx context.Context) error {
	if api.server == nil {
		return nil
	}

	api.logger.Info("Stopping management API")
	return api.server.Shutdown(ctx)
}

// authMiddleware implements authentication for management endpoints
func (api *API) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if api.config.Auth == nil {
			next.ServeHTTP(w, r)
			return
		}

		switch api.config.Auth.Type {
		case "token":
			token := r.Header.Get("Authorization")
			if token == "" {
				token = r.URL.Query().Get("token")
			}
			if token != "Bearer "+api.config.Auth.Token && token != api.config.Auth.Token {
				api.writeError(w, http.StatusUnauthorized, "Invalid token")
				return
			}

		case "basic":
			username, password, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="Management API"`)
				api.writeError(w, http.StatusUnauthorized, "Authentication required")
				return
			}
			
			expectedPass, exists := api.config.Auth.Users[username]
			if !exists || password != expectedPass {
				api.writeError(w, http.StatusUnauthorized, "Invalid credentials")
				return
			}

		default:
			api.writeError(w, http.StatusInternalServerError, "Invalid auth configuration")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Response types
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Uptime    string            `json:"uptime"`
	Services  map[string]string `json:"services,omitempty"`
}

type InfoResponse struct {
	Version   string    `json:"version"`
	StartTime time.Time `json:"startTime"`
	Uptime    string    `json:"uptime"`
	GoVersion string    `json:"goVersion"`
}

type StatsResponse struct {
	Uptime       string                 `json:"uptime"`
	RequestCount uint64                 `json:"requestCount"`
	Services     int                    `json:"serviceCount"`
	Routes       int                    `json:"routeCount"`
	Memory       map[string]interface{} `json:"memory"`
}

type ServiceResponse struct {
	Name      string                 `json:"name"`
	Instances []core.ServiceInstance `json:"instances"`
}

type RouteResponse struct {
	Routes []core.RouteRule `json:"routes"`
}

// Handler implementations
func (api *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	resp := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Uptime:    time.Since(api.startTime).String(),
	}

	// Add service health status if available
	if api.healthChecker != nil {
		services := make(map[string]string)
		for svc, healthy := range api.healthChecker.GetHealthStatus() {
			if healthy {
				services[svc] = "healthy"
			} else {
				services[svc] = "unhealthy"
				resp.Status = "degraded"
			}
		}
		resp.Services = services
	}

	api.writeJSON(w, http.StatusOK, resp)
}

func (api *API) handleLiveness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (api *API) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Check if all components are ready
	ready := true
	if api.registry == nil || api.router == nil {
		ready = false
	}

	if ready {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Not Ready"))
	}
}

func (api *API) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	resp := InfoResponse{
		Version:   "1.0.0", // TODO: Get from build info
		StartTime: api.startTime,
		Uptime:    time.Since(api.startTime).String(),
		GoVersion: "go1.24.3",
	}

	api.writeJSON(w, http.StatusOK, resp)
}

func (api *API) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	serviceCount := 0
	if api.registry != nil {
		// Count services (implementation depends on registry interface)
		serviceCount = 10 // Placeholder
	}

	routeCount := 0
	if api.router != nil {
		routeCount = len(api.router.GetRoutes())
	}

	resp := StatsResponse{
		Uptime:       time.Since(api.startTime).String(),
		RequestCount: api.requestCount,
		Services:     serviceCount,
		Routes:       routeCount,
		Memory: map[string]interface{}{
			// TODO: Add memory stats
		},
	}

	api.writeJSON(w, http.StatusOK, resp)
}

func (api *API) handleServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if api.registry == nil {
		api.writeError(w, http.StatusServiceUnavailable, "Registry not available")
		return
	}

	// TODO: Implement service listing
	services := []ServiceResponse{}
	api.writeJSON(w, http.StatusOK, services)
}

func (api *API) handleServiceDetail(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement service detail
	api.writeError(w, http.StatusNotImplemented, "Not implemented")
}

func (api *API) handleRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if api.router == nil {
		api.writeError(w, http.StatusServiceUnavailable, "Router not available")
		return
	}

	resp := RouteResponse{
		Routes: api.router.GetRoutes(),
	}

	api.writeJSON(w, http.StatusOK, resp)
}

func (api *API) handleRouteReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// TODO: Implement route reload
	api.writeError(w, http.StatusNotImplemented, "Route reload not implemented")
}

func (api *API) handleCircuitBreakers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if api.circuitBreaker == nil {
		api.writeError(w, http.StatusServiceUnavailable, "Circuit breaker not available")
		return
	}

	status := api.circuitBreaker.GetStatus()
	api.writeJSON(w, http.StatusOK, status)
}

func (api *API) handleCircuitBreakerReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// TODO: Implement circuit breaker reset
	api.writeError(w, http.StatusNotImplemented, "Circuit breaker reset not implemented")
}

func (api *API) handleRateLimits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if api.rateLimiter == nil {
		api.writeError(w, http.StatusServiceUnavailable, "Rate limiter not available")
		return
	}

	stats := api.rateLimiter.GetStats()
	api.writeJSON(w, http.StatusOK, stats)
}

func (api *API) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// TODO: Return current configuration (sanitized)
	api.writeError(w, http.StatusNotImplemented, "Config endpoint not implemented")
}

func (api *API) handleConfigReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// TODO: Trigger configuration reload
	api.writeError(w, http.StatusNotImplemented, "Config reload not implemented")
}

// Helper methods
func (api *API) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		api.logger.Error("Failed to encode response", "error", err)
	}
}

func (api *API) writeError(w http.ResponseWriter, status int, message string) {
	api.writeJSON(w, status, map[string]string{
		"error": message,
	})
}