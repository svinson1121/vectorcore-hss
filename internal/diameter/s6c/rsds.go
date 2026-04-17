package s6c

import (
	"encoding/base64"
	"context"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/diameter/avputil"
	"github.com/svinson1121/vectorcore-hss/internal/models"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

// RDSMR handles a Report-SM-Delivery-Status-Request from an SMS-SC.
//
// Behavior per TS 29.338 §5.3.2.4:
//
//   - SM-Delivery-Cause = SuccessfulTransfer (2): clear any existing MWD record,
//     return success. No ALR will be sent.
//   - SM-Delivery-Cause = AbsentUser (1): store MWD with MNRF flag; HSS will send
//     ALR once the subscriber registers via ULR or is reported reachable via NOR.
//   - SM-Delivery-Cause = MemoryCapacityExceeded (0): store MWD with MCEF flag.
//   - SM-Delivery-Outcome absent: default to AbsentUser / MNRF.
func (h *Handlers) RDSMR(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
	var req RDR
	if err := msg.Unmarshal(&req); err != nil {
		h.log.Error("s6c: RSDS unmarshal failed", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, "", h.originHost, h.originRealm, diam.UnableToComply), err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	userIdentifierIMSI, userIdentifierMSISDN := parseUserIdentifier(msg)

	// Resolve subscriber — Rel-15 User-Identifier preferred, then legacy direct AVPs.
	var sub *models.Subscriber
	var imsi string
	var err error
	if userIdentifierIMSI != "" {
		imsi = userIdentifierIMSI
		sub, err = h.store.GetSubscriberByIMSI(ctx, imsi)
		if err == repository.ErrNotFound {
			h.log.Warn("s6c: RSDS unknown subscriber", zap.String("imsi", imsi))
			return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUserUnknown), err
		}
		if err != nil {
			h.log.Error("s6c: RSDS store error", zap.Error(err))
			return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
		}
	} else if userIdentifierMSISDN != "" {
		sub, err = h.store.GetSubscriberByMSISDN(ctx, userIdentifierMSISDN)
		if err == repository.ErrNotFound {
			h.log.Warn("s6c: RSDS unknown subscriber", zap.String("msisdn", userIdentifierMSISDN))
			return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUserUnknown), err
		}
		if err != nil {
			h.log.Error("s6c: RSDS store error", zap.Error(err))
			return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
		}
		imsi = sub.IMSI
	} else if string(req.UserName) != "" {
		imsi = string(req.UserName)
		sub, err = h.store.GetSubscriberByIMSI(ctx, imsi)
		if err == repository.ErrNotFound {
			h.log.Warn("s6c: RSDS unknown subscriber", zap.String("imsi", imsi))
			return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUserUnknown), err
		}
		if err != nil {
			h.log.Error("s6c: RSDS store error", zap.Error(err))
			return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
		}
	} else {
		msisdn := decodeMSISDN(req.MSISDN)
		sub, err = h.store.GetSubscriberByMSISDN(ctx, msisdn)
		if err == repository.ErrNotFound {
			h.log.Warn("s6c: RSDS unknown subscriber", zap.String("msisdn", msisdn))
			return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUserUnknown), err
		}
		if err != nil {
			h.log.Error("s6c: RSDS store error", zap.Error(err))
			return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
		}
		imsi = sub.IMSI
	}

	scAddr := decodeMSISDN(req.SCAddress)
	scOriginHost := string(req.OriginHost)
	scOriginRealm := string(req.OriginRealm)
	mti := int(req.SMRPMTI)
	rdrFlags := uint32(req.RDRFlags)

	outcome := parseDeliveryOutcome(msg)

	// SuccessfulTransfer — clear MWD, no notification needed.
	if outcome.Cause == SMDeliveryCauseSuccessfulTransfer {
		if err := h.store.DeleteMWD(ctx, imsi, scAddr); err != nil {
			h.log.Warn("s6c: RSDS clear MWD failed",
				zap.String("imsi", imsi), zap.String("sc_addr", scAddr), zap.Error(err))
		}
		h.log.Info("s6c: RSDS successful transfer — MWD cleared",
			zap.String("imsi", imsi), zap.String("sc_addr", scAddr))
		ans := avputil.ConstructSuccessAnswer(msg, req.SessionID, h.originHost, h.originRealm, AppIDS6c)
		appendUserIdentifier(ans, imsi, sub.MSISDN)
		return ans, nil
	}

	if rdrFlags&RDRFlagsSingleAttemptDelivery != 0 {
		h.log.Info("s6c: RSDS single-attempt-delivery set — skipping MWD store",
			zap.String("imsi", imsi),
			zap.String("sc_addr", scAddr),
			zap.Uint32("rdr_flags", rdrFlags),
			zap.Int32("delivery_cause", outcome.Cause))
		ans := avputil.ConstructSuccessAnswer(msg, req.SessionID, h.originHost, h.originRealm, AppIDS6c)
		appendUserIdentifier(ans, imsi, sub.MSISDN)
		return ans, nil
	}

	records, err := h.store.GetMWDForIMSI(ctx, imsi)
	if err != nil {
		h.log.Error("s6c: RSDS MWD lookup failed", zap.String("imsi", imsi), zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
	}
	if shouldRejectMWDInsert(records, scAddr) {
		h.log.Warn("s6c: RSDS MWD list full",
			zap.String("imsi", imsi),
			zap.String("sc_addr", scAddr),
			zap.Int("existing_records", len(records)))
		return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorMWDListFull), nil
	}

	// Absent or memory-full — store MWD with the appropriate status flags.
	var statusFlags uint32
	var mwdStatusBit uint32
	switch outcome.Cause {
	case SMDeliveryCauseMemoryCapacityExceeded:
		statusFlags = MWDStatusMCEF
		mwdStatusBit = MWDStatusMCEF
	default: // AbsentUser or no outcome present
		statusFlags = MWDStatusMNRF
		mwdStatusBit = MWDStatusMNRF
	}

	mwd := &models.MessageWaitingData{
		IMSI:                   imsi,
		SCAddress:              scAddr,
		SCOriginHost:           scOriginHost,
		SCOriginRealm:          scOriginRealm,
		SMRPMTI:                mti,
		MWDStatusFlags:         statusFlags,
		SMSMICorrelationID:     extractSMSMICorrelationID(msg),
		AbsentUserDiagnosticSM: uint32Ptr(outcome.AbsentUserDiagnostic),
	}
	if err := h.store.StoreMWD(ctx, mwd); err != nil {
		h.log.Error("s6c: RSDS store MWD failed",
			zap.String("imsi", imsi), zap.String("sc_addr", scAddr), zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
	}

	h.log.Info("s6c: RSDS MWD stored",
		zap.String("imsi", imsi),
		zap.String("sc_addr", scAddr),
		zap.String("sc_origin_host", scOriginHost),
		zap.Int32("delivery_cause", outcome.Cause))

	ans := avputil.ConstructSuccessAnswer(msg, req.SessionID, h.originHost, h.originRealm, AppIDS6c)
	appendUserIdentifier(ans, imsi, sub.MSISDN)
	ans.NewAVP(avpMWDStatus, avp.Vbit, Vendor3GPP, datatype.Unsigned32(mwdStatusBit))
	return ans, nil
}

