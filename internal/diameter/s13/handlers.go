package s13

import (
	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
	"github.com/svinson1121/vectorcore-hss/internal/taccache"
	"go.uber.org/zap"
)

type Handlers struct {
	store          repository.Repository
	log            *zap.Logger
	originHost     string
	originRealm    string
	eirNoMatchResp int
	eirIMSIIMEILog bool
	tac            *taccache.Cache // nil = TAC enrichment disabled
}

func NewHandlers(cfg *config.Config, store repository.Repository, log *zap.Logger) *Handlers {
	return &Handlers{
		store:          store,
		log:            log,
		originHost:     cfg.HSS.OriginHost,
		originRealm:    cfg.HSS.OriginRealm,
		eirNoMatchResp: cfg.EIR.NoMatchResponse,
		eirIMSIIMEILog: cfg.EIR.IMSIIMEILogging,
	}
}

// WithTAC attaches a TAC cache so device make/model is written into EIR history.
func (h *Handlers) WithTAC(c *taccache.Cache) *Handlers {
	h.tac = c
	return h
}
