package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/internal/api/middleware"
	"github.com/theblitlabs/parity-protocol/pkg/wallet"
)

func TestAuthMiddleware(t *testing.T) {
	// Generate a valid token
	validToken, err := wallet.GenerateToken("0x1234567890123456789012345678901234567890")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	tests := []struct {
		name           string
		token          string
		expectedStatus int
	}{
		{
			name:           "valid token",
			token:          validToken,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing token",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid token",
			token:          "invalid.token.here",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler that will be wrapped by the auth middleware
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Create the auth middleware
			handler := middleware.Auth(nextHandler)

			// Create a test request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			// Create a response recorder
			rr := httptest.NewRecorder()

			// Serve the request
			handler.ServeHTTP(rr, req)

			// Check the status code
			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func TestLoggingMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		statusCode int
	}{
		{
			name:       "successful request",
			method:     "GET",
			path:       "/test",
			statusCode: http.StatusOK,
		},
		{
			name:       "not found request",
			method:     "POST",
			path:       "/notfound",
			statusCode: http.StatusNotFound,
		},
		{
			name:       "server error request",
			method:     "PUT",
			path:       "/error",
			statusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler that will return the specified status code
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			// Create the logging middleware
			handler := middleware.Logging(nextHandler)

			// Create a test request
			req := httptest.NewRequest(tt.method, tt.path, nil)

			// Create a response recorder
			rr := httptest.NewRecorder()

			// Serve the request
			handler.ServeHTTP(rr, req)

			// Check the status code
			assert.Equal(t, tt.statusCode, rr.Code)
		})
	}
}

func TestLoggingResponseWriter(t *testing.T) {
	tests := []struct {
		name        string
		writeHeader bool
		statusCode  int
		body        string
	}{
		{
			name:        "write header and body",
			writeHeader: true,
			statusCode:  http.StatusOK,
			body:        "test response",
		},
		{
			name:        "write body only",
			writeHeader: false,
			statusCode:  http.StatusOK,
			body:        "test response without explicit header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test response recorder
			rr := httptest.NewRecorder()

			// Create a test handler that uses the logging middleware
			handler := middleware.Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.writeHeader {
					w.WriteHeader(tt.statusCode)
				}
				w.Write([]byte(tt.body))
			}))

			// Create a test request
			req := httptest.NewRequest("GET", "/test", nil)

			// Serve the request
			handler.ServeHTTP(rr, req)

			// Check the response
			assert.Equal(t, tt.statusCode, rr.Code)
			assert.Equal(t, tt.body, rr.Body.String())
		})
	}
}
