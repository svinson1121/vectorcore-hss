package swx

import (
	"sync"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
	"go.uber.org/zap"
)

// PeerLookup lets SWx send RTR/PPR to connected AAA servers.
type PeerLookup interface {
	GetConn(originHost string) (diam.Conn, bool)
}

type Handlers struct {
	store       repository.Repository
	log         *zap.Logger
	originHost  string
	originRealm string
	peers       PeerLookup
	pendingMu   sync.Mutex
	pending     map[swxTransactionKey]swxTransaction
}

func NewHandlers(cfg *config.Config, store repository.Repository, log *zap.Logger, peers PeerLookup) *Handlers {
	return &Handlers{
		store:       store,
		log:         log,
		originHost:  cfg.HSS.OriginHost,
		originRealm: cfg.HSS.OriginRealm,
		peers:       peers,
		pending:     make(map[swxTransactionKey]swxTransaction),
	}
}

type swxTransactionKey struct {
	appID     uint32
	command   uint32
	hopByHop  uint32
	endToEnd  uint32
	sessionID string
}

type swxTransaction struct {
	imsi  string
	timer *time.Timer
}

func newSWxVendorSpecificApplicationID() *diam.AVP {
	return diam.NewAVP(avp.VendorSpecificApplicationID, avp.Mbit, 0, &diam.GroupedAVP{AVP: []*diam.AVP{
		diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(Vendor3GPP)),
		diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(AppIDSWx)),
	}})
}

func (h *Handlers) RTA(_ diam.Conn, msg *diam.Message) {
	h.logSWxAnswer("RTA", msg)
}

func (h *Handlers) PPA(_ diam.Conn, msg *diam.Message) {
	h.logSWxAnswer("PPA", msg)
}

func (h *Handlers) UnsupportedRequest(_ diam.Conn, msg *diam.Message) (*diam.Message, error) {
	h.log.Warn("unsupported inbound SWx request",
		zap.Uint32("application_id", msg.Header.ApplicationID),
		zap.Uint32("command_code", msg.Header.CommandCode),
		zap.Bool("request", msg.Header.CommandFlags&diam.RequestFlag == diam.RequestFlag),
		zap.String("action", "send_error_answer"))

	ans := msg.Answer(diam.CommandUnsupported)
	ans.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(h.originHost))
	ans.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(h.originRealm))
	ans.AddAVP(newSWxVendorSpecificApplicationID())
	ans.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	return ans, nil
}

func (h *Handlers) logSWxAnswer(name string, msg *diam.Message) {
	var ans struct {
		SessionID  datatype.UTF8String       `avp:"Session-Id"`
		OriginHost datatype.DiameterIdentity `avp:"Origin-Host"`
		ResultCode datatype.Unsigned32       `avp:"Result-Code"`
		UserName   datatype.UTF8String       `avp:"User-Name,omitempty"`
	}
	if err := msg.Unmarshal(&ans); err != nil {
		h.log.Error("SWx 304/305 decode failure",
			zap.String("answer", name),
			zap.Uint32("application_id", msg.Header.ApplicationID),
			zap.Uint32("command_code", msg.Header.CommandCode),
			zap.Error(err))
		return
	}
	txn, correlated := h.completeSWxTransaction(msg, string(ans.SessionID))
	fields := []zap.Field{
		zap.String("session_id", string(ans.SessionID)),
		zap.String("origin_host", string(ans.OriginHost)),
		zap.Uint32("result_code", uint32(ans.ResultCode)),
		zap.Uint32("application_id", msg.Header.ApplicationID),
		zap.Uint32("command_code", msg.Header.CommandCode),
		zap.Uint32("hop_by_hop_id", msg.Header.HopByHopID),
		zap.Uint32("end_to_end_id", msg.Header.EndToEndID),
		zap.Bool("correlated", correlated),
	}
	if correlated && txn.imsi != "" {
		fields = append(fields, zap.String("imsi", txn.imsi))
	} else if ans.UserName != "" {
		fields = append(fields, zap.String("imsi", string(ans.UserName)))
	}
	h.log.Info("SWx "+name+" received", fields...)
}

func (h *Handlers) trackSWxTransaction(msg *diam.Message, sessionID, imsi string) {
	key := swxTransactionKey{
		appID:     msg.Header.ApplicationID,
		command:   msg.Header.CommandCode,
		hopByHop:  msg.Header.HopByHopID,
		endToEnd:  msg.Header.EndToEndID,
		sessionID: sessionID,
	}
	timer := time.AfterFunc(30*time.Second, func() {
		h.pendingMu.Lock()
		_, ok := h.pending[key]
		if ok {
			delete(h.pending, key)
		}
		h.pendingMu.Unlock()
		if ok {
			h.log.Warn("SWx transaction timeout",
				zap.String("imsi", imsi),
				zap.String("session_id", sessionID),
				zap.Uint32("application_id", key.appID),
				zap.Uint32("command_code", key.command),
				zap.Uint32("hop_by_hop_id", key.hopByHop),
				zap.Uint32("end_to_end_id", key.endToEnd))
		}
	})

	h.pendingMu.Lock()
	h.pending[key] = swxTransaction{imsi: imsi, timer: timer}
	h.pendingMu.Unlock()
}

func (h *Handlers) forgetSWxTransaction(msg *diam.Message, sessionID string) {
	key := swxTransactionKey{
		appID:     msg.Header.ApplicationID,
		command:   msg.Header.CommandCode,
		hopByHop:  msg.Header.HopByHopID,
		endToEnd:  msg.Header.EndToEndID,
		sessionID: sessionID,
	}
	h.pendingMu.Lock()
	txn, ok := h.pending[key]
	if ok {
		delete(h.pending, key)
	}
	h.pendingMu.Unlock()
	if ok && txn.timer != nil {
		txn.timer.Stop()
	}
}

func (h *Handlers) completeSWxTransaction(msg *diam.Message, sessionID string) (swxTransaction, bool) {
	key := swxTransactionKey{
		appID:     msg.Header.ApplicationID,
		command:   msg.Header.CommandCode,
		hopByHop:  msg.Header.HopByHopID,
		endToEnd:  msg.Header.EndToEndID,
		sessionID: sessionID,
	}
	h.pendingMu.Lock()
	txn, ok := h.pending[key]
	if ok {
		delete(h.pending, key)
	}
	h.pendingMu.Unlock()
	if ok && txn.timer != nil {
		txn.timer.Stop()
	}
	return txn, ok
}
