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
	vo "aurum/internal/common/value_objects"
	"aurum/internal/spending/api"
	"aurum/internal/spending/application"
	"aurum/internal/spending/infrastructure"
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
	startupCtx := logging.WithCorrelationID(context.Background(), vo.NewCorrelationID())

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

	// Setup spending context
	authRepo := infrastructure.NewMemoryAuthorizationRepository()
	cardAccountRepo := infrastructure.NewMemoryCardAccountRepository()
	idempotencyStore := infrastructure.NewMemoryIdempotencyStore()
	spendingService := application.NewSpendingService(authRepo, cardAccountRepo, idempotencyStore)
	spendingHandler := api.NewHandler(spendingService)
	spendingHandler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      correlationMiddleware(mux),
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

// correlationMiddleware adds correlation ID to each request.
func correlationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for existing correlation ID in header, or generate a new one
		var corrID vo.CorrelationID
		if headerCorrID := r.Header.Get("X-Correlation-ID"); headerCorrID != "" {
			// Try to parse the correlation ID from header
			var err error
			corrID, err = vo.ParseCorrelationID(headerCorrID)
			if err != nil {
				// If parsing fails, generate a new one
				corrID = vo.NewCorrelationID()
			}
		} else {
			corrID = vo.NewCorrelationID()
		}

		// Add to context
		ctx := logging.WithCorrelationID(r.Context(), corrID)

		// Add tenant ID if present and valid
		if tenantIDStr := r.Header.Get("X-Tenant-ID"); tenantIDStr != "" {
			if tenantID, err := vo.ParseTenantID(tenantIDStr); err == nil {
				ctx = logging.WithTenantID(ctx, tenantID)
			}
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
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	}); err != nil {
		logging.Error("Failed to encode health response", "error", err)
	}
}

// readyHandler checks if all dependencies are available.
func readyHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: Add actual dependency checks (DB, Kafka) in future slices
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"status":      "ready",
			"environment": cfg.Environment,
		}); err != nil {
			logging.Error("Failed to encode ready response", "error", err)
		}
	}
}
