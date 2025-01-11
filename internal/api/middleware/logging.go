package middleware

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
)

// responseWriter is a custom ResponseWriter that captures the status code
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	size        int64
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	size, err := rw.ResponseWriter.Write(b)
	rw.size += int64(size)
	return size, err
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log := logger.Get()
		rw := newResponseWriter(w)

		// Extract request ID or generate new one
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Log incoming request
		log.Info().
			Str("request_id", requestID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("query", r.URL.RawQuery).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Str("referer", r.Referer()).
			Msg("Request started")

		// Add request ID to response headers
		w.Header().Set("X-Request-ID", requestID)

		defer func() {
			duration := time.Since(start)

			// Determine log level based on status code
			logEvent := log.Info()
			if rw.status >= 400 {
				logEvent = log.Error()
			} else if rw.status >= 300 {
				logEvent = log.Warn()
			} else if rw.status >= 200 {
				logEvent = log.Info()
			}

			// Log request completion
			logEvent.
				Str("request_id", requestID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", rw.status).
				Int64("size", rw.size).
				Str("duration", duration.String()).
				Float64("duration_ms", float64(duration.Microseconds())/1000).
				Msg("Request completed")
		}()

		next.ServeHTTP(rw, r)
	})
}

// generateRequestID creates a unique request ID
func generateRequestID() string {
	return uuid.New().String()
}