// parseDeliveryOutcome extracts the SM-Delivery-Cause from the SM-Delivery-Outcome
// grouped AVP (code 3316, vendor 10415). It checks each node-specific sub-AVP
// (MME, SGSN, MSC, IP-SM-GW) and returns the first cause found.
// Returns Cause = -1 if the AVP is absent, which the caller treats as AbsentUser.
func parseDeliveryOutcome(msg *diam.Message) SMDeliveryOutcomeResult {
	outerAVP, err := msg.FindAVP(avpSMDeliveryOutcome, Vendor3GPP)
	if err != nil {
		return SMDeliveryOutcomeResult{Cause: -1}
	}
	outer, ok := outerAVP.Data.(*diam.GroupedAVP)
	if !ok {
		return SMDeliveryOutcomeResult{Cause: -1}
	}

	// Each node-outcome sub-AVP (MME/SGSN/MSC/IP-SM-GW) is itself grouped,
	// containing SM-Delivery-Cause and optionally Absent-User-Diagnostic-SM.
	nodeOutcomeCodes := []uint32{
		avpMMEDeliveryOutcome,
		avpSGSNDeliveryOutcome,
		// MSC (3319) and IP-SM-GW (3320) not declared as named consts but share same structure
		uint32(3319),
		uint32(3320),
	}
	for _, sub := range outer.AVP {
		for _, code := range nodeOutcomeCodes {
			if sub.Code != code {
				continue
			}
			nodeGrouped, ok := sub.Data.(*diam.GroupedAVP)
			if !ok {
				continue
			}
			result := SMDeliveryOutcomeResult{Cause: -1}
			for _, inner := range nodeGrouped.AVP {
				switch inner.Code {
				case avpSMDeliveryCause:
					if v, ok := inner.Data.(datatype.Enumerated); ok {
						result.Cause = int32(v)
					}
				case avpAbsentUserDiagnosticSM:
					if v, ok := inner.Data.(datatype.Unsigned32); ok {
						result.AbsentUserDiagnostic = uint32(v)
					}
				}
			}
			if result.Cause != -1 {
				return result
			}
		}
	}
	return SMDeliveryOutcomeResult{Cause: -1}
}

