package udm

// server.go — VectorCore UDR/UDM server.
//
// Implements the 3GPP Nudm SBI interfaces used by Open5GS AUSF, AMF, and SMF.
// VectorCore acts as both UDM (application logic) and UDR (data repository) —
// there is no separate UDR process; we go directly to PostgreSQL.
//
// Interfaces served:
//   nudm-ueau  — Nudm_UEAuthentication  (AUSF calls this for 5G-AKA vectors)
//   nudm-sdm   — Nudm_SDM              (AMF/SMF fetch subscription data)
//   nudm-uecm  — Nudm_UECM             (AMF/SMF register UE context)
//
// Transport: HTTP/2.  TLS absent → h2c (cleartext HTTP/2, typical for labs).
//            TLS present → TLS + h2 (production).

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

// Server is the UDR/UDM HTTP/2 server.
type Server struct {
	cfg   config.UDMConfig
	store repository.Repository
	log   *zap.Logger
	jwks  *jwksStore
}

// New creates a new UDM server.
// If cfg.NFInstanceID is empty a random UUID v4 is generated so the NRF
// registration always has a stable-per-process identity.
func New(cfg config.UDMConfig, store repository.Repository, log *zap.Logger) *Server {
	if cfg.NFInstanceID == "" {
		cfg.NFInstanceID = newUUID()
		log.Info("udm: generated NFInstanceID", zap.String("nf_instance_id", cfg.NFInstanceID))
	}
	return &Server{
		cfg:   cfg,
		store: store,
		log:   log,
		jwks:  &jwksStore{},
	}
}

// newUUID returns a random RFC 4122 version 4 UUID string.
func newUUID() string {
	var b [16]byte
	rand.Read(b[:]) //nolint:errcheck — crypto/rand.Read never returns an error on supported platforms
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Start begins listening.  Blocks until a fatal error.
func (s *Server) Start() error {
	r := chi.NewRouter()
	s.mountRoutes(r)

	addr := net.JoinHostPort(s.cfg.BindAddress, fmt.Sprintf("%d", s.cfg.BindPort))
	s.log.Info("udm: UDR/UDM listening", zap.String("addr", addr))

	// Start NRF registration in the background.
	go s.startNRFRegistration()

	if s.cfg.TLSCertFile != "" && s.cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(s.cfg.TLSCertFile, s.cfg.TLSKeyFile)
		if err != nil {
			return fmt.Errorf("udm: load TLS cert: %w", err)
		}
		srv := &http.Server{
			Addr:    addr,
			Handler: r,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				NextProtos:   []string{"h2"},
			},
		}
		http2.ConfigureServer(srv, nil)
		return srv.ListenAndServeTLS("", "")
	}

	// Cleartext HTTP/2 (h2c) — the normal mode for Open5GS lab deployments.
	srv := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(r, &http2.Server{}),
	}
	return srv.ListenAndServe()
}

// mountRoutes registers all Nudm endpoints on the chi router.
// Both v1 and v2 URL prefixes are mounted to the same handlers.
func (s *Server) mountRoutes(r *chi.Mux) {
	for _, ver := range []string{"v1", "v2"} {
		// nudm-ueau — UE authentication (AUSF)
		ueau := fmt.Sprintf("/nudm-ueau/%s/{supi}", ver)
		r.Post(ueau+"/security-information/generate-auth-data",
			s.wrapOAuth("nudm-ueau", s.handleGenerateAuthData))
		r.Post(ueau+"/auth-events",
			s.wrapOAuth("nudm-ueau", s.handleAuthEvent))

		// nudm-sdm — Subscription data management (AMF, SMF)
		sdm := fmt.Sprintf("/nudm-sdm/%s/{supi}", ver)
		r.Get(sdm+"/am-data",
			s.wrapOAuth("nudm-sdm", s.handleAMData))
		r.Get(sdm+"/sm-data",
			s.wrapOAuth("nudm-sdm", s.handleSMData))
		r.Get(sdm+"/smf-select-data",
			s.wrapOAuth("nudm-sdm", s.handleSMFSelectData))
		r.Get(sdm+"/nssai",
			s.wrapOAuth("nudm-sdm", s.handleNSSAI))
		r.Post(sdm+"/sdm-subscriptions",
			s.wrapOAuth("nudm-sdm", s.handleSDMSubscribe))
		r.Delete(sdm+"/sdm-subscriptions/{subscriptionId}",
			s.wrapOAuth("nudm-sdm", s.handleSDMUnsubscribe))
	}

	// nudm-uecm — UE context management (AMF, SMF) — v1 only in Open5GS
	for _, ver := range []string{"v1"} {
		uecm := fmt.Sprintf("/nudm-uecm/%s/{supi}/registrations", ver)
		r.Put(uecm+"/amf-3gpp-access",
			s.wrapOAuth("nudm-uecm", s.handleAMFRegistrationPut))
		r.Patch(uecm+"/amf-3gpp-access",
			s.wrapOAuth("nudm-uecm", s.handleAMFRegistrationPatch))
		r.Delete(uecm+"/amf-3gpp-access",
			s.wrapOAuth("nudm-uecm", s.handleAMFRegistrationDelete))
		r.Get(uecm,
			s.wrapOAuth("nudm-uecm", s.handleGetRegistrations))
		r.Put(uecm+"/smf-registrations/{pduSessionId}",
			s.wrapOAuth("nudm-uecm", s.handleSMFRegistrationPut))
		r.Delete(uecm+"/smf-registrations/{pduSessionId}",
			s.wrapOAuth("nudm-uecm", s.handleSMFRegistrationDelete))
	}

	// nudr-dr — UDR Data Management (N36: PCF → UDR, N37: NEF → UDR)
	// v1 only — Open5GS PCF does not use v2 for nudr-dr.
	for _, ver := range []string{"v1", "v2"} {
		pd := fmt.Sprintf("/nudr-dr/%s/policy-data/ues/{ueId}", ver)
		r.Get(pd+"/am-data",
			s.wrapOAuth("nudr-dr", s.handleUDRAMPolicyData))
		r.Get(pd+"/sm-data",
			s.wrapOAuth("nudr-dr", s.handleUDRSMPolicyData))
		r.Get(pd+"/ue-policy-set",
			s.wrapOAuth("nudr-dr", s.handleUDRUEPolicySet))
		r.Get(fmt.Sprintf("/nudr-dr/%s/policy-data/sms-management-data/{ueId}", ver),
			s.wrapOAuth("nudr-dr", s.handleUDRSMSManagement))
	}

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","nf":"UDM/UDR"}`))
	})
}

// wrapOAuth wraps a handler with OAuth2 middleware for the given scope.
func (s *Server) wrapOAuth(scope string, h http.HandlerFunc) http.HandlerFunc {
	wrapped := oauthMiddleware(s.cfg.OAuth2Enabled, s.cfg.OAuth2Bypass, scope, s.jwks, s.log, h)
	return wrapped.ServeHTTP
}

// ── JSON helpers ─────────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, code int, cause string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"cause": cause})
}
