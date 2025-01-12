package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

func Init() {
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
		NoColor:    false,
		FormatLevel: func(i interface{}) string {
			return colorizeLevel(i.(string))
		},
		FormatMessage: func(i interface{}) string {
			return colorize(i.(string), cyan)
		},
		FormatFieldName: func(i interface{}) string {
			return colorize(i.(string)+":", gray)
		},
		FormatFieldValue: func(i interface{}) string {
			// Handle different types of field values
			switch v := i.(type) {
			case string:
				return colorize(v, blue)
			case json.Number:
				return colorize(v.String(), blue)
			default:
				return colorize(fmt.Sprint(v), blue)
			}
		},
	}

	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
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
	default:
		return colorize(level, blue)
	}
}

// Get returns the logger instance
func Get() zerolog.Logger {
	return log
}

// Error logs an error message
func Error(err error, msg string) {
	log.Error().Err(err).Msg(msg)
}

// Info logs an info message
func Info(msg string) {
	log.Info().Msg(msg)
}

// Debug logs a debug message
func Debug(msg string) {
	log.Debug().Msg(msg)
}
