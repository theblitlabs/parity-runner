package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

func Init() {
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "15:04:05",
		NoColor:    false,
		FormatLevel: func(i interface{}) string {
			return colorizeLevel(i.(string))
		},
		FormatFieldName: func(i interface{}) string {
			return colorize(fmt.Sprintf("%s=", i.(string)), gray)
		},
		FormatFieldValue: func(i interface{}) string {
			switch v := i.(type) {
			case string:
				return colorize(v, blue)
			case json.Number:
				return colorize(v.String(), blue)
			case error:
				return colorize(v.Error(), red)
			case nil:
				return ""
			default:
				return colorize(fmt.Sprint(v), blue)
			}
		},
		PartsOrder: []string{
			zerolog.TimestampFieldName,
			zerolog.LevelFieldName,
			zerolog.MessageFieldName,
			"component",
		},
		PartsExclude: []string{
			"query",
			"referer",
			"user_agent",
			"remote_addr",
			"duration_human",
			zerolog.CallerFieldName,
		},
		FormatMessage: func(i interface{}) string {
			msg := fmt.Sprint(i)
			msg = strings.Replace(msg, "Request started", "→", 1)
			msg = strings.Replace(msg, "Request completed", "←", 1)
			return fmt.Sprintf("%-40s", msg) // Reduced padding for better alignment
		},
	}

	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log = zerolog.New(output).With().Timestamp().Logger()
	zerolog.DefaultContextLogger = &log
}

// ANSI color codes
const (
	gray  = "\x1b[37m"
	blue  = "\x1b[34m"
	cyan  = "\x1b[36m"
	red   = "\x1b[31m"
	green = "\x1b[32m"
	reset = "\x1b[0m"
)

func colorize(s, color string) string {
	return color + s + reset
}

func colorizeLevel(level string) string {
	switch level {
	case "debug":
		return colorize("DBG", gray)
	case "info":
		return colorize("INF", blue)
	case "warn":
		return colorize("WRN", cyan)
	case "error":
		return colorize("ERR", red)
	case "fatal":
		return colorize("FTL", red)
	default:
		return colorize(level, blue)
	}
}

// Get returns the logger instance
func Get() zerolog.Logger {
	return log
}

// Error logs an error message with a component
func Error(component string, err error, msg string) {
	logger := log.With().Str("component", component).Logger()
	logger.Error().Err(err).Msg(msg)
}

// Info logs an info message with a component
func Info(component string, msg string) {
	logger := log.With().Str("component", component).Logger()
	logger.Info().Msg(msg)
}

// Debug logs a debug message with a component
func Debug(component string, msg string) {
	logger := log.With().Str("component", component).Logger()
	logger.Debug().Msg(msg)
}
