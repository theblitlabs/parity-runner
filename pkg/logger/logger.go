package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var (
	log zerolog.Logger
	mu  sync.RWMutex
)

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
	// LogLevelDisabled disables all logging
	LogLevelDisabled LogLevel = "disabled"
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
	mu.Lock()
	defer mu.Unlock()

	// If logging is disabled, set global level to disabled and return early
	if cfg.Level == LogLevelDisabled {
		zerolog.SetGlobalLevel(zerolog.Disabled)
		log = zerolog.New(io.Discard).With().Logger()
		zerolog.DefaultContextLogger = &log
		return
	}

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
				name := fmt.Sprint(i)
				// Skip component and trace_id if they're going to have empty values
				if name == "component" || name == "trace_id" {
					return ""
				}
				return colorize(fmt.Sprintf("%s=", name), dim+cyan)
			},
			FormatFieldValue: func(i interface{}) string {
				switch v := i.(type) {
				case string:
					if v == "" {
						return ""
					}
					return colorize(v, blue)
				case json.Number:
					return colorize(v.String(), magenta)
				case error:
					return colorize(v.Error(), red)
				case nil:
					return ""
				default:
					s := fmt.Sprint(v)
					if s == "" {
						return ""
					}
					return colorize(s, blue)
				}
			},
			FormatMessage: func(i interface{}) string {
				msg := fmt.Sprint(i)
				msg = strings.Replace(msg, "Request started", colorize("→", bold+green), 1)
				msg = strings.Replace(msg, "Request completed", colorize("←", bold+green), 1)
				return colorize(msg, bold)
			},
			FormatTimestamp: func(i interface{}) string {
				t := fmt.Sprint(i)
				return colorize(t, dim+gray)
			},
			PartsOrder: []string{
				zerolog.TimestampFieldName,
				zerolog.LevelFieldName,
				zerolog.CallerFieldName,
				zerolog.MessageFieldName,
				"device_id",
			},
			PartsExclude: []string{
				"query",
				"referer",
				"user_agent",
				"remote_addr",
				"duration_human",
				"component",
				"trace_id",
			},
		}
	} else {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: cfg.TimeFormat,
			NoColor:    true,
		}
	}

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
	gray    = "\x1b[37m"
	blue    = "\x1b[34m"
	cyan    = "\x1b[36m"
	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	magenta = "\x1b[35m"
	bold    = "\x1b[1m"
	dim     = "\x1b[2m"
	reset   = "\x1b[0m"
)

func colorize(s, color string) string {
	return color + s + reset
}

func colorizeLevel(level string) string {
	switch level {
	case "debug":
		return colorize("DBG", dim+magenta)
	case "info":
		return colorize("INF", bold+green)
	case "warn":
		return colorize("WRN", bold+yellow)
	case "error":
		return colorize("ERR", bold+red)
	case "fatal":
		return colorize("FTL", bold+red+"\x1b[7m")
	default:
		return colorize(level, blue)
	}
}

func Get() zerolog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return log
}

func WithComponent(component string) zerolog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return log.With().Str("component", component).Logger()
}

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
