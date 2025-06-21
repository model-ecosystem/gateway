package cors

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORS_DefaultConfig(t *testing.T) {
	config := DefaultConfig()
	cors := New(config)
	
	// Test simple request
	handler := cors.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	// Check response headers
	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin header, got: %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
	
	if w.Header().Get("Vary") != "Origin" {
		t.Errorf("Expected Vary: Origin header, got: %s", w.Header().Get("Vary"))
	}
}

func TestCORS_Preflight(t *testing.T) {
	config := Config{
		AllowedOrigins:   []string{"https://example.com", "https://test.com"},
		AllowedMethods:   []string{"GET", "POST", "PUT"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           3600,
	}
	cors := New(config)
	
	handler := cors.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for preflight")
	}))
	
	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	// Check preflight response
	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got: %d", w.Code)
	}
	
	headers := w.Header()
	if headers.Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Wrong Allow-Origin: %s", headers.Get("Access-Control-Allow-Origin"))
	}
	
	if headers.Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("Expected Allow-Credentials: true")
	}
	
	allowedMethods := headers.Get("Access-Control-Allow-Methods")
	if !strings.Contains(allowedMethods, "POST") {
		t.Errorf("POST not in allowed methods: %s", allowedMethods)
	}
	
	if headers.Get("Access-Control-Allow-Headers") != "Content-Type, Authorization" {
		t.Errorf("Wrong Allow-Headers: %s", headers.Get("Access-Control-Allow-Headers"))
	}
	
	if headers.Get("Access-Control-Max-Age") != "3600" {
		t.Errorf("Wrong Max-Age: %s", headers.Get("Access-Control-Max-Age"))
	}
}

func TestCORS_PreflightPassthrough(t *testing.T) {
	config := Config{
		AllowedOrigins:     []string{"*"},
		OptionsPassthrough: true,
	}
	cors := New(config)
	
	handlerCalled := false
	handler := cors.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	
	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	if !handlerCalled {
		t.Error("Handler should be called with OptionsPassthrough")
	}
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	config := Config{
		AllowedOrigins: []string{"https://allowed.com"},
	}
	cors := New(config)
	
	handler := cors.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://notallowed.com")
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	// Should not have CORS headers for disallowed origin
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Should not set Allow-Origin for disallowed origin")
	}
}

func TestCORS_ExposedHeaders(t *testing.T) {
	config := Config{
		AllowedOrigins: []string{"*"},
		ExposedHeaders: []string{"X-Request-ID", "X-Rate-Limit"},
	}
	cors := New(config)
	
	handler := cors.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	exposed := w.Header().Get("Access-Control-Expose-Headers")
	if exposed != "X-Request-ID, X-Rate-Limit" {
		t.Errorf("Wrong exposed headers: %s", exposed)
	}
}

func TestCORS_WildcardHeaders(t *testing.T) {
	config := Config{
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"*"},
	}
	cors := New(config)
	
	handler := cors.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	
	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "X-Custom-Header, Another-Header")
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	// Should allow any headers with wildcard
	if w.Header().Get("Access-Control-Allow-Headers") != "X-Custom-Header, Another-Header" {
		t.Errorf("Should allow requested headers with wildcard config")
	}
}

func TestCORS_CaseInsensitive(t *testing.T) {
	config := Config{
		AllowedOrigins: []string{"https://EXAMPLE.com"},
		AllowedMethods: []string{"get", "POST"},
		AllowedHeaders: []string{"content-type"},
	}
	cors := New(config)
	
	handler := cors.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	
	// Test with different case origin
	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Error("Origin comparison should be case-insensitive")
	}
	
	if w.Header().Get("Access-Control-Allow-Headers") != "Content-Type" {
		t.Error("Header comparison should be case-insensitive")
	}
}

func TestCORS_NoOrigin(t *testing.T) {
	config := DefaultConfig()
	cors := New(config)
	
	handler := cors.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	// Request without Origin header
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	// Should not set CORS headers without origin
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Should not set CORS headers without Origin")
	}
}