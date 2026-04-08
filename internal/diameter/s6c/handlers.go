package s6c

import (
	"sync"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
	"go.uber.org/zap"
)

// PeerLookup resolves an active Diameter connection by the peer's OriginHost.
// Implemented by *diameter.ConnTracker; the interface breaks the import cycle.
type PeerLookup interface {
	GetConn(originHost string) (diam.Conn, bool)
}

// pendingALSCEntry tracks an in-flight ALSC so MWD is only deleted after the
// SMS-SC returns a successful ASA (Result-Code 2001).
type pendingALSCEntry struct {
	imsi    string
	scAddr  string
}

type Handlers struct {
	store       repository.Repository
	log         *zap.Logger
	originHost  string
	originRealm string
	peers       PeerLookup
	// pendingALSC maps Diameter Session-ID → pendingALSCEntry for in-flight ALSC requests.
	pendingALSC sync.Map
}

func NewHandlers(cfg *config.Config, store repository.Repository, log *zap.Logger, peers PeerLookup) *Handlers {
	return &Handlers{
		store:       store,
		log:         log,
		originHost:  cfg.HSS.OriginHost,
		originRealm: cfg.HSS.OriginRealm,
		peers:       peers,
	}
}
