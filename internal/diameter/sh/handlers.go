package sh

import (
	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
	"go.uber.org/zap"
	"sync"
)

// PeerLookup lets Sh send PNR to registered Application Servers.
type PeerLookup interface {
	GetConn(originHost string) (diam.Conn, bool)
}

type Handlers struct {
	store       repository.Repository
	log         *zap.Logger
	originHost  string
	originRealm string
	mcc         string
	mnc         string
	peers       PeerLookup
	// pendingPNR maps Diameter Session-ID to the original outbound request
	// context so PNA answers can be correlated and logged accurately.
	pendingPNR sync.Map
}

func NewHandlers(cfg *config.Config, store repository.Repository, log *zap.Logger, peers PeerLookup) *Handlers {
	return &Handlers{
		store:       store,
		log:         log,
		originHost:  cfg.HSS.OriginHost,
		originRealm: cfg.HSS.OriginRealm,
		mcc:         cfg.HSS.MCC,
		mnc:         cfg.HSS.MNC,
		peers:       peers,
	}
}
