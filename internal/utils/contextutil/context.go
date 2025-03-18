package contextutil

import (
	"context"
	"time"
)

const (
	// DefaultTimeout is the default timeout for operations
	DefaultTimeout = 30 * time.Second
	// ShortTimeout is a shorter timeout for quick operations
	ShortTimeout = 5 * time.Second
	// LongTimeout is a longer timeout for more intensive operations
	LongTimeout = 2 * time.Minute
)

// WithTimeout creates a context with the default timeout
func WithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), DefaultTimeout)
}

// WithCustomTimeout creates a context with a custom timeout
func WithCustomTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// WithShortTimeout creates a context with a short timeout
func WithShortTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), ShortTimeout)
}

// WithLongTimeout creates a context with a long timeout
func WithLongTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), LongTimeout)
}
