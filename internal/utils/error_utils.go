package utils

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

func HandleError(log zerolog.Logger, err error, msg string) {
	if err != nil {
		log.Error().Err(err).Msg(msg)
	}
}

func HandleFatal(log zerolog.Logger, err error, msg string) {
	if err != nil {
		log.Fatal().Err(err).Msg(msg)
	}
}

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

func WrapError(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format+": %w", append(args, err)...)
}