func uint32Ptr(v uint32) *uint32 {
	if v == 0 {
		return nil
	}
	return &v
}

func shouldRejectMWDInsert(records []models.MessageWaitingData, scAddr string) bool {
	for _, record := range records {
		if record.SCAddress == scAddr {
			return false
		}
	}
	return len(records) >= maxMWDEntriesPerSubscriber
}

func extractSMSMICorrelationID(msg *diam.Message) *string {
	if msg == nil {
		return nil
	}
	a, err := msg.FindAVP(avpSMSMICorrelationID, Vendor3GPP)
	if err != nil {
		return nil
	}

	switch data := a.Data.(type) {
	case datatype.OctetString:
		if len(data) == 0 {
			return nil
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(data))
		return &encoded
	case *diam.GroupedAVP:
		payload := data.Serialize()
		if len(payload) == 0 {
			return nil
		}
		encoded := base64.StdEncoding.EncodeToString(payload)
		return &encoded
	default:
		return nil
	}
}

func parseUserIdentifier(msg *diam.Message) (imsi string, msisdn string) {
	if msg == nil {
		return "", ""
	}
	a, err := msg.FindAVP(avpUserIdentifier, Vendor3GPP)
	if err != nil {
		return "", ""
	}
	group, ok := a.Data.(*diam.GroupedAVP)
	if !ok {
		return "", ""
	}
	for _, child := range group.AVP {
		switch {
		case child.Code == avp.UserName && child.VendorID == 0:
			if v, ok := child.Data.(datatype.UTF8String); ok {
				imsi = string(v)
			}
		case child.Code == avpMSISDN && child.VendorID == Vendor3GPP:
			if v, ok := child.Data.(datatype.OctetString); ok {
				msisdn = decodeMSISDN(v)
			}
		}
	}
	return imsi, msisdn
}

func appendUserIdentifier(ans *diam.Message, imsi string, msisdn *string) {
	if ans == nil || imsi == "" {
		return
	}
	ans.NewAVP(avpUserIdentifier, avp.Mbit|avp.Vbit, Vendor3GPP, buildUserIdentifierAVP(imsi, msisdn))
}
