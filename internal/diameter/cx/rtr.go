package cx

import (
	"fmt"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"
)

const (
	cmdRTR = uint32(304)

	RTRReasonPermanentTermination = 0
	RTRReasonNewServerAssigned    = 1
	RTRReasonServerChange         = 2
	RTRReasonRemoveSCSCF          = 3
)

// SendRTR sends a Registration-Termination-Request to the S-CSCF currently
// serving the IMS subscriber identified by publicIdentity.
func (h *Handlers) SendRTR(publicIdentity, destHost, destRealm string, reasonCode int, reasonInfo string) {
	conn, ok := h.peers.GetConn(destHost)
	if !ok {
		h.log.Warn("cx: RTR skipped — S-CSCF not connected",
			zap.String("identity", publicIdentity), zap.String("scscf", destHost))
		return
	}

	sid := fmt.Sprintf("%s;%d;rtr", h.originHost, time.Now().UnixNano())
	msg := diam.NewRequest(cmdRTR, AppIDCx, nil)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(h.originHost))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(h.originRealm))
	msg.NewAVP(avp.DestinationHost, avp.Mbit, 0, datatype.DiameterIdentity(destHost))
	msg.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity(destRealm))
	msg.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(publicIdentity))

	reasonAVPs := []*diam.AVP{
		diam.NewAVP(avpReasonCode, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Enumerated(reasonCode)),
	}
	if reasonInfo != "" {
		reasonAVPs = append(reasonAVPs,
			diam.NewAVP(avpReasonInfo, avp.Vbit, Vendor3GPP, datatype.UTF8String(reasonInfo)))
	}
	msg.NewAVP(avpDeregistrationReason, avp.Mbit|avp.Vbit, Vendor3GPP,
		&diam.GroupedAVP{AVP: reasonAVPs})

	if _, err := msg.WriteTo(conn); err != nil {
		h.log.Error("cx: RTR send failed",
			zap.String("identity", publicIdentity), zap.String("dest_host", destHost), zap.Error(err))
		return
	}
	h.log.Info("cx: RTR sent",
		zap.String("identity", publicIdentity), zap.String("dest_host", destHost))
}
