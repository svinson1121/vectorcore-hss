package s6a

import (
	"context"
	"fmt"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

const cmdIDR = uint32(319)

// SendIDR sends an Insert-Subscriber-Data-Request to the MME currently serving
// the subscriber. Call this after updating a subscriber's profile so the MME
// gets the new data without requiring a fresh ULR.
func (h *Handlers) SendIDR(imsi string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := h.store.GetSubscriberByIMSI(ctx, imsi)
	if err == repository.ErrNotFound || sub.ServingMME == nil {
		return
	}
	if err != nil {
		h.log.Error("s6a: IDR lookup failed", zap.String("imsi", imsi), zap.Error(err))
		return
	}

	destHost := *sub.ServingMME
	conn, ok := h.peers.GetConn(destHost)
	if !ok {
		h.log.Warn("s6a: IDR skipped — MME not connected",
			zap.String("imsi", imsi), zap.String("mme", destHost))
		return
	}

	destRealm := ""
	if sub.ServingMMERealm != nil {
		destRealm = *sub.ServingMMERealm
	}

	sd, err := h.buildSubscriptionData(ctx, sub)
	if err != nil {
		h.log.Error("s6a: IDR build subscription data failed",
			zap.String("imsi", imsi), zap.Error(err))
		return
	}

	sid := fmt.Sprintf("%s;%d;idr", h.originHost, time.Now().UnixNano())
	msg := diam.NewRequest(cmdIDR, AppIDS6a, nil)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(h.originHost))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(h.originRealm))
	msg.NewAVP(avp.DestinationHost, avp.Mbit, 0, datatype.DiameterIdentity(destHost))
	msg.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity(destRealm))
	msg.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))
	appendSubscriptionDataAVPs(msg, sd)

	if _, err := msg.WriteTo(conn); err != nil {
		h.log.Error("s6a: IDR send failed",
			zap.String("imsi", imsi), zap.String("dest_host", destHost), zap.Error(err))
		return
	}
	h.log.Info("s6a: IDR sent", zap.String("imsi", imsi), zap.String("dest_host", destHost))
}
