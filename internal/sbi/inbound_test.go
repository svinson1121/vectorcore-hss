package sbi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/svinson1121/vectorcore-hss/internal/peertracker"
)

func TestParseRequestMetaDirect(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/health", nil)
	req.RemoteAddr = "10.0.0.10:7777"
	req.Header.Set("User-Agent", "SMF/smf-1")

	meta := ParseRequestMeta(req)
	if meta.ViaSCP {
		t.Fatal("expected direct request")
	}
	if meta.RequesterNFType != "SMF" {
		t.Fatalf("RequesterNFType: got %q", meta.RequesterNFType)
	}
	if got, want := meta.DisplayName(), "SMF/smf-1"; got != want {
		t.Fatalf("DisplayName: got %q want %q", got, want)
	}
	if got, want := meta.DisplayRemoteAddr(), "10.0.0.10:7777"; got != want {
		t.Fatalf("DisplayRemoteAddr: got %q want %q", got, want)
	}
	if got, want := meta.DisplayTransport("h2c"), "h2c"; got != want {
		t.Fatalf("DisplayTransport: got %q want %q", got, want)
	}
}

func TestParseRequestMetaSCPForwarded(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/health", nil)
	req.RemoteAddr = "10.0.0.2:7777"
	req.Header.Set("User-Agent", "AMF/amf-1")
	req.Header.Set("X-Forwarded-For", "192.0.2.10, 10.0.0.2")
	req.Header.Set("3gpp-Sbi-Discovery-target-nf-type", "UDM")

	meta := ParseRequestMeta(req)
	if !meta.ViaSCP {
		t.Fatal("expected SCP forwarded request")
	}
	if got, want := meta.DisplayName(), "AMF/amf-1 via SCP"; got != want {
		t.Fatalf("DisplayName: got %q want %q", got, want)
	}
	if got, want := meta.DisplayRemoteAddr(), "192.0.2.10"; got != want {
		t.Fatalf("DisplayRemoteAddr: got %q want %q", got, want)
	}
	if got, want := meta.DisplayTransport("h2c"), "h2c via scp"; got != want {
		t.Fatalf("DisplayTransport: got %q want %q", got, want)
	}
}

func TestInboundMetadataMiddlewareTracksForwardedPeer(t *testing.T) {
	pt := peertracker.New()
	var meta RequestMeta
	handler := InboundMetadataMiddleware(pt, "h2c")(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		meta = RequestMetaFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/health", nil)
	req.RemoteAddr = "10.0.0.2:7777"
	req.Header.Set("User-Agent", "SMF/smf-1")
	req.Header.Set("X-Forwarded-For", "192.0.2.11")
	req.Header.Set("3gpp-Sbi-Discovery-target-nf-type", "PCF")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !meta.ViaSCP {
		t.Fatal("expected context metadata to indicate SCP")
	}
	peers := pt.List()
	if len(peers) != 1 {
		t.Fatalf("expected one tracked forwarded peer, got %#v", peers)
	}
	var found bool
	for _, p := range peers {
		if p.RemoteAddr == "192.0.2.11" {
			found = true
			if p.Name != "SMF/smf-1 via SCP" {
				t.Fatalf("peer name: got %q", p.Name)
			}
			if p.Transport != "h2c via scp" {
				t.Fatalf("peer transport: got %q", p.Transport)
			}
		}
	}
	if !found {
		t.Fatalf("forwarded peer not found: %#v", peers)
	}
}
