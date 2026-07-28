package slh

import (
	"context"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/models"
	"go.uber.org/zap"
)

type Handlers struct {
	store            subscriberStore
	log              *zap.Logger
	originHost       string
	originRealm      string
	authorizedRealms []string
}

type subscriberStore interface {
	GetSubscriberByIMSI(context.Context, string) (*models.Subscriber, error)
	GetSubscriberByMSISDN(context.Context, string) (*models.Subscriber, error)
}

func NewHandlers(cfg *config.Config, store subscriberStore, log *zap.Logger) *Handlers {
	return &Handlers{
		store:            store,
		log:              log,
		originHost:       cfg.HSS.OriginHost,
		originRealm:      cfg.HSS.OriginRealm,
		authorizedRealms: cfg.HSS.SLhAuthorizedRealms,
	}
}
