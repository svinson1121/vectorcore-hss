package sbi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func TestNewRequestDirectPreservesTargetURL(t *testing.T) {
	client := NewClient(config.SBIClientConfig{Mode: RoutingModeDirect})
	req, err := client.NewRequestWithOptions(context.Background(), "GET", "http://nrf:7777/nnrf-nfm/v1/nf-instances?nf-type=NRF", nil, RequestOptions{
		RequesterNFType:       "UDM",
		RequesterNFInstanceID: "udm-1",
		TargetNFType:          "NRF",
		TargetServiceName:     "nnrf-nfm",
	})
	if err != nil {
		t.Fatalf("NewRequest direct: %v", err)
	}
	if got, want := req.URL.String(), "http://nrf:7777/nnrf-nfm/v1/nf-instances?nf-type=NRF"; got != want {
		t.Fatalf("URL: got %q want %q", got, want)
	}
	if got := req.Header.Get("3gpp-Sbi-Target-apiRoot"); got != "" {
		t.Fatalf("unexpected Target-apiRoot header %q", got)
	}
	if got, want := req.Header.Get("User-Agent"), "UDM-udm-1"; got != want {
		t.Fatalf("User-Agent: got %q want %q", got, want)
	}
	if got := req.Header.Get("3gpp-Sbi-Discovery-target-nf-type"); got != "" {
		t.Fatalf("unexpected discovery target header %q", got)
	}
}

func TestNewRequestSCPRewritesURLAndSetsTargetAPIRoot(t *testing.T) {
	client := NewClient(config.SBIClientConfig{
		Mode:       RoutingModeSCP,
		SCPAddress: "http://scp:7777/nscp-proxy/v1",
	})
	req, err := client.NewRequestWithOptions(context.Background(), "GET", "http://nrf:7777/nnrf-nfm/v1/nf-instances?nf-type=NRF", nil, RequestOptions{
		RequesterNFType:       "UDM",
		RequesterNFInstanceID: "udm-1",
		TargetNFType:          "NRF",
		TargetServiceName:     "nnrf-nfm",
	})
	if err != nil {
		t.Fatalf("NewRequest scp: %v", err)
	}
	if got, want := req.URL.String(), "http://scp:7777/nscp-proxy/v1/nnrf-nfm/v1/nf-instances?nf-type=NRF"; got != want {
		t.Fatalf("URL: got %q want %q", got, want)
	}
	if got, want := req.Header.Get("3gpp-Sbi-Target-apiRoot"), "http://nrf:7777"; got != want {
		t.Fatalf("Target-apiRoot: got %q want %q", got, want)
	}
	if got, want := req.Header.Get("3gpp-Sbi-Discovery-target-nf-type"), "NRF"; got != want {
		t.Fatalf("Discovery-target-nf-type: got %q want %q", got, want)
	}
	if got, want := req.Header.Get("3gpp-Sbi-Discovery-service-names"), "nnrf-nfm"; got != want {
		t.Fatalf("Discovery-service-names: got %q want %q", got, want)
	}
	if got, want := req.Header.Get("User-Agent"), "UDM-udm-1"; got != want {
		t.Fatalf("User-Agent: got %q want %q", got, want)
	}
}

func TestNewRequestSCPRequiresAbsoluteTarget(t *testing.T) {
	client := NewClient(config.SBIClientConfig{
		Mode:       RoutingModeSCP,
		SCPAddress: "http://scp:7777",
	})
	if _, err := client.NewRequest(context.Background(), "GET", "/nnrf-nfm/v1/nf-instances", nil); err == nil {
		t.Fatal("expected error for relative target URL")
	}
}

func TestDoHTTP2CarriesUserAgentHeader(t *testing.T) {
	var gotUserAgent string
	srv := httptest.NewUnstartedServer(h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNoContent)
	}), &http2.Server{}))
	srv.EnableHTTP2 = true
	srv.Start()
	defer srv.Close()

	client := NewClient(config.SBIClientConfig{Mode: RoutingModeDirect})
	req, err := client.NewRequestWithOptions(context.Background(), http.MethodGet, srv.URL+"/nnrf-nfm/v1/nf-instances", nil, RequestOptions{
		RequesterNFType:       "UDM",
		RequesterNFInstanceID: "udm-1",
	})
	if err != nil {
		t.Fatalf("NewRequestWithOptions: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	if gotUserAgent != "UDM-udm-1" {
		t.Fatalf("wire User-Agent: got %q", gotUserAgent)
	}
}

func TestDoHTTP2SCPModeCarriesUserAgentAndDiscoveryHeaders(t *testing.T) {
	var gotUserAgent, gotTargetRoot, gotTargetNF, gotService, gotPath string
	srv := httptest.NewUnstartedServer(h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		gotTargetRoot = r.Header.Get("3gpp-Sbi-Target-apiRoot")
		gotTargetNF = r.Header.Get("3gpp-Sbi-Discovery-target-nf-type")
		gotService = r.Header.Get("3gpp-Sbi-Discovery-service-names")
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}), &http2.Server{}))
	srv.EnableHTTP2 = true
	srv.Start()
	defer srv.Close()

	client := NewClient(config.SBIClientConfig{Mode: RoutingModeSCP, SCPAddress: srv.URL + "/nscp-proxy/v1"})
	req, err := client.NewRequestWithOptions(
		context.Background(),
		http.MethodPut,
		"http://nrf:7777/nnrf-nfm/v1/nf-instances/abc",
		bytes.NewReader([]byte(`{"nfType":"UDM"}`)),
		RequestOptions{
			RequesterNFType:       "UDM",
			RequesterNFInstanceID: "udm-1",
			TargetNFType:          "NRF",
			TargetServiceName:     "nnrf-nfm",
		},
	)
	if err != nil {
		t.Fatalf("NewRequestWithOptions: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	if gotPath != "/nscp-proxy/v1/nnrf-nfm/v1/nf-instances/abc" {
		t.Fatalf("wire path: got %q", gotPath)
	}
	if gotUserAgent != "UDM-udm-1" {
		t.Fatalf("wire User-Agent: got %q", gotUserAgent)
	}
	if gotTargetRoot != "http://nrf:7777" {
		t.Fatalf("wire Target-apiRoot: got %q", gotTargetRoot)
	}
	if gotTargetNF != "NRF" {
		t.Fatalf("wire target nf: got %q", gotTargetNF)
	}
	if gotService != "nnrf-nfm" {
		t.Fatalf("wire service names: got %q", gotService)
	}
}
