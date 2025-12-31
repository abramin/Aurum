package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"

	types "aurum/internal/common/value_objects"
)

// Context keys for logging attributes
type contextKey string

const (
	correlationIDKey contextKey = "correlation_id"
	tenantIDKey      contextKey = "tenant_id"
)

// Config holds logging configuration.
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json, text
}

// Setup initializes the global logger with the given configuration.
func Setup(cfg Config) {
	level := parseLevel(cfg.Level)

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if strings.ToLower(cfg.Format) == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// parseLevel converts a string level to slog.Level.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithCorrelationID adds a correlation ID to the context.
func WithCorrelationID(ctx context.Context, id types.CorrelationID) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// WithTenantID adds a tenant ID to the context.
func WithTenantID(ctx context.Context, id types.TenantID) context.Context {
	return context.WithValue(ctx, tenantIDKey, id)
}

// CorrelationIDFromContext extracts the correlation ID from context.
func CorrelationIDFromContext(ctx context.Context) types.CorrelationID {
	if id, ok := ctx.Value(correlationIDKey).(types.CorrelationID); ok {
		return id
	}
	return types.CorrelationID{}
}

// TenantIDFromContext extracts the tenant ID from context.
func TenantIDFromContext(ctx context.Context) types.TenantID {
	if id, ok := ctx.Value(tenantIDKey).(types.TenantID); ok {
		return id
	}
	return types.TenantID{}
}

// FromContext returns a logger with context attributes (correlation_id, tenant_id).
func FromContext(ctx context.Context) *slog.Logger {
	logger := slog.Default()

	if corrID := CorrelationIDFromContext(ctx); !corrID.IsEmpty() {
		logger = logger.With("correlation_id", corrID.String())
	}

	if tenantID := TenantIDFromContext(ctx); !tenantID.IsEmpty() {
		logger = logger.With("tenant_id", tenantID.String())
	}

	return logger
}

// With returns a logger with the given attributes.
func With(args ...any) *slog.Logger {
	return slog.Default().With(args...)
}

// Info logs at info level.
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Debug logs at debug level.
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// Warn logs at warn level.
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// Error logs at error level.
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}

// InfoContext logs at info level with context attributes.
func InfoContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Info(msg, args...)
}

// DebugContext logs at debug level with context attributes.
func DebugContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Debug(msg, args...)
}

// WarnContext logs at warn level with context attributes.
func WarnContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Warn(msg, args...)
}

// ErrorContext logs at error level with context attributes.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Error(msg, args...)
}
