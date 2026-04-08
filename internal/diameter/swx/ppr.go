package swx

import (
	"fmt"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"
)

const cmdPPR = uint32(305)

// SendPPR sends a Push-Profile-Request to the AAA Server serving the
// non-3GPP subscriber, pushing updated Non-3GPP-User-Data.
func (h *Handlers) SendPPR(imsi, destHost, destRealm string, accessAllowed bool) {
	conn, ok := h.peers.GetConn(destHost)
	if !ok {
		h.log.Warn("swx: PPR skipped — AAA server not connected",
			zap.String("imsi", imsi), zap.String("aaa", destHost))
		return
	}

	accessStatus := Non3GPPAccessAllowed
	if !accessAllowed {
		accessStatus = Non3GPPAccessBarred
	}

	sid := fmt.Sprintf("%s;%d;ppr", h.originHost, time.Now().UnixNano())
	msg := diam.NewRequest(cmdPPR, AppIDSWx, nil)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(h.originHost))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(h.originRealm))
	msg.NewAVP(avp.DestinationHost, avp.Mbit, 0, datatype.DiameterIdentity(destHost))
	msg.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity(destRealm))
	msg.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))
	msg.NewAVP(avpNon3GPPUserData, avp.Mbit|avp.Vbit, Vendor3GPP,
		&diam.GroupedAVP{AVP: []*diam.AVP{
			diam.NewAVP(avpNon3GPPIPAccess, avp.Mbit|avp.Vbit, Vendor3GPP,
				datatype.Enumerated(accessStatus)),
		}})

	if _, err := msg.WriteTo(conn); err != nil {
		h.log.Error("swx: PPR send failed",
			zap.String("imsi", imsi), zap.String("dest_host", destHost), zap.Error(err))
		return
	}
	h.log.Info("swx: PPR sent", zap.String("imsi", imsi), zap.String("dest_host", destHost))
}
