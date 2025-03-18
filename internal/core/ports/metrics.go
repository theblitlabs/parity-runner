package ports

type MetricsProvider interface {
	GetSystemMetrics() (memory int64, cpu float64)
}
