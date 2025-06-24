package router

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"gateway/internal/core"
)

// DynamicRouter extends the base router with dynamic route management capabilities
type DynamicRouter struct {
	base          *Router
	mu            sync.RWMutex
	dynamicRoutes map[string][]core.RouteRule // source -> routes
	logger        *slog.Logger
}

// NewDynamicRouter creates a new dynamic router
func NewDynamicRouter(registry core.ServiceRegistry, logger *slog.Logger) *DynamicRouter {
	if logger == nil {
		logger = slog.Default()
	}

	return &DynamicRouter{
		base:          NewRouter(registry, logger),
		dynamicRoutes: make(map[string][]core.RouteRule),
		logger:        logger.With("component", "dynamic_router"),
	}
}

// UpdateRoutes updates routes from a specific source (e.g., OpenAPI spec file)
func (r *DynamicRouter) UpdateRoutes(source string, routes []core.RouteRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove old routes from this source (no-op for now since base router doesn't support removal)
	// In a production system, we'd need to enhance the base router or recreate it
	
	// For now, just track the routes
	r.dynamicRoutes[source] = routes

	// Add new routes to base router
	var errors []error
	addedRoutes := make([]core.RouteRule, 0, len(routes))
	
	for _, route := range routes {
		// Ensure route has a unique ID that includes the source
		if route.ID == "" {
			route.ID = fmt.Sprintf("%s:%s:%s", source, route.Path, route.ServiceName)
		} else {
			route.ID = fmt.Sprintf("%s:%s", source, route.ID)
		}
		
		// Add metadata to track source
		if route.Metadata == nil {
			route.Metadata = make(map[string]interface{})
		}
		route.Metadata["source"] = source
		route.Metadata["dynamic"] = true
		
		if err := r.base.AddRule(route); err != nil {
			// Skip if already exists (might be from a previous load)
			if !strings.Contains(err.Error(), "duplicate rule id") {
				errors = append(errors, fmt.Errorf("route %s: %w", route.ID, err))
			}
		} else {
			addedRoutes = append(addedRoutes, route)
		}
	}

	// Store the successfully added routes
	r.dynamicRoutes[source] = addedRoutes

	r.logger.Info("Updated dynamic routes",
		"source", source,
		"added", len(addedRoutes),
		"failed", len(errors),
	)

	if len(errors) > 0 {
		return fmt.Errorf("failed to add %d routes", len(errors))
	}

	return nil
}

// RemoveRoutes removes all routes from a specific source
func (r *DynamicRouter) RemoveRoutes(source string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, exists := r.dynamicRoutes[source]
	if !exists {
		return nil // No routes to remove
	}

	// Remove from dynamic routes map
	delete(r.dynamicRoutes, source)

	r.logger.Info("Removed dynamic routes tracking",
		"source", source,
	)

	// Note: The base router doesn't support route removal yet
	// In a production system, we'd need to enhance it or recreate the router
	
	return nil
}

// GetDynamicSources returns all dynamic route sources
func (r *DynamicRouter) GetDynamicSources() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sources := make([]string, 0, len(r.dynamicRoutes))
	for source := range r.dynamicRoutes {
		sources = append(sources, source)
	}
	return sources
}

// GetRoutesBySource returns all routes from a specific source
func (r *DynamicRouter) GetRoutesBySource(source string) []core.RouteRule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if routes, exists := r.dynamicRoutes[source]; exists {
		// Return a copy
		result := make([]core.RouteRule, len(routes))
		copy(result, routes)
		return result
	}
	return nil
}

// GetAllDynamicRoutes returns all dynamic routes
func (r *DynamicRouter) GetAllDynamicRoutes() map[string][]core.RouteRule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy
	result := make(map[string][]core.RouteRule, len(r.dynamicRoutes))
	for source, routes := range r.dynamicRoutes {
		routesCopy := make([]core.RouteRule, len(routes))
		copy(routesCopy, routes)
		result[source] = routesCopy
	}
	return result
}

// Route delegates to the base router
func (r *DynamicRouter) Route(ctx context.Context, req core.Request) (*core.RouteResult, error) {
	return r.base.Route(ctx, req)
}

// AddRule delegates to the base router
func (r *DynamicRouter) AddRule(rule core.RouteRule) error {
	return r.base.AddRule(rule)
}

// GetRoutes returns all routes including dynamic ones
func (r *DynamicRouter) GetRoutes() []core.RouteRule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var allRoutes []core.RouteRule
	for _, routes := range r.dynamicRoutes {
		allRoutes = append(allRoutes, routes...)
	}
	return allRoutes
}