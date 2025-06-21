package grpc

import (
	"bytes"
	"context"
	"fmt"
	grpcConnector "gateway/internal/connector/grpc"
	"gateway/internal/core"
	"gateway/pkg/errors"
	"io"
	"log/slog"
	"net/http"
)

// Adapter implements HTTP to gRPC gateway functionality
type Adapter struct {
	transcoder *grpcConnector.Transcoder
	logger     *slog.Logger
}

// NewAdapter creates a new HTTP to gRPC adapter
func NewAdapter(logger *slog.Logger) *Adapter {
	return &Adapter{
		transcoder: grpcConnector.NewTranscoder(logger),
		logger:     logger,
	}
}

// TranscodeRequest converts HTTP request to gRPC format
func (a *Adapter) TranscodeRequest(req core.Request) (core.Request, error) {
	// Check if this is a gRPC-Web request
	contentType := ""
	if headers := req.Headers()["Content-Type"]; len(headers) > 0 {
		contentType = headers[0]
	}

	isGRPCWeb := contentType == "application/grpc-web" ||
		contentType == "application/grpc-web+proto" ||
		contentType == "application/grpc-web+json"

	if isGRPCWeb {
		// Pass through gRPC-Web requests
		return req, nil
	}

	// For regular HTTP requests, check if path looks like gRPC
	if !isGRPCPath(req.Path()) {
		// Not a gRPC path, return original request
		return req, nil
	}

	// Transcode HTTP to gRPC
	grpcReq, err := a.transcoder.HTTPToGRPC(req)
	if err != nil {
		return nil, errors.NewError(
			errors.ErrorTypeBadRequest,
			"failed to transcode HTTP to gRPC",
		).WithCause(err)
	}

	// Create new request with gRPC format
	return &transcodedRequest{
		original: req,
		grpcReq:  grpcReq,
	}, nil
}

// TranscodeResponse converts gRPC response to HTTP format
func (a *Adapter) TranscodeResponse(resp core.Response) (core.Response, error) {
	// Check if this is a gRPC response that needs transcoding
	if resp.StatusCode() != 200 {
		// gRPC always returns 200, so this is already an HTTP response
		return resp, nil
	}

	// Read response body
	body, err := io.ReadAll(resp.Body())
	if err != nil {
		return nil, errors.NewError(
			errors.ErrorTypeInternal,
			"failed to read gRPC response",
		).WithCause(err)
	}

	// Transcode gRPC to HTTP
	httpResp := a.transcoder.GRPCToHTTP(body, resp.Headers())

	return httpResp.ToResponse(), nil
}

// HandleGRPCWebPreflight returns headers for gRPC-Web CORS preflight
func (a *Adapter) HandleGRPCWebPreflight() map[string][]string {
	return map[string][]string{
		"Access-Control-Allow-Origin":  {"*"},
		"Access-Control-Allow-Methods": {"POST, GET, OPTIONS"},
		"Access-Control-Allow-Headers": {"Content-Type, X-Grpc-Web, X-User-Agent"},
		"Access-Control-Max-Age":       {"86400"},
	}
}

// isGRPCPath checks if the path looks like a gRPC method
func isGRPCPath(path string) bool {
	// gRPC paths follow the pattern: /package.Service/Method
	if len(path) < 2 || path[0] != '/' {
		return false
	}

	// Count dots and slashes
	dots := 0
	slashes := 0
	for _, c := range path {
		if c == '.' {
			dots++
		} else if c == '/' {
			slashes++
		}
	}

	// Should have at least one dot and exactly two slashes
	return dots >= 1 && slashes == 2
}

// transcodedRequest wraps an HTTP request with gRPC transcoding
type transcodedRequest struct {
	original core.Request
	grpcReq  *grpcConnector.GRPCRequest
}

func (r *transcodedRequest) ID() string     { return r.original.ID() }
func (r *transcodedRequest) Method() string { return "POST" } // gRPC is always POST
func (r *transcodedRequest) Path() string {
	return fmt.Sprintf("/%s/%s", r.grpcReq.Service, r.grpcReq.Method)
}
func (r *transcodedRequest) URL() string                  { return r.original.URL() }
func (r *transcodedRequest) RemoteAddr() string           { return r.original.RemoteAddr() }
func (r *transcodedRequest) Headers() map[string][]string { return r.grpcReq.Headers }
func (r *transcodedRequest) Body() io.ReadCloser {
	return io.NopCloser(bytes.NewReader(r.grpcReq.Body))
}
func (r *transcodedRequest) Context() context.Context { return r.original.Context() }

// GRPCWebMiddleware handles gRPC-Web protocol
func GRPCWebMiddleware() core.Middleware {
	return func(next core.Handler) core.Handler {
		return func(ctx context.Context, req core.Request) (core.Response, error) {
			// Check for gRPC-Web headers
			contentType := ""
			if headers := req.Headers()["Content-Type"]; len(headers) > 0 {
				contentType = headers[0]
			}

			isGRPCWeb := contentType == "application/grpc-web" ||
				contentType == "application/grpc-web+proto" ||
				contentType == "application/grpc-web+json"

			if !isGRPCWeb {
				return next(ctx, req)
			}

			// Handle CORS for gRPC-Web
			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			// Add gRPC-Web specific headers
			if resp != nil {
				headers := resp.Headers()
				headers["Access-Control-Allow-Origin"] = []string{"*"}
				headers["Access-Control-Allow-Methods"] = []string{"POST, GET, OPTIONS"}
				headers["Access-Control-Allow-Headers"] = []string{"Content-Type, X-Grpc-Web, X-User-Agent"}
				headers["Access-Control-Expose-Headers"] = []string{"Grpc-Status, Grpc-Message"}
			}

			return resp, nil
		}
	}
}

// CORSPreflight handles OPTIONS requests for gRPC-Web
func CORSPreflight() core.Handler {
	return func(ctx context.Context, req core.Request) (core.Response, error) {
		if req.Method() != "OPTIONS" {
			return nil, errors.NewError(
				errors.ErrorTypeNotFound,
				"not found",
			)
		}

		headers := make(map[string][]string)
		headers["Access-Control-Allow-Origin"] = []string{"*"}
		headers["Access-Control-Allow-Methods"] = []string{"POST, GET, OPTIONS"}
		headers["Access-Control-Allow-Headers"] = []string{"Content-Type, X-Grpc-Web, X-User-Agent"}
		headers["Access-Control-Max-Age"] = []string{"86400"}

		return &corsResponse{
			headers: headers,
		}, nil
	}
}

// corsResponse implements core.Response for CORS preflight
type corsResponse struct {
	headers map[string][]string
}

func (r *corsResponse) StatusCode() int              { return http.StatusNoContent }
func (r *corsResponse) Headers() map[string][]string { return r.headers }
func (r *corsResponse) Body() io.ReadCloser          { return io.NopCloser(bytes.NewReader(nil)) }
