package router

import (
	"context"
	"fmt"
	"gateway/internal/core"
	"gateway/pkg/errors"
	"gateway/pkg/routing"
	"log/slog"
	"net/http"
	"sync"
)

// Router routes requests to services
type Router struct {
	mux       *http.ServeMux
	registry  core.ServiceRegistry
	routes    map[string]*core.RouteRule // pattern -> rule mapping
	mu        sync.RWMutex
	logger    *slog.Logger
}

// NewRouter creates a new router
func NewRouter(registry core.ServiceRegistry, logger *slog.Logger) *Router {
	if logger == nil {
		logger = slog.Default()
	}
	return &Router{
		mux:       http.NewServeMux(),
		registry:  registry,
		routes:    make(map[string]*core.RouteRule),
		logger:    logger.With("component", "router"),
	}
}

// AddRule adds a routing rule
func (r *Router) AddRule(rule core.RouteRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check duplicate ID
	for _, existing := range r.routes {
		if existing.ID == rule.ID {
			return errors.NewError(errors.ErrorTypeBadRequest, fmt.Sprintf("duplicate rule id: %s", rule.ID))
		}
	}

	// Convert path pattern to ServeMux format
	pattern := routing.ConvertToServeMuxPattern(rule.Path)

	// Register routes for each method (or all methods if none specified)
	methods := rule.Methods
	if len(methods) == 0 {
		methods = []string{""} // empty means all methods
	}

	for _, method := range methods {
		var muxPattern string
		if method == "" {
			// Register for all methods
			muxPattern = pattern
		} else {
			// Method-specific pattern
			muxPattern = method + " " + pattern
		}

		// Store rule reference
		r.routes[muxPattern] = &rule

		// Register handler
		r.mux.HandleFunc(muxPattern, func(w http.ResponseWriter, req *http.Request) {
			// This handler is just for route matching, not actual handling
			// The pattern is stored in the request context by ServeMux
		})
	}

	// Create load balancer for this route
	switch rule.LoadBalance {
	case core.LoadBalanceStickySession:
		// Use sticky session with round-robin fallback
		fallback := NewRoundRobinBalancer()
		rule.Balancer = NewStickySessionBalancer(fallback, rule.SessionAffinity)
	case core.LoadBalanceWeightedRoundRobin:
		rule.Balancer = NewWeightedRoundRobinBalancer()
	case core.LoadBalanceWeightedRandom:
		rule.Balancer = NewWeightedRandomBalancer()
	case core.LoadBalanceLeastConnections:
		rule.Balancer = NewLeastConnectionsBalancer()
	case core.LoadBalanceResponseTime:
		rule.Balancer = NewResponseTimeBalancer()
	case core.LoadBalanceAdaptive:
		rule.Balancer = NewAdaptiveBalancer()
	case core.LoadBalanceConsistentHash:
		rule.Balancer = NewConsistentHashBalancer(150) // Default 150 virtual nodes
	default:
		rule.Balancer = NewRoundRobinBalancer()
	}

	return nil
}

// Route finds a service instance for the request
func (r *Router) Route(ctx context.Context, req core.Request) (*core.RouteResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a fake http.Request for ServeMux matching
	httpReq, err := http.NewRequest(req.Method(), req.Path(), nil)
	if err != nil {
		return nil, errors.NewError(errors.ErrorTypeBadRequest, "invalid request").
			WithCause(err)
	}

	// Use ServeMux to find the matching handler
	_, pattern := r.mux.Handler(httpReq)
	if pattern == "" {
		return nil, errors.NewError(errors.ErrorTypeNotFound, "route not found").
			WithDetail("method", req.Method()).
			WithDetail("path", req.Path())
	}

	// Look up the rule by pattern
	var matched *core.RouteRule

	// First try method-specific pattern
	methodPattern := req.Method() + " " + pattern
	if rule, ok := r.routes[methodPattern]; ok {
		matched = rule
	} else if rule, ok := r.routes[pattern]; ok {
		// Fall back to method-agnostic pattern
		// But verify method is allowed if methods are specified
		if len(rule.Methods) == 0 || matchMethod(rule.Methods, req.Method()) {
			matched = rule
		}
	}

	if matched == nil {
		return nil, errors.NewError(errors.ErrorTypeNotFound, "route not found").
			WithDetail("method", req.Method()).
			WithDetail("path", req.Path())
	}

	// Check for version-based service override
	serviceName := matched.ServiceName
	if serviceOverride := getServiceOverrideFromContext(ctx); serviceOverride != "" {
		serviceName = serviceOverride
		r.logger.Debug("Using version-specific service", 
			"original", matched.ServiceName,
			"override", serviceName)
	}

	// Get instances
	instances, err := r.registry.GetService(serviceName)
	if err != nil {
		return nil, errors.NewError(errors.ErrorTypeNotFound, "service not found").
			WithDetail("service", serviceName).
			WithCause(err)
	}

	if len(instances) == 0 {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "no instances available").
			WithDetail("service", matched.ServiceName)
	}

	// Select instance using route's balancer
	balancer := matched.Balancer
	if balancer == nil {
		// Defensive check - this should never happen
		return nil, errors.NewError(errors.ErrorTypeInternal, "no balancer configured for route")
	}

	// Check if balancer supports request-based selection
	var instance *core.ServiceInstance
	if requestAwareBalancer, ok := balancer.(core.RequestAwareLoadBalancer); ok {
		instance, err = requestAwareBalancer.SelectForRequest(req, instances)
	} else {
		instance, err = balancer.Select(instances)
	}

	if err != nil {
		return nil, err
	}

	return &core.RouteResult{
		Instance:    instance,
		Rule:        matched,
		ServiceName: serviceName,
	}, nil
}

// getServiceOverrideFromContext extracts service override from context
func getServiceOverrideFromContext(ctx context.Context) string {
	if service, ok := ctx.Value("version.service").(string); ok {
		return service
	}
	return ""
}

// matchMethod is still needed for method validation
// when a route is registered without specific methods

func matchMethod(methods []string, method string) bool {
	if len(methods) == 0 {
		return true
	}
	for _, m := range methods {
		if m == method {
			return true
		}
	}
	return false
}

// GetRoutes returns all configured routes
func (r *Router) GetRoutes() []core.RouteRule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := make([]core.RouteRule, 0, len(r.routes))
	seen := make(map[string]bool)
	
	for _, rule := range r.routes {
		// Avoid duplicates (same route might be registered for multiple methods)
		if !seen[rule.ID] {
			routes = append(routes, *rule)
			seen[rule.ID] = true
		}
	}
	
	return routes
}

// Close cleans up the router resources
func (r *Router) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Close all balancers that implement io.Closer
	for _, rule := range r.routes {
		if rule.Balancer != nil {
			if closer, ok := rule.Balancer.(interface{ Close() error }); ok {
				closer.Close()
			}
		}
	}

	return nil
}
