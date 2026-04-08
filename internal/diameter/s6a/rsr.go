package s6a

import (
	"fmt"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"
)

const cmdRSR = uint32(322)

// SendRSR sends a Reset-Request to the MME for a specific subscriber.
// Called after a SQN re-sync to invalidate stale auth vectors at the MME.
func (h *Handlers) SendRSR(imsi, destHost, destRealm string) {
	conn, ok := h.peers.GetConn(destHost)
	if !ok {
		h.log.Warn("s6a: RSR skipped — MME not connected",
			zap.String("imsi", imsi), zap.String("mme", destHost))
		return
	}

	sid := fmt.Sprintf("%s;%d;rsr", h.originHost, time.Now().UnixNano())
	msg := diam.NewRequest(cmdRSR, AppIDS6a, nil)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(h.originHost))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(h.originRealm))
	msg.NewAVP(avp.DestinationHost, avp.Mbit, 0, datatype.DiameterIdentity(destHost))
	msg.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity(destRealm))
	msg.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))

	if _, err := msg.WriteTo(conn); err != nil {
		h.log.Error("s6a: RSR send failed",
			zap.String("imsi", imsi), zap.String("dest_host", destHost), zap.Error(err))
		return
	}
	h.log.Info("s6a: RSR sent", zap.String("imsi", imsi), zap.String("dest_host", destHost))
}
