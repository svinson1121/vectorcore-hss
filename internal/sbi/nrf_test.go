package sbi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/peertracker"
)

func TestNRFHeartbeatRemovesPeerOnRequestBuildError(t *testing.T) {
	peers := peertracker.New()
	r := NewNRFRegistrar("udm", "http://127.0.0.1:7777", "inst-1", NFProfile{
		NFInstanceID: "inst-1",
		NFType:       "UDM",
	}, zap.NewNop(), peers, config.SBIClientConfig{Mode: RoutingModeSCP})
	r.addPeer("NRF", r.nrfAddress)

	r.heartbeatOnce(context.Background(), "http://127.0.0.1:7777/nnrf-nfm/v1/nf-instances/inst-1", []byte(`[]`))

	if got := peers.List(); len(got) != 0 {
		t.Fatalf("expected heartbeat build failure to remove NRF peer, got %#v", got)
	}
}

func TestNRFHeartbeatRemovesPeerOnTransportError(t *testing.T) {
	peers := peertracker.New()
	r := NewNRFRegistrar("udm", "http://127.0.0.1:1", "inst-1", NFProfile{
		NFInstanceID: "inst-1",
		NFType:       "UDM",
	}, zap.NewNop(), peers, config.SBIClientConfig{Mode: RoutingModeDirect})
	r.addPeer("NRF", r.nrfAddress)

	r.heartbeatOnce(context.Background(), "http://127.0.0.1:1/nnrf-nfm/v1/nf-instances/inst-1", []byte(`[]`))

	if got := peers.List(); len(got) != 0 {
		t.Fatalf("expected transport failure to remove NRF peer, got %#v", got)
	}
}

func TestNRFHeartbeatAddsPeerOnSuccess(t *testing.T) {
	srv := httptest.NewUnstartedServer(h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("unexpected method %s", req.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}), &http2.Server{}))
	srv.Start()
	defer srv.Close()

	peers := peertracker.New()
	r := NewNRFRegistrar("udm", srv.URL, "inst-1", NFProfile{
		NFInstanceID: "inst-1",
		NFType:       "UDM",
	}, zap.NewNop(), peers, config.SBIClientConfig{Mode: RoutingModeDirect})

	r.heartbeatOnce(context.Background(), srv.URL+"/nnrf-nfm/v1/nf-instances/inst-1", []byte(`[]`))

	got := peers.List()
	if len(got) != 1 {
		t.Fatalf("expected successful heartbeat to add NRF peer, got %#v", got)
	}
	if got[0].Name != "NRF" {
		t.Fatalf("expected peer name NRF, got %#v", got[0])
	}
}

func TestNRFHeartbeatReregistersOnNotFound(t *testing.T) {
	var patchCalls atomic.Int32
	var putCalls atomic.Int32
	srv := httptest.NewUnstartedServer(h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPatch:
			patchCalls.Add(1)
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPut:
			putCalls.Add(1)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
	}), &http2.Server{}))
	srv.Start()
	defer srv.Close()

	peers := peertracker.New()
	r := NewNRFRegistrar("udm", srv.URL, "inst-1", NFProfile{
		NFInstanceID: "inst-1",
		NFType:       "UDM",
	}, zap.NewNop(), peers, config.SBIClientConfig{Mode: RoutingModeDirect})

	r.heartbeatOnce(context.Background(), srv.URL+"/nnrf-nfm/v1/nf-instances/inst-1", []byte(`[]`))

	if got := patchCalls.Load(); got != 1 {
		t.Fatalf("expected 1 PATCH call, got %d", got)
	}
	if got := putCalls.Load(); got != 1 {
		t.Fatalf("expected 1 PUT re-registration call, got %d", got)
	}
	got := peers.List()
	if len(got) != 1 {
		t.Fatalf("expected successful re-registration to add NRF peer, got %#v", got)
	}
	if got[0].Name != "NRF" {
		t.Fatalf("expected peer name NRF, got %#v", got[0])
	}
}
