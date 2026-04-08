package s6c

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

const diamResultSuccess = uint32(2001)

// SendALSCForIMSI is called by the S6a ULR handler after a successful
// registration. It checks for pending MWD and sends an Alert-Service-Centre
// Request to each SMS-SC that requested notification.
//
// MWD records are NOT deleted here. Deletion happens in ASA() once the SMS-SC
// returns a successful answer (Result-Code 2001). If the peer is unreachable or
// returns an error the MWD remains and will be retried on the next ULR.
func (h *Handlers) SendALSCForIMSI(imsi string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sub, err := h.store.GetSubscriberByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		return
	}
	if err != nil {
		h.log.Error("s6c: ALSC subscriber lookup failed", zap.String("imsi", imsi), zap.Error(err))
		return
	}

	records, err := h.store.GetMWDForIMSI(ctx, imsi)
	if err != nil {
		h.log.Error("s6c: ALSC MWD lookup failed", zap.String("imsi", imsi), zap.Error(err))
		return
	}
	if len(records) == 0 {
		return
	}

	for _, mwd := range records {
		conn, ok := h.peers.GetConn(mwd.SCOriginHost)
		if !ok {
			h.log.Warn("s6c: ALSC skipped — SMS-SC not connected",
				zap.String("imsi", imsi), zap.String("sc_origin_host", mwd.SCOriginHost))
			continue
		}

		sid := fmt.Sprintf("%s;%d;alsc", h.originHost, time.Now().UnixNano())
		req := diam.NewRequest(cmdALSC, AppIDS6c, nil)
		req.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
		req.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
		req.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(h.originHost))
		req.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(h.originRealm))
		req.NewAVP(avp.DestinationHost, avp.Mbit, 0, datatype.DiameterIdentity(mwd.SCOriginHost))
		req.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity(mwd.SCOriginRealm))

		// SC-Address: E.164 of the SMS-SC, BCD-encoded.
		req.NewAVP(avpSCAddress, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.OctetString(encodeMSISDNBytes(mwd.SCAddress)))

		// User-Name: IMSI of the now-reachable subscriber.
		req.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))

		// MSISDN (optional).
		if sub.MSISDN != nil {
			req.NewAVP(avpMSISDN, avp.Mbit|avp.Vbit, Vendor3GPP,
				datatype.OctetString(encodeMSISDNBytes(*sub.MSISDN)))
		}

		if _, err := req.WriteTo(conn); err != nil {
			h.log.Error("s6c: ALSC send failed",
				zap.String("imsi", imsi),
				zap.String("sc_origin_host", mwd.SCOriginHost),
				zap.Error(err))
			continue
		}

		// Record the in-flight session so ASA() can delete MWD on success.
		h.pendingALSC.Store(sid, pendingALSCEntry{imsi: imsi, scAddr: mwd.SCAddress})

		h.log.Info("s6c: ALSC sent — awaiting ASA",
			zap.String("imsi", imsi),
			zap.String("sc_origin_host", mwd.SCOriginHost),
			zap.String("sc_addr", mwd.SCAddress),
			zap.String("session_id", sid))
	}
}

// ASA handles an Alert-Service-Centre-Answer returned by the SMS-SC.
// On success (Result-Code 2001) it deletes the pending MWD record.
// On failure the MWD is left in place and will be retried on next ULR.
func (h *Handlers) ASA(conn diam.Conn, msg *diam.Message) {
	var asa ASA
	if err := msg.Unmarshal(&asa); err != nil {
		h.log.Warn("s6c: ASA unmarshal failed", zap.Error(err))
		return
	}

	sidAVP, err := msg.FindAVP(avp.SessionID, 0)
	if err != nil {
		h.log.Warn("s6c: ASA missing Session-Id")
		return
	}
	sid := string(sidAVP.Data.(datatype.UTF8String))

	entry, ok := h.pendingALSC.LoadAndDelete(sid)
	if !ok {
		// Not originated by us or already handled.
		h.log.Warn("s6c: ASA for unknown session",
			zap.String("session_id", sid),
			zap.String("origin_host", string(asa.OriginHost)))
		return
	}
	pending := entry.(pendingALSCEntry)

	if uint32(asa.ResultCode) != diamResultSuccess {
		h.log.Warn("s6c: ASA failure — MWD retained for retry",
			zap.String("imsi", pending.imsi),
			zap.String("sc_addr", pending.scAddr),
			zap.Uint32("result_code", uint32(asa.ResultCode)))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.store.DeleteMWD(ctx, pending.imsi, pending.scAddr); err != nil {
		h.log.Warn("s6c: ASA MWD delete failed",
			zap.String("imsi", pending.imsi),
			zap.String("sc_addr", pending.scAddr),
			zap.Error(err))
		return
	}

	h.log.Info("s6c: ASA success — MWD deleted",
		zap.String("imsi", pending.imsi),
		zap.String("sc_addr", pending.scAddr),
		zap.String("origin_host", string(asa.OriginHost)))
}
