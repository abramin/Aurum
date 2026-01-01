package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aurum/internal/common/config"
	"aurum/internal/common/logging"
	"aurum/internal/common/metrics"
	"aurum/internal/common/types"
	spendingapi "aurum/internal/spending/api"
	"aurum/internal/spending/application"
	"aurum/internal/spending/infrastructure/memory"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Setup structured logging
	logging.Setup(logging.Config{
		Level:  cfg.LogLevel,
		Format: cfg.LogFormat,
	})

	// Generate correlation ID for startup
	startupCtx := logging.WithCorrelationID(context.Background(), types.NewCorrelationID())

	logging.InfoContext(startupCtx, "Starting Aurum finance workspace",
		"port", cfg.Port,
		"environment", cfg.Environment,
		"log_level", cfg.LogLevel,
	)

	// Setup HTTP server
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("GET /health", healthHandler)

	// Ready check endpoint (checks dependencies)
	mux.HandleFunc("GET /ready", readyHandler(cfg))

	// Prometheus metrics endpoint
	mux.Handle("GET /metrics", metrics.Handler())

	// Setup Spending context with in-memory datastore
	// In production, this would use postgres.NewDataStore(pool) instead
	spendingDataStore := memory.NewDataStore()
	spendingService := application.NewSpendingService(spendingDataStore)
	spendingHandler := spendingapi.NewHandler(spendingService)
	spendingHandler.RegisterRoutes(mux)

	logging.InfoContext(startupCtx, "Spending context initialized")

	// Middleware chain: metrics -> correlation -> handler
	handler := metrics.Middleware(correlationMiddleware(mux))

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logging.Info("HTTP server listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logging.Info("Shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logging.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logging.Info("Server stopped")
}

// requestTimeout is the maximum time allowed for processing a single request.
const requestTimeout = 5 * time.Second

// correlationMiddleware adds correlation ID and request timeout to each request.
func correlationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for existing correlation ID in header
		corrID := types.CorrelationID(r.Header.Get("X-Correlation-ID"))
		if corrID.IsEmpty() {
			corrID = types.NewCorrelationID()
		}

		// Add request timeout to prevent runaway requests
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()

		// Add correlation ID to context
		ctx = logging.WithCorrelationID(ctx, corrID)

		// Add tenant ID if present
		if tenantID := r.Header.Get("X-Tenant-ID"); tenantID != "" {
			ctx = logging.WithTenantID(ctx, types.TenantID(tenantID))
		}

		// Set response header
		w.Header().Set("X-Correlation-ID", corrID.String())

		// Log request
		logging.InfoContext(ctx, "HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
		)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// healthHandler returns basic health status.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

// readyHandler checks if all dependencies are available.
func readyHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: Add actual dependency checks (DB, Kafka) in future slices
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"status":      "ready",
			"environment": cfg.Environment,
		})
	}
}
