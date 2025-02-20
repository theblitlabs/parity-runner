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

// LogLevel represents the logging level
type LogLevel string

const (
	// LogLevelDebug enables debug level logging
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo enables info level logging
	LogLevelInfo LogLevel = "info"
	// LogLevelWarn enables warn level logging
	LogLevelWarn LogLevel = "warn"
	// LogLevelError enables error level logging
	LogLevelError LogLevel = "error"
)

// Config represents logger configuration
type Config struct {
	// Level sets the logging level (debug, info, warn, error)
	Level LogLevel
	// Pretty enables pretty console output (for development)
	Pretty bool
	// TimeFormat sets the time format string
	TimeFormat string
}

// DefaultConfig returns the default logger configuration
func DefaultConfig() Config {
	return Config{
		Level:      LogLevelInfo,
		Pretty:     false,
		TimeFormat: time.RFC3339,
	}
}

func Init(cfg Config) {
	var output zerolog.ConsoleWriter
	if cfg.Pretty {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: cfg.TimeFormat,
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
				"trace_id",
			},
			PartsExclude: []string{
				"query",
				"referer",
				"user_agent",
				"remote_addr",
				"duration_human",
			},
			FormatMessage: func(i interface{}) string {
				msg := fmt.Sprint(i)
				msg = strings.Replace(msg, "Request started", "→", 1)
				msg = strings.Replace(msg, "Request completed", "←", 1)
				return msg
			},
		}
	} else {
		// Production JSON logging
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: cfg.TimeFormat,
			NoColor:    true,
		}
	}

	// Set global logging level
	switch cfg.Level {
	case LogLevelDebug:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case LogLevelInfo:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case LogLevelWarn:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case LogLevelError:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	zerolog.TimeFieldFormat = cfg.TimeFormat
	log = zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Logger()
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

// WithComponent returns a logger with the component field set
func WithComponent(component string) zerolog.Logger {
	return log.With().Str("component", component).Logger()
}

// WithTraceID returns a logger with the trace_id field set
func WithTraceID(traceID string) zerolog.Logger {
	return log.With().Str("trace_id", traceID).Logger()
}

// Error logs an error message with a component and optional fields
func Error(component string, err error, msg string, fields ...map[string]interface{}) {
	logger := WithComponent(component)
	event := logger.Error().Err(err)
	if len(fields) > 0 {
		for key, value := range fields[0] {
			event = event.Interface(key, value)
		}
	}
	event.Msg(msg)
}

// Info logs an info message with a component and optional fields
func Info(component string, msg string, fields ...map[string]interface{}) {
	logger := WithComponent(component)
	event := logger.Info()
	if len(fields) > 0 {
		for key, value := range fields[0] {
			event = event.Interface(key, value)
		}
	}
	event.Msg(msg)
}

// Debug logs a debug message with a component and optional fields
func Debug(component string, msg string, fields ...map[string]interface{}) {
	logger := WithComponent(component)
	event := logger.Debug()
	if len(fields) > 0 {
		for key, value := range fields[0] {
			event = event.Interface(key, value)
		}
	}
	event.Msg(msg)
}

// Warn logs a warning message with a component and optional fields
func Warn(component string, msg string, fields ...map[string]interface{}) {
	logger := WithComponent(component)
	event := logger.Warn()
	if len(fields) > 0 {
		for key, value := range fields[0] {
			event = event.Interface(key, value)
		}
	}
	event.Msg(msg)
}
