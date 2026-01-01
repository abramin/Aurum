package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTP metrics
var (
	// HTTPRequestDuration tracks request latency by method, path, and status.
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestsTotal counts HTTP requests by method, path, and status.
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestTimeout counts requests that hit the timeout threshold by path.
	HTTPRequestTimeout = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_request_timeout_total",
			Help: "Total number of HTTP request timeouts",
		},
		[]string{"path"},
	)
)

// Database metrics
var (
	// DBTransactionDuration tracks transaction duration by operation label.
	DBTransactionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_transaction_duration_seconds",
			Help:    "Duration of database transactions in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
		},
		[]string{"operation"},
	)

	// DBOptimisticLockConflicts counts optimistic lock conflicts by repository.
	DBOptimisticLockConflicts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_optimistic_lock_conflicts_total",
			Help: "Total number of optimistic lock conflicts",
		},
		[]string{"repository"},
	)

	// DBPoolConnectionsInUse gauges the number of in-use database connections.
	DBPoolConnectionsInUse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_pool_connections_in_use",
			Help: "Number of database connections currently in use",
		},
	)

	// DBPoolConnectionsIdle gauges the number of idle database connections.
	DBPoolConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_pool_connections_idle",
			Help: "Number of idle database connections",
		},
	)
)

// Outbox metrics
var (
	// OutboxPendingEvents gauges the number of unpublished outbox events.
	OutboxPendingEvents = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "outbox_pending_events",
			Help: "Number of unpublished events in outbox",
		},
	)

	// OutboxOldestUnpublishedAge gauges the age in seconds of the oldest unpublished event.
	OutboxOldestUnpublishedAge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "outbox_oldest_unpublished_age_seconds",
			Help: "Age of the oldest unpublished outbox event in seconds",
		},
	)
)

// Business metrics
var (
	// IdempotencyCacheHits counts cache hits for idempotency lookups.
	IdempotencyCacheHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "idempotency_cache_hits_total",
			Help: "Total number of idempotency cache hits",
		},
	)

	// AuthorizationsCreated counts created authorizations by status.
	AuthorizationsCreated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "authorization_created_total",
			Help: "Total number of authorizations created",
		},
		[]string{"status"},
	)
)

// Handler returns the Prometheus metrics HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Middleware returns an HTTP middleware that records request metrics.
// Side effects: records Prometheus metrics and reads the current time.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip metrics endpoint itself
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.statusCode)
		path := normalizePath(r.URL.Path)

		HTTPRequestDuration.WithLabelValues(r.Method, path, status).Observe(duration)
		HTTPRequestsTotal.WithLabelValues(r.Method, path, status).Inc()

		// Check for timeout (context canceled with 5s timeout typically means timeout)
		if r.Context().Err() != nil && duration >= 4.9 {
			HTTPRequestTimeout.WithLabelValues(path).Inc()
		}
	})
}

// normalizePath normalizes URL paths to avoid cardinality explosion.
// Replaces UUIDs and numeric IDs with placeholders.
func normalizePath(path string) string {
	// Common patterns: /authorizations/{uuid}/capture
	// This is a simple implementation; enhance as needed
	switch {
	case len(path) > 16 && path[:16] == "/authorizations/":
		if len(path) > 52 && path[52:] == "/capture" {
			return "/authorizations/{id}/capture"
		}
		if len(path) == 52 { // /authorizations/ + 36 char UUID
			return "/authorizations/{id}"
		}
		return "/authorizations/{id}"
	case len(path) > 15 && path[:15] == "/card-accounts/":
		return "/card-accounts/{id}"
	default:
		return path
	}
}

// RecordOptimisticLockConflict increments the optimistic lock conflict counter.
// Side effects: records a Prometheus metric.
func RecordOptimisticLockConflict(repository string) {
	DBOptimisticLockConflicts.WithLabelValues(repository).Inc()
}

// RecordTransactionDuration records a transaction duration.
// Side effects: records a Prometheus metric.
func RecordTransactionDuration(operation string, duration time.Duration) {
	DBTransactionDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordIdempotencyCacheHit increments the cache hit counter.
// Side effects: records a Prometheus metric.
func RecordIdempotencyCacheHit() {
	IdempotencyCacheHits.Inc()
}

// RecordAuthorizationCreated increments the authorization counter.
// Side effects: records a Prometheus metric.
func RecordAuthorizationCreated(status string) {
	AuthorizationsCreated.WithLabelValues(status).Inc()
}
