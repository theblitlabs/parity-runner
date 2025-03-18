package contextutil

import (
	"context"
	"time"
)

const (
	DefaultTimeout = 30 * time.Second

	ShortTimeout = 5 * time.Second
	LongTimeout  = 2 * time.Minute
)

func WithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), DefaultTimeout)
}

func WithCustomTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

func WithShortTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), ShortTimeout)
}

func WithLongTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), LongTimeout)
}
