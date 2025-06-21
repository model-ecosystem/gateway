package grpc

import (
	"bytes"
	"io"
	"net/http"
	"reflect"
	"testing"

	"log/slog"
)

func TestNewTranscoder(t *testing.T) {
	logger := slog.Default()
	transcoder := NewTranscoder(logger)

	if transcoder == nil {
		t.Fatal("Expected transcoder, got nil")
	}
	if transcoder.logger != logger {
		t.Error("Logger not set correctly")
	}
}

func TestTranscoder_HTTPToGRPC(t *testing.T) {
	logger := slog.Default()
	transcoder := NewTranscoder(logger)

	tests := []struct {
		name      string
		request   *mockRequest
		want      *GRPCRequest
		wantError bool
	}{
		{
			name: "valid gRPC path",
			request: &mockRequest{
				path: "/package.Service/Method",
				headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
				body: io.NopCloser(bytes.NewReader([]byte(`{"key": "value"}`))),
			},
			want: &GRPCRequest{
				Service: "package.Service",
				Method:  "Method",
				Body:    []byte(`{"key": "value"}`),
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
			},
			wantError: false,
		},
		{
			name: "complex service name",
			request: &mockRequest{
				path:    "/com.example.api.v1.UserService/GetUser",
				headers: map[string][]string{},
				body:    io.NopCloser(bytes.NewReader([]byte(`{"id": 123}`))),
			},
			want: &GRPCRequest{
				Service: "com.example.api.v1.UserService",
				Method:  "GetUser",
				Body:    []byte(`{"id": 123}`),
				Headers: map[string][]string{},
			},
			wantError: false,
		},
		{
			name: "empty body",
			request: &mockRequest{
				path:    "/package.Service/Method",
				headers: map[string][]string{},
				body:    io.NopCloser(bytes.NewReader([]byte{})),
			},
			want: &GRPCRequest{
				Service: "package.Service",
				Method:  "Method",
				Body:    []byte{},
				Headers: map[string][]string{},
			},
			wantError: false,
		},
		{
			name: "invalid path format",
			request: &mockRequest{
				path: "/invalid",
				body: io.NopCloser(bytes.NewReader([]byte{})),
			},
			wantError: true,
		},
		{
			name: "path with too many slashes",
			request: &mockRequest{
				path:    "/package/Service/Method/Extra",
				headers: map[string][]string{},
				body:    io.NopCloser(bytes.NewReader([]byte{})),
			},
			want: &GRPCRequest{
				Service: "package",
				Method:  "Service",
				Body:    []byte{},
				Headers: map[string][]string{},
			},
			wantError: false, // Will parse but with different result
		},
		{
			name: "body read error",
			request: &mockRequest{
				path: "/package.Service/Method",
				body: io.NopCloser(&errorReader{}),
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transcoder.HTTPToGRPC(tt.request)

			if (err != nil) != tt.wantError {
				t.Errorf("HTTPToGRPC() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && got != nil {
				if got.Service != tt.want.Service {
					t.Errorf("Service = %v, want %v", got.Service, tt.want.Service)
				}
				if got.Method != tt.want.Method {
					t.Errorf("Method = %v, want %v", got.Method, tt.want.Method)
				}
				if !bytes.Equal(got.Body, tt.want.Body) {
					t.Errorf("Body = %v, want %v", got.Body, tt.want.Body)
				}
				if tt.want.Headers != nil && !reflect.DeepEqual(got.Headers, tt.want.Headers) {
					t.Errorf("Headers = %v, want %v", got.Headers, tt.want.Headers)
				}
			}
		})
	}
}

func TestTranscoder_GRPCToHTTP(t *testing.T) {
	logger := slog.Default()
	transcoder := NewTranscoder(logger)

	tests := []struct {
		name    string
		resp    []byte
		headers map[string][]string
		want    *HTTPResponse
	}{
		{
			name: "basic response",
			resp: []byte(`{"result": "success"}`),
			headers: map[string][]string{
				"X-Custom": {"value"},
			},
			want: &HTTPResponse{
				StatusCode: http.StatusOK,
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
					"X-Custom":     {"value"},
				},
				Body: []byte(`{"result": "success"}`),
			},
		},
		{
			name: "with content type",
			resp: []byte(`{"data": []}`),
			headers: map[string][]string{
				"Content-Type": {"application/json; charset=utf-8"},
			},
			want: &HTTPResponse{
				StatusCode: http.StatusOK,
				Headers: map[string][]string{
					"Content-Type": {"application/json; charset=utf-8"},
				},
				Body: []byte(`{"data": []}`),
			},
		},
		{
			name:    "empty response",
			resp:    []byte{},
			headers: nil,
			want: &HTTPResponse{
				StatusCode: http.StatusOK,
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
				Body: []byte{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transcoder.GRPCToHTTP(tt.resp, tt.headers)

			if got.StatusCode != tt.want.StatusCode {
				t.Errorf("StatusCode = %v, want %v", got.StatusCode, tt.want.StatusCode)
			}
			if !bytes.Equal(got.Body, tt.want.Body) {
				t.Errorf("Body = %v, want %v", got.Body, tt.want.Body)
			}
			if !reflect.DeepEqual(got.Headers, tt.want.Headers) {
				t.Errorf("Headers = %v, want %v", got.Headers, tt.want.Headers)
			}
		})
	}
}

func TestHTTPResponse_ToResponse(t *testing.T) {
	httpResp := &HTTPResponse{
		StatusCode: http.StatusOK,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
			"X-Custom":     {"value"},
		},
		Body: []byte(`{"result": "ok"}`),
	}

	resp := httpResp.ToResponse()

	if resp.StatusCode() != httpResp.StatusCode {
		t.Errorf("StatusCode() = %v, want %v", resp.StatusCode(), httpResp.StatusCode)
	}

	headers := resp.Headers()
	if !reflect.DeepEqual(headers, httpResp.Headers) {
		t.Errorf("Headers() = %v, want %v", headers, httpResp.Headers)
	}

	body, err := io.ReadAll(resp.Body())
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}
	if !bytes.Equal(body, httpResp.Body) {
		t.Errorf("Body = %v, want %v", body, httpResp.Body)
	}
}

// errorReader simulates a read error
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func (e *errorReader) Close() error {
	return nil
}
