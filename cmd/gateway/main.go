package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"time"

	"gateway/internal/app"
	"gateway/internal/config"
)

var (
	configFile = flag.String("config", "configs/gateway.yaml", "config file path")
	logLevel   = flag.String("log-level", "info", "log level")
	hotReload  = flag.Bool("hot-reload", false, "enable configuration hot reload")
)

func main() {
	flag.Parse()

	// Setup logging
	setupLogging(*logLevel)

	// Load config or use default
	cfg, err := config.NewLoader(*configFile).Load()
	if err != nil {
		// If default config file doesn't exist, use built-in defaults
		if *configFile == "configs/gateway.yaml" && os.IsNotExist(err) {
			slog.Info("No config file found, using built-in defaults", "path", *configFile)
			cfg, err = config.LoadDefault()
			if err != nil {
				slog.Error("failed to load default config", "error", err)
				os.Exit(1)
			}
		} else {
			slog.Error("failed to load config", "error", err)
			os.Exit(1)
		}
	}

	// Create server
	server, err := app.NewServer(cfg, slog.Default())
	if err != nil {
		slog.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	// Setup signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Setup hot reload if enabled
	var watcher *config.Watcher
	if *hotReload && *configFile != "" {
		watcherConfig := &config.WatcherConfig{
			OnChange: func(newConfig *config.Config) error {
				slog.Info("Configuration changed, reloading...")
				
				// Create new server with new config
				newServer, err := app.NewServer(newConfig, slog.Default())
				if err != nil {
					return err
				}
				
				// Start new server
				if err := newServer.Start(ctx); err != nil {
					return err
				}
				
				// Stop old server gracefully
				stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer stopCancel()
				if err := server.Stop(stopCtx); err != nil {
					slog.Error("failed to stop old server", "error", err)
				}
				
				// Replace server reference
				server = newServer
				slog.Info("Configuration reloaded successfully")
				return nil
			},
			OnError: func(err error) {
				slog.Error("Configuration reload error", "error", err)
			},
		}
		
		watcher, err = config.NewWatcher(*configFile, watcherConfig, slog.Default())
		if err != nil {
			slog.Error("failed to create config watcher", "error", err)
			os.Exit(1)
		}
		watcher.Start()
		defer watcher.Stop()
		
		slog.Info("Hot reload enabled", "config", *configFile)
	}

	// Start server
	if err := server.Start(ctx); err != nil {
		slog.Error("failed to start server", "error", err)
		os.Exit(1)
	}

	// Wait for shutdown signal
	<-ctx.Done()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Stop(shutdownCtx); err != nil {
		slog.Error("failed to stop server", "error", err)
		os.Exit(1)
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
