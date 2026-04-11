package sbi

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestOAuthMiddlewareLogsRequesterContextOnMissingToken(t *testing.T) {
	core, recorded := observer.New(zap.WarnLevel)
	log := zap.New(core)

	jwks := &JWKSStore{}
	if err := jwks.Set([]byte(validTestJWKS(t))); err != nil {
		t.Fatalf("jwks.Set: %v", err)
	}

	handler := InboundMetadataMiddleware(nil, "h2c")(OAuthMiddleware(true, false, "nudm-ueau", jwks, log, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/nudm-ueau/v1/imsi-001/security-information/generate-auth-data", nil)
	req.RemoteAddr = "10.0.0.2:7777"
	req.Header.Set("User-Agent", "AUSF-ausf-1")
	req.Header.Set("X-Forwarded-For", "192.0.2.5")
	req.Header.Set("3gpp-Sbi-Discovery-target-nf-type", "UDM")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected one log entry, got %d", len(entries))
	}
	ctx := entries[0].ContextMap()
	if got, want := ctx["requester_nf_type"], "AUSF"; got != want {
		t.Fatalf("requester_nf_type: got %#v want %q", got, want)
	}
	if got, want := ctx["requester"], "AUSF-ausf-1"; got != want {
		t.Fatalf("requester: got %#v want %q", got, want)
	}
	if got, want := ctx["requester_remote_addr"], "192.0.2.5"; got != want {
		t.Fatalf("requester_remote_addr: got %#v want %q", got, want)
	}
	if got, want := ctx["via_scp"], true; got != want {
		t.Fatalf("via_scp: got %#v want %v", got, want)
	}
	if got, want := ctx["required_scope"], "nudm-ueau"; got != want {
		t.Fatalf("required_scope: got %#v want %q", got, want)
	}
}

func TestOAuthMiddlewareLogsRequesterContextOnJWKSUnavailable(t *testing.T) {
	core, recorded := observer.New(zap.WarnLevel)
	log := zap.New(core)

	handler := InboundMetadataMiddleware(nil, "h2c")(OAuthMiddleware(true, false, "npcf-smpolicycontrol", &JWKSStore{}, log, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodPost, "http://example.com/npcf-smpolicycontrol/v1/sm-policies", nil)
	req.RemoteAddr = "10.0.0.10:7777"
	req.Header.Set("User-Agent", "SMF-smf-1")
	req.Header.Set("Authorization", "Bearer token")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("expected one log entry, got %d", len(entries))
	}
	ctx := entries[0].ContextMap()
	if got, want := ctx["requester_nf_type"], "SMF"; got != want {
		t.Fatalf("requester_nf_type: got %#v want %q", got, want)
	}
	if got, want := ctx["requester_remote_addr"], "10.0.0.10:7777"; got != want {
		t.Fatalf("requester_remote_addr: got %#v want %q", got, want)
	}
	if got, want := ctx["via_scp"], false; got != want {
		t.Fatalf("via_scp: got %#v want %v", got, want)
	}
	if got, want := ctx["required_scope"], "npcf-smpolicycontrol"; got != want {
		t.Fatalf("required_scope: got %#v want %q", got, want)
	}
}

func TestOAuthMiddlewareAcceptsValidToken(t *testing.T) {
	core, recorded := observer.New(zap.DebugLevel)
	log := zap.New(core)
	jwks, token := testJWKSAndToken(t, map[string]any{
		"scope": "nudm-sdm npcf-smpolicycontrol",
		"exp":   time.Now().Add(5 * time.Minute).Unix(),
	})

	handler := OAuthMiddleware(true, false, "nudm-sdm", jwks, log, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/nudm-sdm/v2/imsi-001/am-data", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(recorded.All()) != 1 {
		t.Fatalf("expected one debug log entry, got %d", len(recorded.All()))
	}
}

func TestOAuthMiddlewareRejectsMissingScope(t *testing.T) {
	jwks, token := testJWKSAndToken(t, map[string]any{
		"scp": []string{"nudm-ueau"},
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})

	handler := OAuthMiddleware(true, false, "nudm-sdm", jwks, zap.NewNop(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/nudm-sdm/v2/imsi-001/am-data", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOAuthMiddlewareRejectsExpiredToken(t *testing.T) {
	jwks, token := testJWKSAndToken(t, map[string]any{
		"scope": "nudm-sdm",
		"exp":   time.Now().Add(-1 * time.Minute).Unix(),
	})

	handler := OAuthMiddleware(true, false, "nudm-sdm", jwks, zap.NewNop(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/nudm-sdm/v2/imsi-001/am-data", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
}

func testJWKSAndToken(t *testing.T, claims map[string]any) (*JWKSStore, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	kid := "test-key-1"
	jwksJSON := marshalTestJWKS(t, kid, &privateKey.PublicKey)
	jwks := &JWKSStore{}
	if err := jwks.Set(jwksJSON); err != nil {
		t.Fatalf("jwks.Set: %v", err)
	}
	return jwks, signRS256Token(t, kid, privateKey, claims)
}

func validTestJWKS(t *testing.T) string {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	return string(marshalTestJWKS(t, "test-key", &privateKey.PublicKey))
}

func marshalTestJWKS(t *testing.T, kid string, pub *rsa.PublicKey) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"keys": []map[string]any{
			{
				"kid": kid,
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e":   encodeInt(pub.E),
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal JWKS: %v", err)
	}
	return raw
}

func signRS256Token(t *testing.T, kid string, privateKey *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	headerRaw, err := json.Marshal(map[string]any{
		"alg": "RS256",
		"typ": "JWT",
		"kid": kid,
	})
	if err != nil {
		t.Fatalf("json.Marshal header: %v", err)
	}
	payloadRaw, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("json.Marshal claims: %v", err)
	}

	header := base64.RawURLEncoding.EncodeToString(headerRaw)
	payload := base64.RawURLEncoding.EncodeToString(payloadRaw)
	signingInput := header + "." + payload
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("rsa.SignPKCS1v15: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func encodeInt(v int) string {
	if v == 0 {
		return ""
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte(v & 0xff)}, b...)
		v >>= 8
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func TestParseJWKSRejectsEmptyKeys(t *testing.T) {
	_, err := parseJWKS([]byte(`{"keys":[]}`))
	if err == nil || !strings.Contains(err.Error(), "no usable keys") {
		t.Fatalf("parseJWKS error: %v", err)
	}
}

func TestValidateJWTRejectsUnknownKid(t *testing.T) {
	jwks, _ := testJWKSAndToken(t, map[string]any{
		"scope": "nudm-sdm",
		"exp":   time.Now().Add(5 * time.Minute).Unix(),
	})
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	token := signRS256Token(t, "other-key", privateKey, map[string]any{
		"scope": "nudm-sdm",
		"exp":   time.Now().Add(5 * time.Minute).Unix(),
	})
	_, err = validateJWT(token, jwks, "nudm-sdm", time.Now())
	if err == nil || !strings.Contains(err.Error(), `no jwk for kid "other-key"`) {
		t.Fatalf("validateJWT error: %v", err)
	}
}

func TestClaimScopesSupportsSpaceSeparatedAndScpArray(t *testing.T) {
	claims := &jwtClaims{Scope: "a b", Scp: []any{"c", "d"}}
	got := claimScopes(claims)
	want := []string{"a", "b", "c", "d"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("claimScopes: got %v want %v", got, want)
	}
}
