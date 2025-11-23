package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/shahram/prompt-registry/backend/handlers"
	"github.com/shahram/prompt-registry/backend/store"
)

func main() {
	// Initialize logger
	var logHandler slog.Handler
	logFormat := getEnv("LOG_FORMAT", "text")
	logLevel := getEnv("LOG_LEVEL", "info")

	level := slog.LevelInfo
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	if logFormat == "json" {
		logHandler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	// Configuration from environment variables
	port := getEnv("PORT", "8080")
	dbPath := getEnv("DATABASE_PATH", "./data/prompts.db")
	baseURL := getEnv("BASE_URL", "http://localhost:8080")

	logger.Info("starting prompt registry server",
		"port", port,
		"database", dbPath,
		"base_url", baseURL,
		"log_format", logFormat,
		"log_level", logLevel,
	)

	// Create data directory if needed
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		logger.Error("failed to create data directory", "error", err, "path", dbDir)
		os.Exit(1)
	}

	// Initialize database
	db, err := store.New(dbPath)
	if err != nil {
		logger.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize handlers
	h := handlers.New(db, logger)

	// Mount all routes (including frontend)
	handler := h.Routes()

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server listening", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		logger.Error("server error", "error", err)
		os.Exit(1)
	case sig := <-quit:
		logger.Info("received shutdown signal", "signal", sig.String())
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Info("shutting down server...")
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped gracefully")
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
