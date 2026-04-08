package api

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/metrics"
	"github.com/svinson1121/vectorcore-hss/internal/taccache"
	"github.com/svinson1121/vectorcore-hss/internal/version"
)

// CLRSender allows the API to trigger a Cancel-Location-Request via the
// Diameter layer without importing the diameter package directly.
type CLRSender interface {
	SendCLRByIMSI(ctx context.Context, imsi string) error
}

// CacheInvalidator allows the API to evict subscriber cache entries when a
// subscriber record is modified, ensuring Diameter handlers see fresh data.
type CacheInvalidator interface {
	InvalidateCache(imsi string)
}

// ConnectedPeer describes a directly connected Diameter peer.
type ConnectedPeer struct {
	OriginHost  string `json:"origin_host" doc:"Diameter Origin-Host from CER"`
	OriginRealm string `json:"origin_realm" doc:"Diameter Origin-Realm from CER"`
	RemoteAddr  string `json:"remote_addr" doc:"Remote address (host:port)"`
	Transport   string `json:"transport" doc:"Transport protocol: tcp or sctp"`
}

// PeerLister returns a snapshot of directly connected Diameter peers.
type PeerLister interface {
	List() []ConnectedPeer
}

// AuthFailureLister returns a snapshot of recent S6a AIR authentication failures.
type AuthFailureLister interface {
	RecentAuthFailures() []AuthFailure
}

// AuthFailure is a single failed S6a AIR attempt exposed by the API.
type AuthFailure struct {
	IMSI      string `json:"imsi"`
	Timestamp string `json:"timestamp"`
	Reason    string `json:"reason"`
	PeerAddr  string `json:"peer_addr"`
}

type Server struct {
	db           *gorm.DB
	log          *zap.Logger
	cfg          config.APIConfig
	clr          CLRSender        // nil when CLR is not wired (e.g. Diameter disabled)
	cache        CacheInvalidator // nil when Diameter store is not wired
	tac          *taccache.Cache  // nil when TAC DB is disabled in config
	geored       GeoredManager    // nil when GeoRed is disabled
	peers        PeerLister       // nil when Diameter is not wired
	authFailures AuthFailureLister // nil when Diameter is not wired
}

// WithTAC attaches the TAC cache so API writes keep the cache in sync.
func (s *Server) WithTAC(c *taccache.Cache) *Server {
	s.tac = c
	return s
}

func New(db *gorm.DB, cfg config.APIConfig, log *zap.Logger) *Server {
	registerAuditCallbacks(db)
	return &Server{db: db, cfg: cfg, log: log}
}

// WithPeers attaches a Diameter peer lister so the API can expose connected peers.
func (s *Server) WithPeers(p PeerLister) *Server {
	s.peers = p
	return s
}

// WithAuthFailures attaches an auth failure lister so the API can expose recent S6a failures.
func (s *Server) WithAuthFailures(a AuthFailureLister) *Server {
	s.authFailures = a
	return s
}

// WithCLR attaches a CLR dispatcher to the API server.
func (s *Server) WithCLR(c CLRSender) *Server {
	s.clr = c
	return s
}

// WithCache attaches a cache invalidator to the API server so that subscriber
// updates immediately evict stale cache entries in the Diameter store.
func (s *Server) WithCache(c CacheInvalidator) *Server {
	s.cache = c
	return s
}

func (s *Server) Start() error {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, req.ProtoMajor)
			next.ServeHTTP(ww, req.WithContext(WithAudit(req.Context())))
			elapsed := time.Since(start)
			path := req.URL.Path
			metrics.APIRequestsTotal.WithLabelValues(req.Method, path, strconv.Itoa(ww.Status())).Inc()
			metrics.APIRequestDuration.WithLabelValues(req.Method, path).Observe(elapsed.Seconds())
		})
	})

	// Prometheus scrape endpoint — stays at /metrics (outside versioned API)
	r.Handle("/metrics", promhttp.Handler())

	// React UI — served at /ui/ with SPA fallback routing
	r.Mount("/ui", uiHandler())
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/", http.StatusFound)
	})

	// All OAM REST endpoints live under /api/v1
	r.Route("/api/v1", func(sub chi.Router) {
		humaConfig := huma.DefaultConfig("VectorCore HSS API", version.APIVersion)
		humaConfig.Info.Description = "OAM REST API for the VectorCore Home Subscriber Server"
		// Tell Huma its base path so the docs page fetches /api/v1/openapi.yaml correctly.
		humaConfig.Servers = []*huma.Server{{URL: "/api/v1"}}
		api := humachi.New(sub, humaConfig)
		s.registerRoutes(api)
	})

	addr := net.JoinHostPort(s.cfg.BindAddress, strconv.Itoa(s.cfg.BindPort))
	srv := &http.Server{Addr: addr, Handler: s.auth(r)}

	if s.cfg.TLSCertFile != "" && s.cfg.TLSKeyFile != "" {
		s.log.Info("api: TLS listening", zap.String("addr", addr))
		return srv.ListenAndServeTLS(s.cfg.TLSCertFile, s.cfg.TLSKeyFile)
	}
	s.log.Info("api: listening", zap.String("addr", addr))
	return srv.ListenAndServe()
}

func (s *Server) registerRoutes(api huma.API) {
	registerAPNRoutes(s, api)
	registerAUCRoutes(s, api)
	registerAlgorithmProfileRoutes(s, api)
	registerSubscriberRoutes(s, api)
	registerIMSSubscriberRoutes(s, api)
	registerIFCProfileRoutes(s, api)
	registerEIRRoutes(s, api)
	registerEIRHistoryRoutes(s, api)
	registerRoamingRoutes(s, api)
	registerChargingRoutes(s, api)
	registerTFTRoutes(s, api)
	registerSubscriberRoutingRoutes(s, api)
	registerServingAPNRoutes(s, api)
	registerPDUSessionRoutes(s, api)
	registerSubscriberAttributeRoutes(s, api)
	registerEmergencySubscriberRoutes(s, api)
	registerTACRoutes(s, api)
	registerOperationLogRoutes(s, api)
	registerMetricsRoutes(s, api)
	registerBackupRoutes(s, api)
	registerVersionRoutes(api)
	registerSubscriberActionRoutes(s, api)
	if s.geored != nil {
		registerGeoredRoutes(s, api)
	}
	if s.peers != nil {
		registerDiameterPeersRoutes(s, api)
	}
	if s.authFailures != nil {
		registerAuthFailureRoutes(s, api)
	}
}

