package s6c

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/models"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

const diamResultSuccess = uint32(2001)
const maxMWDEntriesPerSubscriber = 10

// SendALRForIMSI loads any pending MWD for a subscriber and sends an ALR
// request to the relevant SMS-SC peers when the trigger matches the stored
// retry state.
//
// MWD records are NOT deleted here. Deletion happens in ALA() once the SMS-SC
// returns a successful answer (Result-Code 2001). If the peer is unreachable or
// returns an error the MWD remains and will be retried on the next trigger.
func (h *Handlers) SendALRForIMSI(imsi string, trigger AlertTrigger) {
	h.sendALRForIMSI(imsi, trigger, nil)
}

func (h *Handlers) SendALRForIMSIWithMaximumAvailability(imsi string, trigger AlertTrigger, maximumUEAvailabilityTime *time.Time) {
	h.sendALRForIMSI(imsi, trigger, maximumUEAvailabilityTime)
}

func (h *Handlers) sendALRForIMSI(imsi string, trigger AlertTrigger, maximumUEAvailabilityTime *time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sub, err := h.store.GetSubscriberByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		return
	}
	if err != nil {
		h.log.Error("s6c: ALR subscriber lookup failed", zap.String("imsi", imsi), zap.Error(err))
		return
	}

	records, err := h.store.GetMWDForIMSI(ctx, imsi)
	if err != nil {
		h.log.Error("s6c: ALR MWD lookup failed", zap.String("imsi", imsi), zap.Error(err))
		return
	}
	if len(records) == 0 {
		return
	}

	for _, mwd := range records {
		if !triggerMatchesMWD(trigger, mwd.MWDStatusFlags) {
			continue
		}

		conn, ok := h.peers.GetConn(mwd.SCOriginHost)
		if !ok {
			h.log.Warn("s6c: ALR skipped — SMS-SC not connected",
				zap.String("imsi", imsi),
				zap.String("sc_origin_host", mwd.SCOriginHost),
				zap.String("trigger", string(trigger)))
			continue
		}

		sid := fmt.Sprintf("%s;%d;alr", h.originHost, time.Now().UnixNano())
		req := diam.NewRequest(cmdALR, AppIDS6c, nil)
		req.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
		req.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
		req.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(h.originHost))
		req.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(h.originRealm))
		req.NewAVP(avp.DestinationHost, avp.Mbit, 0, datatype.DiameterIdentity(mwd.SCOriginHost))
		req.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity(mwd.SCOriginRealm))

		// SC-Address: E.164 of the SMS-SC, BCD-encoded.
		req.NewAVP(avpSCAddress, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.OctetString(encodeMSISDNBytes(mwd.SCAddress)))

		req.NewAVP(avpUserIdentifier, avp.Mbit|avp.Vbit, Vendor3GPP,
			buildUserIdentifierAVP(imsi, sub.MSISDN))

		if payload, err := decodeSMSMICorrelationID(mwd.SMSMICorrelationID); err != nil {
			h.log.Warn("s6c: ALR skipping invalid SMSMI-Correlation-ID",
				zap.String("imsi", imsi),
				zap.String("sc_addr", mwd.SCAddress),
				zap.Error(err))
		} else if len(payload) > 0 {
			req.NewAVP(avpSMSMICorrelationID, avp.Vbit, Vendor3GPP, datatype.OctetString(payload))
		}

		if mwd.AbsentUserDiagnosticSM != nil {
			req.NewAVP(avpAbsentUserDiagnosticSM, avp.Mbit|avp.Vbit, Vendor3GPP,
				datatype.Unsigned32(*mwd.AbsentUserDiagnosticSM))
		}

		if servingNode := buildServingNodeAVP(sub); servingNode != nil {
			req.InsertAVP(servingNode)
			req.NewAVP(avpSMSGMSCAlertEvent, avp.Vbit, Vendor3GPP,
				datatype.Unsigned32(SMSGMSCAlertEventUEAvailableForMTSMS))
		}
		if maximumUEAvailabilityTime != nil {
			req.NewAVP(avpMaximumUEAvailabilityTime, avp.Vbit, Vendor3GPP,
				datatype.Time(*maximumUEAvailabilityTime))
		}

		if _, err := req.WriteTo(conn); err != nil {
			h.log.Error("s6c: ALR send failed",
				zap.String("imsi", imsi),
				zap.String("sc_origin_host", mwd.SCOriginHost),
				zap.String("trigger", string(trigger)),
				zap.Error(err))
			continue
		}

		now := time.Now().UTC()
		triggerName := string(trigger)
		mwd.LastAlertTrigger = &triggerName
		mwd.LastAlertAttemptAt = &now
		mwd.AlertAttemptCount++
		if err := h.store.StoreMWD(ctx, &mwd); err != nil {
			h.log.Warn("s6c: ALR metadata update failed",
				zap.String("imsi", imsi),
				zap.String("sc_addr", mwd.SCAddress),
				zap.Error(err))
		}

		// Record the in-flight session so ALA() can delete MWD on success.
		h.pendingALR.Store(sid, pendingALREntry{imsi: imsi, scAddr: mwd.SCAddress})

		h.log.Info("s6c: ALR sent — awaiting ALA",
			zap.String("imsi", imsi),
			zap.String("sc_origin_host", mwd.SCOriginHost),
			zap.String("sc_addr", mwd.SCAddress),
			zap.String("session_id", sid),
			zap.String("trigger", string(trigger)))
	}
}

