package errorutil

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

// HandleError is a utility function for handling errors with logging
func HandleError(log zerolog.Logger, err error, msg string) {
	if err != nil {
		log.Error().Err(err).Msg(msg)
	}
}

// HandleFatal logs a fatal error and panics
func HandleFatal(log zerolog.Logger, err error, msg string) {
	if err != nil {
		log.Fatal().Err(err).Msg(msg)
	}
}

// HandleContextError handles errors based on context status
func HandleContextError(log zerolog.Logger, ctx context.Context, err error, timeoutMsg, errorMsg string) {
	if err != nil {
		select {
		case <-ctx.Done():
			log.Error().Err(ctx.Err()).Msg(timeoutMsg)
		default:
			log.Error().Err(err).Msg(errorMsg)
		}
	}
}

// HandleContextFatal handles fatal errors based on context status
func HandleContextFatal(log zerolog.Logger, ctx context.Context, err error, timeoutMsg, errorMsg string) {
	if err != nil {
		select {
		case <-ctx.Done():
			log.Fatal().Err(ctx.Err()).Msg(timeoutMsg)
		default:
			log.Fatal().Err(err).Msg(errorMsg)
		}
	}
}

// WrapError wraps an error with additional context
func WrapError(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format+": %w", append(args, err)...)
}
