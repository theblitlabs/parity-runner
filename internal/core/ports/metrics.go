package ports

// MetricsProvider defines the interface for obtaining system metrics
type MetricsProvider interface {
	GetSystemMetrics() (memory int64, cpu float64)
}
