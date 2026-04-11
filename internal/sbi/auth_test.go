package sbi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestOAuthMiddlewareLogsRequesterContextOnMissingToken(t *testing.T) {
	core, recorded := observer.New(zap.WarnLevel)
	log := zap.New(core)

	handler := InboundMetadataMiddleware(nil, "h2c")(OAuthMiddleware(true, false, "nudm-ueau", &JWKSStore{json: []byte(`{}`)}, log, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