// SendALSCForIMSI is a compatibility wrapper for older call sites while the
// broader codebase and docs are still being updated. Attach recovery only
// applies to not-reachable MWD.
func (h *Handlers) SendALSCForIMSI(imsi string) {
	h.SendALRForIMSI(imsi, AlertTriggerAttach)
}

func triggerMatchesMWD(trigger AlertTrigger, statusFlags uint32) bool {
	switch trigger {
	case AlertTriggerAttach, AlertTriggerUserAvailable:
		return statusFlags == 0 || statusFlags&MWDStatusMNRF != 0
	case AlertTriggerMemoryAvailable:
		return statusFlags&MWDStatusMCEF != 0
	default:
		return false
	}
}

// ALA handles an Alert-Service-Centre-Answer returned by the SMS-SC.
// On success (Result-Code 2001) it deletes the pending MWD record.
// On failure the MWD is left in place and will be retried on next ULR.
func (h *Handlers) ALA(conn diam.Conn, msg *diam.Message) {
	var ala ALA
	if err := msg.Unmarshal(&ala); err != nil {
		h.log.Warn("s6c: ALA unmarshal failed", zap.Error(err))
		return
	}

	sidAVP, err := msg.FindAVP(avp.SessionID, 0)
	if err != nil {
		h.log.Warn("s6c: ALA missing Session-Id")
		return
	}
	sid := string(sidAVP.Data.(datatype.UTF8String))

	entry, ok := h.pendingALR.LoadAndDelete(sid)
	if !ok {
		// Not originated by us or already handled.
		h.log.Warn("s6c: ALA for unknown session",
			zap.String("session_id", sid),
			zap.String("origin_host", string(ala.OriginHost)))
		return
	}
	pending := entry.(pendingALREntry)

	resultCode, ok := alaAnswerResultCode(msg)
	if !ok {
		h.log.Warn("s6c: ALA missing Result-Code / Experimental-Result — MWD retained for retry",
			zap.String("imsi", pending.imsi),
			zap.String("sc_addr", pending.scAddr),
			zap.String("session_id", sid),
			zap.String("origin_host", string(ala.OriginHost)))
		return
	}

	if resultCode != diamResultSuccess {
		h.log.Warn("s6c: ALA failure — MWD retained for retry",
			zap.String("imsi", pending.imsi),
			zap.String("sc_addr", pending.scAddr),
			zap.Uint32("result_code", resultCode))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.store.DeleteMWD(ctx, pending.imsi, pending.scAddr); err != nil {
		h.log.Warn("s6c: ALA MWD delete failed",
			zap.String("imsi", pending.imsi),
			zap.String("sc_addr", pending.scAddr),
			zap.Error(err))
		return
	}

	h.log.Info("s6c: ALA success — MWD deleted",
		zap.String("imsi", pending.imsi),
		zap.String("sc_addr", pending.scAddr),
		zap.String("origin_host", string(ala.OriginHost)))
}

// ASA is a compatibility wrapper while dispatch aliases are still mixed.
func (h *Handlers) ASA(conn diam.Conn, msg *diam.Message) {
	h.ALA(conn, msg)
}

func alaAnswerResultCode(msg *diam.Message) (uint32, bool) {
	if a, err := msg.FindAVP(avp.ResultCode, 0); err == nil {
		if rc, ok := a.Data.(datatype.Unsigned32); ok {
			return uint32(rc), true
		}
	}

	if a, err := msg.FindAVP(avp.ExperimentalResult, 0); err == nil {
		if grp, ok := a.Data.(*diam.GroupedAVP); ok {
			for _, child := range grp.AVP {
				if child.Code != avp.ExperimentalResultCode {
					continue
				}
				if rc, ok := child.Data.(datatype.Unsigned32); ok {
					return uint32(rc), true
				}
			}
		}
	}

	return 0, false
}

func buildUserIdentifierAVP(imsi string, msisdn *string) *diam.GroupedAVP {
	children := []*diam.AVP{
		diam.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi)),
	}
	if msisdn != nil {
		children = append(children,
			diam.NewAVP(avpMSISDN, avp.Mbit|avp.Vbit, Vendor3GPP,
				datatype.OctetString(encodeMSISDNBytes(*msisdn))),
		)
	}
	return &diam.GroupedAVP{AVP: children}
}

func buildServingNodeAVP(sub *models.Subscriber) *diam.AVP {
	if sub == nil {
		return nil
	}
	smsRegistered := sub.MMERegisteredForSMS != nil && *sub.MMERegisteredForSMS
	if sub.ServingMME == nil || !smsRegistered {
		return nil
	}

	nodeAVPs := []*diam.AVP{
		diam.NewAVP(avpMMEName, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.DiameterIdentity(*sub.ServingMME)),
	}
	if sub.MMENumberForMTSMS != nil {
		nodeAVPs = append(nodeAVPs,
			diam.NewAVP(avpMMENumberForMTSMS, avp.Vbit, Vendor3GPP,
				datatype.OctetString(encodeMSISDNBytes(*sub.MMENumberForMTSMS))),
		)
	}
	if sub.ServingMMERealm != nil {
		nodeAVPs = append(nodeAVPs,
			diam.NewAVP(avpMMERealm, avp.Vbit, Vendor3GPP,
				datatype.DiameterIdentity(*sub.ServingMMERealm)),
		)
	}

	return diam.NewAVP(avpServingNode, avp.Mbit|avp.Vbit, Vendor3GPP,
		&diam.GroupedAVP{AVP: nodeAVPs})
}

func decodeSMSMICorrelationID(stored *string) ([]byte, error) {
	if stored == nil || *stored == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(*stored)
}
