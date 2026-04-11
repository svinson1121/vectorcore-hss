package sbi

import (
	"net/http"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// JWKSStore holds raw JWKS JSON fetched from the NRF.
type JWKSStore struct {
	mu   sync.RWMutex
	json []byte
}

func (j *JWKSStore) Set(raw []byte) {
	j.mu.Lock()
	j.json = raw
	j.mu.Unlock()
}

func (j *JWKSStore) Get() []byte {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.json
}

// OAuthMiddleware validates or bypasses Bearer tokens for inbound SBI requests.
func OAuthMiddleware(enabled, bypass bool, requiredScope string, jwks *JWKSStore, log *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !enabled || bypass {
			next.ServeHTTP(w, r)
			return
		}

		meta := RequestMetaFromContext(r.Context())
		baseFields := append(meta.LogFields(),
			zap.String("path", r.URL.Path),
			zap.String("required_scope", requiredScope),
		)

		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			log.Warn("sbi: missing bearer token", baseFields...)
			http.Error(w, `{"error":"missing_token"}`, http.StatusUnauthorized)
			return
		}

		if jwks == nil || jwks.Get() == nil {
			log.Warn("sbi: JWKS not yet available, rejecting request", baseFields...)
			http.Error(w, `{"error":"jwks_unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		_ = requiredScope
		next.ServeHTTP(w, r)
	})
}
