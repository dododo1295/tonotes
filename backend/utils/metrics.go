package utils

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP Metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tonotes",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "tonotes",
			Name:      "http_request_duration_seconds",
			Help:      "Duration of HTTP requests",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	RequestDistribution = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "tonotes",
			Name:      "http_request_distribution_seconds",
			Help:      "Distribution of HTTP request durations",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"path", "status_code"},
	)

	ActiveRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "tonotes",
			Name:      "active_requests",
			Help:      "Current number of active HTTP requests",
		},
	)

	// User Metrics
	ActiveUsers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "tonotes",
			Name:      "active_users",
			Help:      "Number of users active in the last 24 hours",
		},
	)

	UserRegistrations = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "tonotes",
			Name:      "user_registrations_total",
			Help:      "Total number of user registrations",
		},
	)

	UserGrowthRate = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "tonotes",
			Name:      "user_growth_rate",
			Help:      "User growth rate (percentage)",
		},
	)

	// Authentication & Security Metrics
	AuthAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tonotes",
			Name:      "auth_attempts_total",
			Help:      "Total number of authentication attempts",
		},
		[]string{"status", "type"}, // success/failure, login/refresh/2fa
	)

	UnauthorizedAccess = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tonotes",
			Name:      "unauthorized_access_total",
			Help:      "Total number of unauthorized access attempts",
		},
		[]string{"path", "reason"},
	)

	TokenUsage = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tonotes",
			Name:      "token_usage_total",
			Help:      "Token usage statistics",
		},
		[]string{"type", "status"}, // access/refresh, valid/invalid/expired
	)

	// Performance Metrics
	DBOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "tonotes",
			Name:      "db_operation_duration_seconds",
			Help:      "Duration of database operations",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation", "collection"},
	)

	CacheHitRatio = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tonotes",
			Name:      "cache_hit_ratio",
			Help:      "Cache hit ratio percentage",
		},
		[]string{"cache_type"},
	)

	// System Reliability Metrics
	MTTF = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "tonotes",
			Name:      "mttf_hours",
			Help:      "Mean Time To Failure in hours",
		},
	)

	MTTR = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "tonotes",
			Name:      "mttr_minutes",
			Help:      "Mean Time To Recovery in minutes",
		},
	)
	ErrorsByType = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tonotes",
			Name:      "errors_total",
			Help:      "Total number of errors by type",
		},
		[]string{"type", "code"}, // e.g., "validation", "database", "auth"
	)
	DependencyHealth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tonotes",
			Name:      "dependency_health",
			Help:      "Health status of service dependencies",
		},
		[]string{"service", "endpoint"},
	)
	CacheSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tonotes",
			Name:      "cache_size_bytes",
			Help:      "Current size of cache in bytes",
		},
		[]string{"cache_type"},
	)
	CacheEvictions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tonotes",
			Name:      "cache_evictions_total",
			Help:      "Number of cache evictions",
		},
		[]string{"cache_type"},
	)
	// API Health Metrics
	APIHealthcheck = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tonotes",
			Name:      "api_health_status",
			Help:      "API endpoint health status (1 for up, 0 for down)",
		},
		[]string{"endpoint"},
	)

	// Rate Limiting Metrics
	RateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tonotes",
			Name:      "rate_limit_hits_total",
			Help:      "Number of times rate limits were hit",
		},
		[]string{"endpoint"},
	)
	// System Health Metrics
	SystemMemoryUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "tonotes",
			Name:      "system_memory_usage_bytes",
			Help:      "Current system memory usage in bytes",
		},
	)

	SystemCPUUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "tonotes",
			Name:      "system_cpu_usage_percent",
			Help:      "Current CPU usage percentage",
		},
	)

	GoroutineCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "tonotes",
			Name:      "goroutine_count",
			Help:      "Number of running goroutines",
		},
	)

	// Business Metrics
	NotesCreated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tonotes",
			Name:      "notes_created_total",
			Help:      "Total number of notes created",
		},
		[]string{"user_id"},
	)

	TodosCompleted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tonotes",
			Name:      "todos_completed_total",
			Help:      "Total number of todos completed",
		},
		[]string{"user_id"},
	)
)

// MetricsUtil handles metrics collection
func MetricsUtil() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Track active requests
		ActiveRequests.Inc()
		defer ActiveRequests.Dec()

		c.Next()

		// Record metrics after request completion
		duration := time.Since(start).Seconds()
		status := c.Writer.Status()

		HTTPRequestsTotal.WithLabelValues(method, path, fmt.Sprintf("%d", status)).Inc()
		HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
		RequestDistribution.WithLabelValues(path, fmt.Sprintf("%d", status)).Observe(duration)
	}
}

// Metric tracking helper functions
func TrackDBOperation(operation, collection string) *prometheus.Timer {
	return prometheus.NewTimer(DBOperationDuration.WithLabelValues(operation, collection))
}

func TrackAuthAttempt(status, authType string) {
	AuthAttempts.WithLabelValues(status, authType).Inc()
}

func TrackUserActivity(userID string) {
	ActiveUsers.Inc()
}

func TrackRegistration() {
	UserRegistrations.Inc()
}

func TrackUnauthorizedAccess(path, reason string) {
	UnauthorizedAccess.WithLabelValues(path, reason).Inc()
}

func TrackTokenOperation(tokenType, status string) {
	TokenUsage.WithLabelValues(tokenType, status).Inc()
}

func TrackCacheOperation(cacheType string, hit bool) {
	ratio := CacheHitRatio.WithLabelValues(cacheType)
	if hit {
		ratio.Inc()
	} else {
		ratio.Dec()
	}
}

// System reliability tracking
func UpdateMTTF(hours float64) {
	MTTF.Set(hours)
}

func UpdateMTTR(minutes float64) {
	MTTR.Set(minutes)
}

// Business metrics tracking
func TrackNoteCreation(userID string) {
	NotesCreated.WithLabelValues(userID).Inc()
}

func TrackTodoCompletion(userID string) {
	TodosCompleted.WithLabelValues(userID).Inc()
}
func TrackAPIHealth(endpoint string, isHealthy bool) {
	status := 0.0
	if isHealthy {
		status = 1.0
	}
	APIHealthcheck.WithLabelValues(endpoint).Set(status)
}

func TrackRateLimit(endpoint string) {
	RateLimitHits.WithLabelValues(endpoint).Inc()
}

func TrackError(errorType, code string) {
	ErrorsByType.WithLabelValues(errorType, code).Inc()
}

func TrackDependencyHealth(service, endpoint string, isHealthy bool) {
	status := 0.0
	if isHealthy {
		status = 1.0
	}
	DependencyHealth.WithLabelValues(service, endpoint).Set(status)
}

func UpdateCacheSize(cacheType string, sizeBytes float64) {
	CacheSize.WithLabelValues(cacheType).Set(sizeBytes)
}

func TrackCacheEviction(cacheType string) {
	CacheEvictions.WithLabelValues(cacheType).Inc()
}
