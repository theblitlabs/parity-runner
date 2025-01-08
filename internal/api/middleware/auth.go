package middleware

import (
	"context"
	"net/http"
)

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement proper authentication
		// For now, just set a dummy user ID
		ctx := context.WithValue(r.Context(), "user_id", "test-user-123")
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
