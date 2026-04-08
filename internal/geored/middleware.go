package geored

// middleware.go -- Bearer token authentication for inbound GeoRed requests.

import (
	"net/http"
	"strings"
)

// bearerAuth returns an http.Handler that validates the Authorization header
// against the configured inbound bearer token.
func bearerAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
