package swx

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

	avpDeregistrationReason = uint32(615)
	avpReasonCode           = uint32(616)
	avpReasonInfo           = uint32(617)
)

// SendRTR sends a Registration-Termination-Request to the AAA Server serving
// the non-3GPP subscriber.
func (h *Handlers) SendRTR(imsi, destHost, destRealm string, reasonCode int, reasonInfo string) {
	conn, ok := h.peers.GetConn(destHost)
	if !ok {
		h.log.Warn("swx: RTR skipped — AAA server not connected",
			zap.String("imsi", imsi), zap.String("aaa", destHost))
		return
	}

	sid := fmt.Sprintf("%s;%d;rtr", h.originHost, time.Now().UnixNano())
	msg := diam.NewRequest(cmdRTR, AppIDSWx, nil)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(h.originHost))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(h.originRealm))
	msg.NewAVP(avp.DestinationHost, avp.Mbit, 0, datatype.DiameterIdentity(destHost))
	msg.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity(destRealm))
	msg.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))

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
		h.log.Error("swx: RTR send failed",
			zap.String("imsi", imsi), zap.String("dest_host", destHost), zap.Error(err))
		return
	}
	h.log.Info("swx: RTR sent", zap.String("imsi", imsi), zap.String("dest_host", destHost))
}
