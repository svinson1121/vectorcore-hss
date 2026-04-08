package s6a

import (
	"fmt"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"
)

const (
	cmdDSR      = uint32(320)
	avpDSRFlags = uint32(1421)
)

// SendDSR sends a Delete-Subscriber-Data-Request to the MME currently serving
// the subscriber. dsrFlags is a bitmask indicating which data to delete
// (3GPP TS 29.272 §7.3.25). Pass 0 to delete all subscription data.
func (h *Handlers) SendDSR(imsi, destHost, destRealm string, dsrFlags uint32) {
	conn, ok := h.peers.GetConn(destHost)
	if !ok {
		h.log.Warn("s6a: DSR skipped — MME not connected",
			zap.String("imsi", imsi), zap.String("mme", destHost))
		return
	}

	sid := fmt.Sprintf("%s;%d;dsr", h.originHost, time.Now().UnixNano())
	msg := diam.NewRequest(cmdDSR, AppIDS6a, nil)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(h.originHost))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(h.originRealm))
	msg.NewAVP(avp.DestinationHost, avp.Mbit, 0, datatype.DiameterIdentity(destHost))
	msg.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity(destRealm))
	msg.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))
	msg.NewAVP(avpDSRFlags, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(dsrFlags))

	if _, err := msg.WriteTo(conn); err != nil {
		h.log.Error("s6a: DSR send failed",
			zap.String("imsi", imsi), zap.String("dest_host", destHost), zap.Error(err))
		return
	}
	h.log.Info("s6a: DSR sent",
		zap.String("imsi", imsi), zap.String("dest_host", destHost))
}
