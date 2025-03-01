package telemetry

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP metrics
	requestDurationHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	requestCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_request_count_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	activeRequestsGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_request_active",
			Help: "Number of active HTTP requests",
		},
	)

	// Error metrics
	errorCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "error_total",
			Help: "Total number of errors by type and component",
		},
		[]string{"type", "component"},
	)

	// Webhook metrics
	webhookCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_calls_total",
			Help: "Total number of webhook calls",
		},
		[]string{"status", "type"},
	)

	webhookDurationHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "webhook_duration_seconds",
			Help:    "Duration of webhook calls in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status", "type"},
	)

	activeWebhookGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "webhook_connections_active",
			Help: "Number of active webhook connections",
		},
	)

	// Stake metrics
	stakeOperationsCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "stake_operations_total",
			Help: "Total number of stake operations (stake/unstake)",
		},
		[]string{"operation", "status"},
	)

	// Task metrics
	taskCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tasks_total",
			Help: "Total number of tasks",
		},
		[]string{"status"},
	)

	taskTypeCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tasks_by_type_total",
			Help: "Total number of tasks by type and status",
		},
		[]string{"status", "type"},
	)

	taskTypeDurationHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "task_type_duration_seconds",
			Help:    "Duration of task execution in seconds by type",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status", "type"},
	)

	taskDurationHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "task_duration_seconds",
			Help:    "Duration of task execution in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)

	activeTasksGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tasks_active",
			Help: "Number of currently active tasks",
		},
	)
)

// MetricsHandler returns an http.Handler that serves the metrics endpoint
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// MetricsMiddleware wraps an http.Handler and records metrics about the request
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}

		// Increment active requests
		activeRequestsGauge.Inc()
		defer activeRequestsGauge.Dec()

		next.ServeHTTP(sw, r)

		// Record metrics
		duration := time.Since(start).Seconds()
		labels := prometheus.Labels{
			"method": r.Method,
			"path":   r.URL.Path,
			"status": fmt.Sprintf("%d", sw.status),
		}

		requestDurationHistogram.With(labels).Observe(duration)
		requestCounter.With(labels).Inc()
	})
}

// statusWriter wraps http.ResponseWriter to capture the status code
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// RecordWebhook records webhook metrics
func RecordWebhook(webhookType string, status string, duration time.Duration) {
	webhookCounter.WithLabelValues(status, webhookType).Inc()
	webhookDurationHistogram.WithLabelValues(status, webhookType).Observe(duration.Seconds())
}

// RecordWebhookConnection updates the active webhook connections count
func RecordWebhookConnection(delta float64) {
	activeWebhookGauge.Add(delta)
}

// RecordStakeOperation records a stake operation
func RecordStakeOperation(operation string, status string) {
	stakeOperationsCounter.WithLabelValues(operation, status).Inc()
}

// RecordTask records task metrics
func RecordTask(status string, duration time.Duration) {
	taskCounter.WithLabelValues(status).Inc()
	if duration > 0 {
		taskDurationHistogram.WithLabelValues(status).Observe(duration.Seconds())
	}
}

// RecordTaskWithType records task metrics with type information
func RecordTaskWithType(status string, taskType string, duration time.Duration) {
	taskTypeCounter.WithLabelValues(status, taskType).Inc()
	if duration > 0 {
		taskTypeDurationHistogram.WithLabelValues(status, taskType).Observe(duration.Seconds())
	}
}

// UpdateActiveTasks updates the number of active tasks
func UpdateActiveTasks(count float64) {
	activeTasksGauge.Set(count)
}

// RecordError records an error occurrence by type and component
func RecordError(errorType string, component string) {
	errorCounter.WithLabelValues(errorType, component).Inc()
}
