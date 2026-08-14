package gx

import (
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/geored"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
	"go.uber.org/zap"
)

// PeerLookup lets Gx find active peer connections.
type PeerLookup interface {
	GetConn(originHost string) (diam.Conn, bool)
}

type Handlers struct {
	store       repository.Repository
	log         *zap.Logger
	originHost  string
	originRealm string
	peers       PeerLookup
	pub         geored.TypedPublisher
	tftHandling string
	// timeout bounds the database context deadline for each CCR handler
	// call. Configurable via hss.Timeouts.GxSeconds.
	timeout time.Duration
}

func NewHandlers(cfg *config.Config, store repository.Repository, log *zap.Logger, peers PeerLookup) *Handlers {
	timeout := time.Duration(cfg.Timeouts.GxSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Handlers{
		store:       store,
		log:         log,
		originHost:  cfg.HSS.OriginHost,
		originRealm: cfg.HSS.OriginRealm,
		peers:       peers,
		pub:         geored.NoopTypedPublisher{},
		tftHandling: cfg.PCRF.TFTHandling,
		timeout:     timeout,
	}
}

// WithGeored attaches a GeoRed publisher to the Gx handler.
func (h *Handlers) WithGeored(pub geored.TypedPublisher) *Handlers {
	h.pub = pub
	return h
}
