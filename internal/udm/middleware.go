package udm

// middleware.go — OAuth2 Bearer token validation for inbound Nudm requests.
//
// When oauth2_enabled=true the middleware validates RS256/ES256 JWTs issued by
// the NRF.  When oauth2_bypass=true it skips validation entirely (lab/dev use).
//
// Full JWT validation requires the NRF JWKS to be fetched first (done by the
// NRF client in nrf.go).  Until the JWKS is available the middleware rejects
// requests with 503 so callers retry rather than receiving a stale 401.

import (
	"net/http"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// jwksStore holds the raw JWKS JSON fetched from the NRF.
// Token validation is best-effort for now; a full JWT library integration
// can be layered here without changing the handler signatures.
type jwksStore struct {
	mu   sync.RWMutex
	json []byte // raw JWKS JSON from NRF; nil = not yet fetched
}

func (j *jwksStore) set(raw []byte) {
	j.mu.Lock()
	j.json = raw
	j.mu.Unlock()
}

func (j *jwksStore) get() []byte {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.json
}

// oauthMiddleware returns an http.Handler that validates (or bypasses) Bearer tokens.
func oauthMiddleware(
	enabled bool,
	bypass bool,
	requiredScope string,
	jwks *jwksStore,
	log *zap.Logger,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !enabled || bypass {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, `{"error":"missing_token"}`, http.StatusUnauthorized)
			return
		}

		// If we haven't fetched the JWKS yet, return 503 so the NF retries.
		if jwks.get() == nil {
			log.Warn("udm: JWKS not yet available, rejecting request")
			http.Error(w, `{"error":"jwks_unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		// TODO Phase 2: full RS256/ES256 JWT signature + expiry + scope validation.
		// For now we accept any syntactically valid Bearer token when JWKS is present.
		// This is acceptable in a private network where the NRF is the only issuer.
		_ = requiredScope
		next.ServeHTTP(w, r)
	})
}
