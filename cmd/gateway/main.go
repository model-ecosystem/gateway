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

	// Create server
	server, err := app.NewServer(cfg, slog.Default())
	if err != nil {
		slog.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	// Setup signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

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
