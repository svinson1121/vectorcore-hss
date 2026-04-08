package geored

// server.go -- Inter-node HTTP/2 listener on the GeoRed port (default 9869).
//
// Routes (all protected by bearer token middleware):
//   POST /geored/v1/events   — receive batched events from a peer
//   GET  /geored/v1/snapshot — return full dynamic state to a peer
//   GET  /geored/v1/health   — liveness probe

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

// StartServer starts the inter-node HTTP/2 listener and returns immediately.
// It runs in a dedicated goroutine. Errors during startup are fatal (logged
// then the goroutine exits), not returned, because the caller (main.go) has
// already committed to the GeoRed config.
func StartServer(cfg config.GeoredConfig, store repository.Repository, log *zap.Logger) error {
	port := cfg.ListenPort
	if port <= 0 {
		port = 9869
	}

	mux := http.NewServeMux()
	mux.Handle("/geored/v1/events", eventsHandler(cfg.NodeID, store, log))
	mux.Handle("/geored/v1/snapshot", snapshotHandler(store, log))
	mux.Handle("/geored/v1/health", healthHandler())

	// Wrap every route with bearer token auth.
	protected := bearerAuth(cfg.BearerToken, mux)

	addr := fmt.Sprintf(":%d", port)

	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		return startTLS(addr, cfg.TLSCertFile, cfg.TLSKeyFile, protected, log)
	}
	return startH2C(addr, protected, log)
}

// startTLS starts an HTTPS/HTTP2 (TLS + ALPN h2) listener.
func startTLS(addr, certFile, keyFile string, handler http.Handler, log *zap.Logger) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("geored: tls: %w", err)
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h2"},
	}
	ln, err := tls.Listen("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("geored: listen %s: %w", addr, err)
	}

	srv := &http.Server{Handler: handler}
	if err := http2.ConfigureServer(srv, nil); err != nil {
		return fmt.Errorf("geored: http2 configure: %w", err)
	}

	log.Info("geored: inter-node listener started (TLS/h2)", zap.String("addr", addr))
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Error("geored: listener error", zap.Error(err))
		}
	}()
	return nil
}

// startH2C starts a cleartext HTTP/2 (h2c) listener.
func startH2C(addr string, handler http.Handler, log *zap.Logger) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("geored: listen %s: %w", addr, err)
	}

	h2s := &http2.Server{}
	srv := &http.Server{Handler: h2c.NewHandler(handler, h2s)}

	log.Info("geored: inter-node listener started (h2c)", zap.String("addr", addr))
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Error("geored: listener error", zap.Error(err))
		}
	}()
	return nil
}
