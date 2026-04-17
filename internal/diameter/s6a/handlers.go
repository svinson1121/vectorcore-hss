package s6a

import (
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/geored"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
	"go.uber.org/zap"
)

const authFailureRingSize = 10

// AuthFailure records a single failed S6a AIR authentication attempt.
type AuthFailure struct {
	IMSI        string    `json:"imsi"`
	Timestamp   time.Time `json:"timestamp"`
	Reason      string    `json:"reason"`
	PeerAddr    string    `json:"peer_addr"`
	AuthScope   string    `json:"auth_scope"`
	VisitedPLMN string    `json:"visited_plmn"`
	VisitedMCC  string    `json:"visited_mcc"`
	VisitedMNC  string    `json:"visited_mnc"`
}

// PeerLookup lets ULR find an old MME's active connection to send CLR.
// Implemented by *diameter.ConnTracker; the interface breaks the import cycle
// since s6a already imports diam but must not import internal/diameter.
type PeerLookup interface {
	GetConn(originHost string) (diam.Conn, bool)
}

type Handlers struct {
	store                 repository.Repository
	log                   *zap.Logger
	originHost            string
	originRealm           string
	clrEnabled            bool
	peers                 PeerLookup
	eirNoMatchResp        int
	eirIMSIIMEILog        bool
	homeMCC               string
	homeMNC               string
	allowUndefinedRoaming bool
	pub                   geored.TypedPublisher
	// onRegister is called after a successful ULR (LTE attach).
	// Wired to the S6c alert sender by the server to trigger ALR for
	// pending Message Waiting Data caused by subscriber absence.
	onRegister func(imsi string)
	// onSubscriberReady is called after a NOR that indicates the subscriber
	// is ready again for SMS delivery. The trigger differentiates user
	// presence from memory-available recovery. When the MME provides
	// Maximum-UE-Availability-Time, it is passed through for ALR construction.
	onSubscriberReady func(imsi string, trigger AlertTrigger, maximumUEAvailabilityTime *time.Time)

	failMu       sync.Mutex
	authFailures []AuthFailure
}

func (h *Handlers) authFailureContext(visitedPLMN []byte) (scope, plmnHex, mcc, mnc string) {
	if len(visitedPLMN) > 0 {
		plmnHex = strings.ToUpper(hex.EncodeToString(visitedPLMN))
	}
	if len(visitedPLMN) != 3 {
		return "unknown", plmnHex, "", ""
	}
	mcc, mnc = decodePLMN(visitedPLMN)
	if mcc == h.homeMCC && mnc == h.homeMNC {
		return "local", plmnHex, mcc, mnc
	}
	return "roaming", plmnHex, mcc, mnc
}

// RecordAuthFailure appends a failure to the in-memory ring buffer (last 10).
func (h *Handlers) RecordAuthFailure(imsi, peerAddr, reason string, visitedPLMN []byte) {
	scope, plmnHex, mcc, mnc := h.authFailureContext(visitedPLMN)
	h.failMu.Lock()
	defer h.failMu.Unlock()
	h.authFailures = append(h.authFailures, AuthFailure{
		IMSI:        imsi,
		Timestamp:   time.Now().UTC(),
		Reason:      reason,
		PeerAddr:    peerAddr,
		AuthScope:   scope,
		VisitedPLMN: plmnHex,
		VisitedMCC:  mcc,
		VisitedMNC:  mnc,
	})
	if len(h.authFailures) > authFailureRingSize {
		h.authFailures = h.authFailures[len(h.authFailures)-authFailureRingSize:]
	}
}

// RecentAuthFailures returns a copy of recent failures, newest first.
func (h *Handlers) RecentAuthFailures() []AuthFailure {
	h.failMu.Lock()
	defer h.failMu.Unlock()
	out := make([]AuthFailure, len(h.authFailures))
	for i, f := range h.authFailures {
		out[len(h.authFailures)-1-i] = f
	}
	return out
}

func NewHandlers(cfg *config.Config, store repository.Repository, log *zap.Logger, peers PeerLookup) *Handlers {
	return &Handlers{
		store:                 store,
		log:                   log,
		originHost:            cfg.HSS.OriginHost,
		originRealm:           cfg.HSS.OriginRealm,
		clrEnabled:            cfg.HSS.CancelLocationRequestEnabled,
		peers:                 peers,
		eirNoMatchResp:        cfg.EIR.NoMatchResponse,
		eirIMSIIMEILog:        cfg.EIR.IMSIIMEILogging,
		homeMCC:               cfg.HSS.MCC,
		homeMNC:               cfg.HSS.MNC,
		allowUndefinedRoaming: cfg.Roaming.AllowUndefinedNetworks,
		pub:                   geored.NoopTypedPublisher{},
	}
}

// WithGeored attaches a GeoRed publisher to the S6a handler.
func (h *Handlers) WithGeored(pub geored.TypedPublisher) *Handlers {
	h.pub = pub
	return h
}

// WithOnRegister sets a callback invoked after each successful LTE attach (ULR).
// Used to trigger S6c Alert-Service-Centre for pending Message Waiting Data.
func (h *Handlers) WithOnRegister(fn func(imsi string)) *Handlers {
	h.onRegister = fn
	return h
}

// WithOnSubscriberReady sets a callback invoked when a NOR indicates the
// subscriber is again ready for SMS delivery.
func (h *Handlers) WithOnSubscriberReady(fn func(imsi string, trigger AlertTrigger, maximumUEAvailabilityTime *time.Time)) *Handlers {
	h.onSubscriberReady = fn
	return h
}
