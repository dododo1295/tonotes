package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP Metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	HTTPResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "Size of HTTP responses",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "path"},
	)

	ActiveRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_active_requests",
			Help: "Current number of active HTTP requests",
		},
	)

	// Database Metrics
	DBOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_operation_duration_seconds",
			Help:    "Duration of database operations",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation", "collection"},
	)

	// Notes Metrics
	NotesOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notes_operations_total",
			Help: "Total number of note operations",
		},
		[]string{"operation"}, // create, update, delete, archive
	)

	// Authentication Metrics
	AuthAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_attempts_total",
			Help: "Total number of authentication attempts",
		},
		[]string{"status", "type"}, // success/failure, login/refresh/2fa
	)

	// Session Metrics
	ActiveSessions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_sessions_total",
			Help: "Total number of active sessions",
		},
	)

	// Error Metrics
	ErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "errors_total",
			Help: "Total number of errors by type",
		},
		[]string{"type"}, // db, auth, validation, etc.
	)
)

// MetricsMiddleware handles basic HTTP metrics
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		ActiveRequests.Inc()
		defer ActiveRequests.Dec()

		c.Next()

		status := c.Writer.Status()
		duration := time.Since(start).Seconds()
		responseSize := float64(c.Writer.Size())

		HTTPRequestsTotal.WithLabelValues(
			method,
			path,
			string(rune(status)),
		).Inc()

		HTTPRequestDuration.WithLabelValues(
			method,
			path,
		).Observe(duration)

		HTTPResponseSize.WithLabelValues(
			method,
			path,
		).Observe(responseSize)
	}
}

// Helper functions for tracking specific metrics

// TrackDBOperation tracks database operation duration
func TrackDBOperation(operation, collection string) *prometheus.Timer {
	return prometheus.NewTimer(DBOperationDuration.WithLabelValues(operation, collection))
}

// TrackNoteOperation increments the notes operation counter
func TrackNoteOperation(operation string) {
	NotesOperationsTotal.WithLabelValues(operation).Inc()
}

// TrackAuthAttempt records authentication attempts
func TrackAuthAttempt(status, authType string) {
	AuthAttempts.WithLabelValues(status, authType).Inc()
}

// UpdateActiveSessions sets the current number of active sessions
func UpdateActiveSessions(count float64) {
	ActiveSessions.Set(count)
}

// TrackError increments the error counter by type
func TrackError(errorType string) {
	ErrorsTotal.WithLabelValues(errorType).Inc()
}
