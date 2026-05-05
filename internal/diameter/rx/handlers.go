package rx

import (
	"sync"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/gx"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

// PeerLookup lets Rx find active Gx peer connections to send RAR.
type PeerLookup interface {
	GetConn(originHost string) (diam.Conn, bool)
}

type Handlers struct {
	store       repository.Repository
	log         *zap.Logger
	originHost  string
	originRealm string
	peers       PeerLookup
	tftHandling string

	mu       sync.Mutex
	sessions map[string]*rxSession // Rx session-id → rxSession
}

// buildRxAnswer constructs an Rx AAA/STA frame matching the PyHSS AVP layout:
// Session-Id, Auth-Application-Id, Origin-Host, Origin-Realm, Supported-Features, Result-Code.
// This is the correct structure for Rx per 3GPP TS 29.214 — uses Auth-Application-Id,
// NOT Auth-Session-State (which belongs to S6a/Cx).
func buildRxAnswer(req *diam.Message, sessionID datatype.UTF8String, originHost, originRealm string) *diam.Message {
	ans := diam.NewMessage(req.Header.CommandCode, req.Header.CommandFlags&^diam.RequestFlag, AppIDRx, req.Header.HopByHopID, req.Header.EndToEndID, req.Dictionary())
	ans.InsertAVP(diam.NewAVP(avp.SessionID, avp.Mbit, 0, sessionID))
	ans.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(AppIDRx))
	ans.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(originHost))
	ans.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(originRealm))
	ans.NewAVP(avp.SupportedFeatures, avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: []*diam.AVP{
		diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(Vendor3GPP)),
		diam.NewAVP(avp.FeatureListID, avp.Vbit, Vendor3GPP, datatype.Unsigned32(1)),
		diam.NewAVP(avp.FeatureList, avp.Vbit, Vendor3GPP, datatype.Unsigned32(1)),
	}})
	ans.NewAVP(avp.ResultCode, avp.Mbit, 0, datatype.Unsigned32(diam.Success))
	return ans
}

func NewHandlers(cfg *config.Config, store repository.Repository, log *zap.Logger, peers PeerLookup) *Handlers {
	return &Handlers{
		store:       store,
		log:         log,
		originHost:  cfg.HSS.OriginHost,
		originRealm: cfg.HSS.OriginRealm,
		peers:       peers,
		tftHandling: cfg.PCRF.TFTHandling,
		sessions:    make(map[string]*rxSession),
	}
}

func (h *Handlers) applyTFTHandling(tft string) (string, bool) {
	return gx.ApplyTFTHandling(tft, h.tftHandling)
}
