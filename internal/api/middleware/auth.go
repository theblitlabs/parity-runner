package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/virajbhartiya/parity-protocol/pkg/wallet"
)

type contextKey string

const userAddressKey contextKey = "user_address"

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Check if the header starts with "Bearer "
		bearerToken := strings.Split(authHeader, " ")
		if len(bearerToken) != 2 || strings.ToLower(bearerToken[0]) != "bearer" {
			http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		// Verify the JWT token
		claims, err := wallet.VerifyToken(bearerToken[1])
		if err != nil {
			http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}

		// Updated context value setting with custom key type
		ctx := context.WithValue(r.Context(), userAddressKey, claims.Address)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
