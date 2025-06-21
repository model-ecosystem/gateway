package connector

import (
	"context"
	"gateway/internal/core"
)

// Connector handles forwarding requests to backend service instances
type Connector interface {
	// Forward sends a request to the specified backend instance and returns the response
	Forward(ctx context.Context, req core.Request, route *core.RouteResult) (core.Response, error)
}
