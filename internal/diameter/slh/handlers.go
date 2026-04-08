package slh

import (
	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
	"go.uber.org/zap"
)

type Handlers struct {
	store       repository.Repository
	log         *zap.Logger
	originHost  string
	originRealm string
}

func NewHandlers(cfg *config.Config, store repository.Repository, log *zap.Logger) *Handlers {
	return &Handlers{
		store:       store,
		log:         log,
		originHost:  cfg.HSS.OriginHost,
		originRealm: cfg.HSS.OriginRealm,
	}
}
