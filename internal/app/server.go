package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gateway/internal/config"
	httpAdapter "gateway/internal/adapter/http"
	wsAdapter "gateway/internal/adapter/websocket"
)

// Server represents the gateway server
type Server struct {
	config      *config.Config
	httpAdapter *httpAdapter.Adapter
	wsAdapter   *wsAdapter.Adapter
	logger      *slog.Logger
}

// NewServer creates a new gateway server
func NewServer(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	builder := NewBuilder(cfg, logger)
	return builder.Build()
}

// Start starts the gateway server
func (s *Server) Start(ctx context.Context) error {
	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	// Start HTTP adapter
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.logger.Info("Starting HTTP server",
			"host", s.config.Gateway.Frontend.HTTP.Host,
			"port", s.config.Gateway.Frontend.HTTP.Port,
		)
		if err := s.httpAdapter.Start(ctx); err != nil {
			errCh <- fmt.Errorf("HTTP server: %w", err)
		}
	}()

	// Start WebSocket adapter if enabled
	if s.wsAdapter != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.logger.Info("Starting WebSocket server",
				"host", s.config.Gateway.Frontend.WebSocket.Host,
				"port", s.config.Gateway.Frontend.WebSocket.Port,
			)
			if err := s.wsAdapter.Start(ctx); err != nil {
				errCh <- fmt.Errorf("WebSocket server: %w", err)
			}
		}()
	}

	// Give servers time to start
	time.Sleep(100 * time.Millisecond)

	// Check for startup errors
	select {
	case err := <-errCh:
		return err
	default:
		s.logger.Info("Gateway started successfully")
		return nil
	}
}

// Stop stops the gateway server
func (s *Server) Stop(ctx context.Context) error {
	var wg sync.WaitGroup
	var errs []error
	errMu := sync.Mutex{}

	// Stop HTTP adapter
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.httpAdapter.Stop(ctx); err != nil {
			errMu.Lock()
			errs = append(errs, fmt.Errorf("stopping HTTP server: %w", err))
			errMu.Unlock()
		}
	}()

	// Stop WebSocket adapter if running
	if s.wsAdapter != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.wsAdapter.Stop(ctx); err != nil {
				errMu.Lock()
				errs = append(errs, fmt.Errorf("stopping WebSocket server: %w", err))
				errMu.Unlock()
			}
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}

	s.logger.Info("Gateway stopped successfully")
	return nil
}