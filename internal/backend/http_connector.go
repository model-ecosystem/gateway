package backend

import (
	"context"
	"fmt"
	"gateway/internal/core"
	"gateway/pkg/errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPConnector implements Connector for HTTP backend services
type HTTPConnector struct {
	client         *http.Client
	defaultTimeout time.Duration
}

// NewHTTPConnector creates a new HTTP connector with provided client
func NewHTTPConnector(client *http.Client, defaultTimeout time.Duration) *HTTPConnector {
	return &HTTPConnector{
		client:         client,
		defaultTimeout: defaultTimeout,
	}
}

// Forward implements the Connector interface for HTTP backends
func (c *HTTPConnector) Forward(ctx context.Context, req core.Request, route *core.RouteResult) (core.Response, error) {
	instance := route.Instance
	
	// Apply route-specific timeout if configured
	timeout := c.defaultTimeout
	if route.Rule != nil && route.Rule.Timeout > 0 {
		timeout = route.Rule.Timeout
	}
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Build backend URL
	backendURL, err := c.buildBackendURL(req, instance)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build backend URL")
	}

	// Create HTTP request with context
	httpReq, err := http.NewRequestWithContext(ctx, req.Method(), backendURL, req.Body())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create backend request")
	}

	// Copy headers from original request
	for key, values := range req.Headers() {
		// Skip hop-by-hop headers
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	// Set X-Forwarded headers
	httpReq.Header.Set("X-Forwarded-For", req.RemoteAddr())
	httpReq.Header.Set("X-Forwarded-Proto", "http")
	if host := req.Headers()["Host"]; len(host) > 0 {
		httpReq.Header.Set("X-Forwarded-Host", host[0])
	}

	// Send request to backend
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send request to backend")
	}

	// Create and return streaming response
	return &httpResponse{
		statusCode: resp.StatusCode,
		headers:    resp.Header,
		body:       resp.Body,
	}, nil
}

func (c *HTTPConnector) buildBackendURL(req core.Request, instance *core.ServiceInstance) (string, error) {
	// Parse the original request URL
	u, err := url.Parse(req.URL())
	if err != nil {
		return "", err
	}

	// Build backend URL
	scheme := "http"
	if instance.Scheme != "" {
		scheme = instance.Scheme
	}

	backendURL := fmt.Sprintf("%s://%s:%d%s", scheme, instance.Address, instance.Port, u.RequestURI())
	return backendURL, nil
}

// isHopByHopHeader checks if a header is a hop-by-hop header
func isHopByHopHeader(header string) bool {
	hopByHopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"TE",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}
	
	header = strings.ToLower(header)
	for _, h := range hopByHopHeaders {
		if strings.ToLower(h) == header {
			return true
		}
	}
	return false
}

// httpResponse implements core.Response for HTTP responses
type httpResponse struct {
	statusCode int
	headers    http.Header
	body       io.ReadCloser
}

func (r *httpResponse) StatusCode() int {
	return r.statusCode
}

func (r *httpResponse) Headers() map[string][]string {
	return r.headers
}

func (r *httpResponse) Body() io.ReadCloser {
	return r.body
}