package router

import (
	"context"
	"fmt"
	"gateway/internal/core"
	"gateway/pkg/errors"
	"gateway/pkg/routing"
	"net/http"
	"sync"
)

// Router routes requests to services
type Router struct {
	mux       *http.ServeMux
	registry  core.ServiceRegistry
	balancers map[string]core.LoadBalancer
	routes    map[string]*core.RouteRule // pattern -> rule mapping
	mu        sync.RWMutex
}

// NewRouter creates a new router
func NewRouter(registry core.ServiceRegistry) *Router {
	return &Router{
		mux:       http.NewServeMux(),
		registry:  registry,
		balancers: make(map[string]core.LoadBalancer),
		routes:    make(map[string]*core.RouteRule),
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

	// Create load balancer if needed
	if _, ok := r.balancers[rule.ServiceName]; !ok {
		switch rule.LoadBalance {
		case core.LoadBalanceStickySession:
			// Use sticky session with round-robin fallback
			fallback := NewRoundRobinBalancer()
			r.balancers[rule.ServiceName] = NewStickySessionBalancer(fallback, rule.SessionAffinity)
		default:
			r.balancers[rule.ServiceName] = NewRoundRobinBalancer()
		}
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

	// Get instances
	instances, err := r.registry.GetService(matched.ServiceName)
	if err != nil {
		return nil, errors.NewError(errors.ErrorTypeNotFound, "service not found").
			WithDetail("service", matched.ServiceName).
			WithCause(err)
	}

	if len(instances) == 0 {
		return nil, errors.NewError(errors.ErrorTypeUnavailable, "no instances available").
			WithDetail("service", matched.ServiceName)
	}

	// Select instance
	balancer := r.balancers[matched.ServiceName]

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
		Instance: instance,
		Rule:     matched,
	}, nil
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
