package session

import (
	"context"
	"io"
	"testing"

	"gateway/internal/core"
)

// mockRequest implements core.Request for testing
type mockRequest struct {
	id      string
	method  string
	path    string
	url     string
	remote  string
	headers map[string][]string
	body    io.ReadCloser
	ctx     context.Context
}

func (r *mockRequest) ID() string                   { return r.id }
func (r *mockRequest) Method() string               { return r.method }
func (r *mockRequest) Path() string                 { return r.path }
func (r *mockRequest) URL() string                  { return r.url }
func (r *mockRequest) RemoteAddr() string           { return r.remote }
func (r *mockRequest) Headers() map[string][]string { return r.headers }
func (r *mockRequest) Body() io.ReadCloser          { return r.body }
func (r *mockRequest) Context() context.Context     { return r.ctx }

func TestCookieExtractor(t *testing.T) {
	tests := []struct {
		name       string
		cookieName string
		headers    map[string][]string
		want       string
	}{
		{
			name:       "single cookie",
			cookieName: "session",
			headers: map[string][]string{
				"Cookie": {"session=abc123"},
			},
			want: "abc123",
		},
		{
			name:       "multiple cookies",
			cookieName: "session",
			headers: map[string][]string{
				"Cookie": {"foo=bar; session=xyz789; baz=qux"},
			},
			want: "xyz789",
		},
		{
			name:       "quoted value",
			cookieName: "session",
			headers: map[string][]string{
				"Cookie": {`session="value with spaces"`},
			},
			want: "value with spaces",
		},
		{
			name:       "no cookie header",
			cookieName: "session",
			headers:    map[string][]string{},
			want:       "",
		},
		{
			name:       "cookie not found",
			cookieName: "session",
			headers: map[string][]string{
				"Cookie": {"other=value"},
			},
			want: "",
		},
		{
			name:       "default cookie name",
			cookieName: "",
			headers: map[string][]string{
				"Cookie": {"GATEWAY_SESSION=default123"},
			},
			want: "default123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &mockRequest{headers: tt.headers}
			extractor := NewCookieExtractor(tt.cookieName)
			got := extractor.Extract(req)
			if got != tt.want {
				t.Errorf("Extract() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHeaderExtractor(t *testing.T) {
	tests := []struct {
		name       string
		headerName string
		headers    map[string][]string
		want       string
	}{
		{
			name:       "header present",
			headerName: "X-Session-Id",
			headers: map[string][]string{
				"X-Session-Id": {"session123"},
			},
			want: "session123",
		},
		{
			name:       "header missing",
			headerName: "X-Session-Id",
			headers:    map[string][]string{},
			want:       "",
		},
		{
			name:       "default header name",
			headerName: "",
			headers: map[string][]string{
				"X-Session-Id": {"default456"},
			},
			want: "default456",
		},
		{
			name:       "multiple values takes first",
			headerName: "X-Session-Id",
			headers: map[string][]string{
				"X-Session-Id": {"first", "second"},
			},
			want: "first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &mockRequest{headers: tt.headers}
			extractor := NewHeaderExtractor(tt.headerName)
			got := extractor.Extract(req)
			if got != tt.want {
				t.Errorf("Extract() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQueryExtractor(t *testing.T) {
	tests := []struct {
		name      string
		paramName string
		url       string
		want      string
	}{
		{
			name:      "query param present",
			paramName: "session",
			url:       "/api/test?session=query123",
			want:      "query123",
		},
		{
			name:      "query param missing",
			paramName: "session",
			url:       "/api/test",
			want:      "",
		},
		{
			name:      "multiple params",
			paramName: "session",
			url:       "/api/test?foo=bar&session=xyz&baz=qux",
			want:      "xyz",
		},
		{
			name:      "default param name",
			paramName: "",
			url:       "/api/test?session_id=default789",
			want:      "default789",
		},
		{
			name:      "invalid URL",
			paramName: "session",
			url:       "://invalid",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &mockRequest{url: tt.url}
			extractor := NewQueryExtractor(tt.paramName)
			got := extractor.Extract(req)
			if got != tt.want {
				t.Errorf("Extract() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewExtractor(t *testing.T) {
	tests := []struct {
		name   string
		config *core.SessionAffinityConfig
		req    *mockRequest
		want   string
	}{
		{
			name:   "nil config",
			config: nil,
			req:    &mockRequest{},
			want:   "",
		},
		{
			name: "disabled",
			config: &core.SessionAffinityConfig{
				Enabled: false,
			},
			req:  &mockRequest{},
			want: "",
		},
		{
			name: "cookie source",
			config: &core.SessionAffinityConfig{
				Enabled:    true,
				Source:     core.SessionSourceCookie,
				CookieName: "test",
			},
			req: &mockRequest{
				headers: map[string][]string{
					"Cookie": {"test=cookie123"},
				},
			},
			want: "cookie123",
		},
		{
			name: "header source",
			config: &core.SessionAffinityConfig{
				Enabled:    true,
				Source:     core.SessionSourceHeader,
				HeaderName: "X-Test",
			},
			req: &mockRequest{
				headers: map[string][]string{
					"X-Test": {"header123"},
				},
			},
			want: "header123",
		},
		{
			name: "query source",
			config: &core.SessionAffinityConfig{
				Enabled:    true,
				Source:     core.SessionSourceQuery,
				QueryParam: "sid",
			},
			req: &mockRequest{
				url: "/test?sid=query123",
			},
			want: "query123",
		},
		{
			name: "default to cookie",
			config: &core.SessionAffinityConfig{
				Enabled: true,
				Source:  "unknown",
			},
			req: &mockRequest{
				headers: map[string][]string{
					"Cookie": {"GATEWAY_SESSION=default123"},
				},
			},
			want: "default123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewExtractor(tt.config)
			got := extractor.Extract(tt.req)
			if got != tt.want {
				t.Errorf("Extract() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCookies(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   map[string]string
	}{
		{
			name:   "single cookie",
			header: "name=value",
			want:   map[string]string{"name": "value"},
		},
		{
			name:   "multiple cookies",
			header: "a=1; b=2; c=3",
			want:   map[string]string{"a": "1", "b": "2", "c": "3"},
		},
		{
			name:   "with spaces",
			header: " a = 1 ; b = 2 ",
			want:   map[string]string{"a": "1", "b": "2"},
		},
		{
			name:   "quoted values",
			header: `a="value with spaces"; b="quoted"`,
			want:   map[string]string{"a": "value with spaces", "b": "quoted"},
		},
		{
			name:   "empty parts",
			header: "a=1;;b=2;",
			want:   map[string]string{"a": "1", "b": "2"},
		},
		{
			name:   "no equals sign",
			header: "invalid; a=1",
			want:   map[string]string{"a": "1"},
		},
		{
			name:   "empty header",
			header: "",
			want:   map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCookies(tt.header)
			if len(got) != len(tt.want) {
				t.Errorf("parseCookies() returned %d cookies, want %d", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("parseCookies()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
