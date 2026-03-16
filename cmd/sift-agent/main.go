package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sift-works/agent/internal/agent"
	"github.com/sift-works/agent/internal/api"
	"github.com/sift-works/agent/internal/config"
	"github.com/sift-works/agent/internal/db"
	"github.com/sift-works/agent/internal/executor"
)

func main() {
	cfg, err := config.Parse()
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}

	logger := setupLogger(cfg.LogLevel)

	logger.Info("starting sift-agent", "version", config.Version)

	// Open database connection
	database, driver, err := db.Open(cfg.DSN)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// Verify database is reachable
	if err := database.Ping(); err != nil {
		logger.Error("database is not reachable", "error", err)
		os.Exit(1)
	}
	logger.Info("database connection established")

	// Create components
	client := api.NewClient(cfg.APIURL, cfg.Token, logger)
	exec := executor.New(database, driver)
	a := agent.New(cfg, client, exec, database, logger)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := a.Run(ctx); err != nil {
		logger.Error("agent error", "error", err)
		os.Exit(1)
	}

	logger.Info("sift-agent stopped")
}

func setupLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})

	return slog.New(handler)
}
