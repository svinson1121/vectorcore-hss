package cx

import (
	"fmt"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"
)

const cmdPPR = uint32(305)

// SendPPR sends a Push-Profile-Request to the S-CSCF serving the IMS
// subscriber. Call this after updating an IMS subscriber's IFC profile.
func (h *Handlers) SendPPR(publicIdentity, destHost, destRealm, userData string) {
	conn, ok := h.peers.GetConn(destHost)
	if !ok {
		h.log.Warn("cx: PPR skipped — S-CSCF not connected",
			zap.String("identity", publicIdentity), zap.String("scscf", destHost))
		return
	}

	sid := fmt.Sprintf("%s;%d;ppr", h.originHost, time.Now().UnixNano())
	msg := diam.NewRequest(cmdPPR, AppIDCx, nil)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(h.originHost))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(h.originRealm))
	msg.NewAVP(avp.DestinationHost, avp.Mbit, 0, datatype.DiameterIdentity(destHost))
	msg.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity(destRealm))
	msg.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(publicIdentity))
	if userData != "" {
		msg.NewAVP(avpUserData, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.OctetString(userData))
	}

	if _, err := msg.WriteTo(conn); err != nil {
		h.log.Error("cx: PPR send failed",
			zap.String("identity", publicIdentity), zap.String("dest_host", destHost), zap.Error(err))
		return
	}
	h.log.Info("cx: PPR sent",
		zap.String("identity", publicIdentity), zap.String("dest_host", destHost))
}
