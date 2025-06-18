package core

import (
	"context"
	"io"
	"time"
)

// Request represents an incoming request
type Request interface {
	ID() string
	Method() string
	Path() string
	URL() string
	RemoteAddr() string
	Headers() map[string][]string
	Body() io.ReadCloser
	Context() context.Context
}

// Response represents an outgoing response
type Response interface {
	StatusCode() int
	Headers() map[string][]string
	Body() io.ReadCloser
}

// Handler processes requests
type Handler func(context.Context, Request) (Response, error)

// Middleware wraps handlers
type Middleware func(Handler) Handler

// ServiceInstance represents a backend service instance
type ServiceInstance struct {
	ID       string
	Name     string
	Address  string
	Port     int
	Scheme   string
	Healthy  bool
	Metadata map[string]any
}

// ServiceRegistry provides service discovery
type ServiceRegistry interface {
	GetService(name string) ([]ServiceInstance, error)
}

// RouteResult contains the result of routing
type RouteResult struct {
	Instance *ServiceInstance
	Rule     *RouteRule
}

// Router routes requests to services
type Router interface {
	Route(context.Context, Request) (*RouteResult, error)
}

// LoadBalancer selects instances
type LoadBalancer interface {
	Select([]ServiceInstance) (*ServiceInstance, error)
}

// RouteRule defines routing configuration
type RouteRule struct {
	ID          string
	Path        string
	Methods     []string
	ServiceName string
	LoadBalance LoadBalanceStrategy
	Timeout     time.Duration
}

// LoadBalanceStrategy defines load balancing algorithm
type LoadBalanceStrategy string

const (
	LoadBalanceRoundRobin LoadBalanceStrategy = "round_robin"
)