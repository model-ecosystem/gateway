package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	httpAdapter "gateway/internal/adapter/http"
	wsAdapter "gateway/internal/adapter/websocket"
	"gateway/internal/config"
)

// Server represents the gateway server
type Server struct {
	config        *config.Config
	httpAdapter   *httpAdapter.Adapter
	wsAdapter     *wsAdapter.Adapter
	metricsServer *http.Server
	router        interface{ Close() error } // Router with Close method
	registry      interface{ Close() error } // Registry with Close method
	logger        *slog.Logger
}

// NewServer creates a new gateway server
func NewServer(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	builder := NewBuilder(cfg, logger)
	return builder.Build()
}

// Start starts the gateway server
//
// This method is non-blocking and returns after all adapters have been successfully started.
// The server will continue running in the background until Stop() is called or the context is canceled.
//
// Usage example:
//
//	server, err := NewServer(config, logger)
//	if err != nil {
//	    return err
//	}
//
//	// Start server
//	ctx := context.Background()
//	if err := server.Start(ctx); err != nil {
//	    return err
//	}
//
//	// Server is now running in background
//	// Wait for interrupt signal
//	sigCh := make(chan os.Signal, 1)
//	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
//	<-sigCh
//
//	// Gracefully stop
//	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	if err := server.Stop(stopCtx); err != nil {
//	    log.Printf("Error stopping server: %v", err)
//	}
func (s *Server) Start(ctx context.Context) error {
	// Create a startup context that can be canceled if any adapter fails
	// This context is ONLY for the startup phase. The main ctx is for the server lifetime.
	startupCtx, cancelStartup := context.WithCancel(ctx)
	// DO NOT defer cancelStartup() here - it should only be called on error paths

	// Channel to collect startup errors
	errCh := make(chan error, 2)
	// Channel to signal successful starts
	startedCh := make(chan struct{}, 2)
	expectedStarts := 1 // HTTP adapter always starts

	// Start HTTP adapter
	go func() {
		s.logger.Info("Starting HTTP server",
			"host", s.config.Gateway.Frontend.HTTP.Host,
			"port", s.config.Gateway.Frontend.HTTP.Port,
		)
		if err := s.httpAdapter.Start(startupCtx); err != nil {
			errCh <- fmt.Errorf("HTTP server: %w", err)
		} else {
			startedCh <- struct{}{}
		}
	}()

	// Start WebSocket adapter if enabled
	if s.wsAdapter != nil {
		expectedStarts++
		go func() {
			s.logger.Info("Starting WebSocket server",
				"host", s.config.Gateway.Frontend.WebSocket.Host,
				"port", s.config.Gateway.Frontend.WebSocket.Port,
			)
			if err := s.wsAdapter.Start(startupCtx); err != nil {
				errCh <- fmt.Errorf("WebSocket server: %w", err)
			} else {
				startedCh <- struct{}{}
			}
		}()
	}

	// Start metrics server if enabled on separate port
	if s.metricsServer != nil {
		expectedStarts++
		go func() {
			s.logger.Info("Starting metrics server",
				"address", s.metricsServer.Addr,
			)
			// Signal that we're starting (since ListenAndServe blocks)
			startedCh <- struct{}{}
			// ListenAndServe blocks until shutdown
			if err := s.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				// Only report real errors, not expected server closed
				select {
				case errCh <- fmt.Errorf("metrics server: %w", err):
				case <-startupCtx.Done():
					// Context canceled, don't send error
				}
			}
		}()
	}

	// Wait for all adapters to start or fail
	started := 0
	startupTimeout := time.NewTimer(5 * time.Second)
	defer startupTimeout.Stop()

	for started < expectedStarts {
		select {
		case err := <-errCh:
			// One of the adapters failed to start
			// Cancel the startup context to stop any still-starting adapters
			cancelStartup()

			// Stop any adapters that may have started successfully
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer stopCancel()
			if stopErr := s.Stop(stopCtx); stopErr != nil {
				s.logger.Error("Failed to stop server after startup error", "error", stopErr)
			}

			return err
		case <-startedCh:
			started++
		case <-startupTimeout.C:
			// Timeout waiting for adapters to start
			cancelStartup()

			// Stop any adapters that may have started
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer stopCancel()
			if stopErr := s.Stop(stopCtx); stopErr != nil {
				s.logger.Error("Failed to stop server after timeout", "error", stopErr)
			}

			return fmt.Errorf("timeout waiting for adapters to start")
		case <-ctx.Done():
			cancelStartup()
			return ctx.Err()
		}
	}

	// All adapters started successfully
	// Cancel the startup context as it's no longer needed
	cancelStartup()
	
	s.logger.Info("Gateway started successfully")
	return nil
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

	// Stop metrics server if running
	if s.metricsServer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.metricsServer.Shutdown(ctx); err != nil {
				errMu.Lock()
				errs = append(errs, fmt.Errorf("stopping metrics server: %w", err))
				errMu.Unlock()
			}
		}()
	}

	// Close router if it has a Close method
	if s.router != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.router.Close(); err != nil {
				errMu.Lock()
				errs = append(errs, fmt.Errorf("closing router: %w", err))
				errMu.Unlock()
			}
		}()
	}

	// Close registry if it has a Close method
	if s.registry != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.registry.Close(); err != nil {
				errMu.Lock()
				errs = append(errs, fmt.Errorf("closing registry: %w", err))
				errMu.Unlock()
			}
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		// Wrap at least one error for better error chain
		if len(errs) == 1 {
			return errs[0]
		}
		return fmt.Errorf("multiple errors during shutdown: %v", errs)
	}

	s.logger.Info("Gateway stopped successfully")
	return nil
}
