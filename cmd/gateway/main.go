package main

import (
	"context"
	"flag"
	"gateway/internal/backend"
	"gateway/internal/config"
	"gateway/internal/core"
	frontendhttp "gateway/internal/frontend/http"
	"gateway/internal/middleware"
	"gateway/internal/registry/static"
	"gateway/internal/router"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

var (
	configFile = flag.String("config", "configs/gateway.yaml", "config file path")
	logLevel   = flag.String("log-level", "info", "log level")
)

func main() {
	flag.Parse()
	
	// Setup logging
	setupLogging(*logLevel)
	
	// Load config
	cfg, err := config.NewLoader(*configFile).Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	
	// Create registry
	registry, err := static.NewRegistry(cfg.Gateway.Registry.Static)
	if err != nil {
		slog.Error("failed to create registry", "error", err)
		os.Exit(1)
	}
	
	// Create router
	r := router.NewRouter(registry)
	for _, rule := range cfg.Gateway.Router.Rules {
		if err := r.AddRule(rule.ToRouteRule()); err != nil {
			slog.Error("failed to add route", "error", err)
			os.Exit(1)
		}
	}
	
	// Create optimized HTTP client
	httpClient := createHTTPClient(cfg.Gateway.Backend.HTTP)
	
	// Create backend connector with optimized client
	// Use response header timeout as default timeout, fallback to 30s
	defaultTimeout := time.Duration(cfg.Gateway.Backend.HTTP.ResponseHeaderTimeout) * time.Second
	if defaultTimeout == 0 {
		defaultTimeout = 30 * time.Second
	}
	connector := backend.NewHTTPConnector(httpClient, defaultTimeout)
	
	// Create handler
	handler := createHandler(r, connector)
	
	// Apply middleware
	handler = middleware.Chain(
		middleware.Recovery(),
		middleware.Logging(slog.Default()),
	)(handler)
	
	// Create HTTP adapter
	httpConfig := frontendhttp.Config{
		Host:         cfg.Gateway.Frontend.HTTP.Host,
		Port:         cfg.Gateway.Frontend.HTTP.Port,
		ReadTimeout:  time.Duration(cfg.Gateway.Frontend.HTTP.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Gateway.Frontend.HTTP.WriteTimeout) * time.Second,
	}
	
	// Add TLS config if enabled
	if cfg.Gateway.Frontend.HTTP.TLS != nil && cfg.Gateway.Frontend.HTTP.TLS.Enabled {
		httpConfig.TLS = &frontendhttp.TLSConfig{
			Enabled:    true,
			CertFile:   cfg.Gateway.Frontend.HTTP.TLS.CertFile,
			KeyFile:    cfg.Gateway.Frontend.HTTP.TLS.KeyFile,
			MinVersion: cfg.Gateway.Frontend.HTTP.TLS.MinVersion,
		}
	}
	
	adapter := frontendhttp.New(httpConfig, handler)
	
	// Start server
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	
	if err := adapter.Start(ctx); err != nil {
		slog.Error("failed to start server", "error", err)
		os.Exit(1)
	}
	
	// Wait for shutdown
	<-ctx.Done()
	
	// Shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	if err := adapter.Stop(shutdownCtx); err != nil {
		slog.Error("failed to stop server", "error", err)
	}
}

var logLevels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func setupLogging(level string) {
	lvl := logLevels[strings.ToLower(level)]
	if lvl == 0 {
		lvl = slog.LevelInfo
	}
	
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
	})))
}

func createHTTPClient(cfg config.HTTPBackend) *http.Client {
	// Create dialer with keep-alive settings
	dialer := &net.Dialer{
		Timeout: time.Duration(cfg.DialTimeout) * time.Second,
	}
	
	if cfg.KeepAlive {
		dialer.KeepAlive = time.Duration(cfg.KeepAliveTimeout) * time.Second
	} else {
		dialer.KeepAlive = -1 // Disable keep-alive
	}
	
	// Create transport with connection pooling
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       time.Duration(cfg.IdleConnTimeout) * time.Second,
		ResponseHeaderTimeout: time.Duration(cfg.ResponseHeaderTimeout) * time.Second,
		ForceAttemptHTTP2:     true,
		DisableCompression:    false,
	}
	
	return &http.Client{
		Transport: transport,
		// No timeout here, we use context timeout per request
	}
}

func createHandler(r core.Router, connector backend.Connector) core.Handler {
	return func(ctx context.Context, req core.Request) (core.Response, error) {
		// Route request
		route, err := r.Route(ctx, req)
		if err != nil {
			return nil, err
		}
		
		// Forward request using connector
		return connector.Forward(ctx, req, route)
	}
}