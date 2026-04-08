package s6a

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/diameter/avputil"
	"github.com/svinson1121/vectorcore-hss/internal/geored"
	"github.com/svinson1121/vectorcore-hss/internal/models"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

func (h *Handlers) ULR(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
	var ulr ULR
	if err := msg.Unmarshal(&ulr); err != nil {
		h.log.Error("s6a: ULR unmarshal failed", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, "", h.originHost, h.originRealm, diam.UnableToComply), err
	}

	imsi := string(ulr.UserName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := h.store.GetSubscriberByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		h.log.Warn("s6a: ULR unknown IMSI", zap.String("imsi", imsi))
		return avputil.ConstructFailureAnswer(msg, ulr.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUserUnknown), err
	}
	if err != nil {
		return avputil.ConstructFailureAnswer(msg, ulr.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUserUnknown), err
	}
	if sub.Enabled != nil && !*sub.Enabled {
		h.log.Warn("s6a: ULR subscriber disabled", zap.String("imsi", imsi))
		return avputil.ConstructFailureAnswer(msg, ulr.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUnknownEPSSubscription), nil
	}

	if err := h.checkRoaming(ctx, sub, []byte(ulr.VisitedPLMNID)); err != nil {
		h.log.Warn("s6a: ULR roaming denied", zap.String("imsi", imsi), zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, ulr.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorRoamingNotAllowed), nil
	}

	origHost := string(ulr.OriginHost)
	origRealm := string(ulr.OriginRealm)
	peer := origHost + ";" + h.originHost
	ts := time.Now().UTC()
	smsRegReq, smsRegReqPresent := findSMSRegisterRequest(msg)
	mmeNumberForMTSMS, mmeNumberPresent := findMMENumberForMTSMS(msg)
	smsOnlyInd := (uint32(ulr.ULRFlags) & ULRFlagSMSOnlyIndication) != 0
	smsSupported := supportsSMSInMME(msg) || smsRegReqPresent || smsOnlyInd
	smsRequested := mmeNumberPresent && (smsRegReqPresent || smsOnlyInd)
	smsAccepted := (uint32(ulr.ULRFlags)&ULRFlagS6aIndicator) != 0 && smsSupported && smsRequested

	if ulr.RATType == ratTypeGERAN || ulr.RATType == ratTypeUTRAN {
		// GERAN/UTRAN attach: update the serving SGSN instead of the MME.
		_ = h.store.UpdateServingSGSN(ctx, imsi, &repository.ServingSGSNUpdate{
			ServingSGSN: &origHost, Timestamp: &ts,
		})
		h.pub.PublishServingSGSN(geored.PayloadServingSGSN{IMSI: imsi, ServingSGSN: &origHost, Timestamp: &ts})
	} else {
		// If CLR is enabled and the subscriber is already attached to a *different* MME,
		// notify the old MME before updating the record.
		if h.clrEnabled && sub.ServingMME != nil && *sub.ServingMME != origHost {
			oldMME := *sub.ServingMME
			oldRealm := ""
			if sub.ServingMMERealm != nil {
				oldRealm = *sub.ServingMMERealm
			}
			if oldConn, ok := h.peers.GetConn(oldMME); ok {
				go h.SendCLR(oldConn, imsi, oldMME, oldRealm)
			} else {
				h.log.Warn("s6a: CLR skipped — old MME not connected",
					zap.String("imsi", imsi), zap.String("old_mme", oldMME))
			}
		}

		mmeUpdate := &repository.ServingMMEUpdate{
			ServingMME: &origHost, Realm: &origRealm, Peer: &peer, Timestamp: &ts,
		}
		if smsAccepted {
			smsAcceptedVal := true
			mmeUpdate.MMERegisteredForSMS = &smsAcceptedVal
			mmeUpdate.MMENumberForMTSMS = &mmeNumberForMTSMS
			mmeUpdate.SMSRegistrationTimestamp = &ts
			if smsRegReqPresent {
				req := int(smsRegReq)
				mmeUpdate.SMSRegisterRequest = &req
			}
		} else if sub.MMERegisteredForSMS != nil || sub.MMENumberForMTSMS != nil || sub.SMSRegisterRequest != nil || sub.SMSRegistrationTimestamp != nil {
			smsAcceptedVal := false
			mmeUpdate.MMERegisteredForSMS = &smsAcceptedVal
			mmeUpdate.MMENumberForMTSMS = nil
			mmeUpdate.SMSRegisterRequest = nil
			mmeUpdate.SMSRegistrationTimestamp = nil
		}
		uliBytes := []byte(ulr.UserLocationInfo)
		h.log.Debug("s6a: ULR ULI", zap.String("imsi", imsi),
			zap.Int("uli_len", len(uliBytes)),
			zap.Binary("uli_raw", uliBytes))
		if uli, ok := parseULI(uliBytes); ok {
			locTs := time.Now().UTC()
			mmeUpdate.MCC = &uli.MCC
			mmeUpdate.MNC = &uli.MNC
			mmeUpdate.TAC = &uli.TAC
			mmeUpdate.ENodeBID = &uli.ENodeBID
			mmeUpdate.CellID = &uli.CellID
			mmeUpdate.ECI = &uli.ECI
			mmeUpdate.LocationTimestamp = &locTs
			h.log.Debug("s6a: ULR location",
				zap.String("imsi", imsi),
				zap.String("mcc", uli.MCC),
				zap.String("mnc", uli.MNC),
				zap.String("tac", uli.TAC),
				zap.String("enodeb_id", uli.ENodeBID),
				zap.String("cell_id", uli.CellID),
			)
		}
		if err := h.store.UpdateServingMME(ctx, imsi, mmeUpdate); err != nil {
			h.log.Error("s6a: ULR update serving MME failed",
				zap.String("imsi", imsi),
				zap.Error(err))
			return avputil.ConstructFailureAnswer(msg, ulr.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
		}
		h.pub.PublishServingMME(geored.PayloadServingMME{
			IMSI:              imsi,
			ServingMME:        mmeUpdate.ServingMME,
			ServingMMERealm:   mmeUpdate.Realm,
			ServingMMEPeer:    mmeUpdate.Peer,
			Timestamp:         mmeUpdate.Timestamp,
			MCC:               mmeUpdate.MCC,
			MNC:               mmeUpdate.MNC,
			TAC:               mmeUpdate.TAC,
			ENodeBID:          mmeUpdate.ENodeBID,
			CellID:            mmeUpdate.CellID,
			ECI:               mmeUpdate.ECI,
			LocationTimestamp: mmeUpdate.LocationTimestamp,
		})
	}

	sd, err := h.buildSubscriptionData(ctx, sub)
	if err != nil {
		return avputil.ConstructFailureAnswer(msg, ulr.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUnknownEPSSubscription), err
	}

	h.log.Info("s6a: ULR success", zap.String("imsi", imsi))

	// Notify S6c layer so any pending Message Waiting Data triggers ALSC.
	if h.onRegister != nil {
		go h.onRegister(imsi)
	}

	ulaFlags := ULAFlagSeparationIndication
	if smsAccepted {
		ulaFlags |= ULAFlagMMERegisteredForSMS
	}

	h.log.Info("s6a: ULR SMS registration",
		zap.String("imsi", imsi),
		zap.Bool("supported", smsSupported),
		zap.Bool("requested", smsRequested),
		zap.Bool("accepted", smsAccepted),
		zap.Bool("sms_only_indication", smsOnlyInd),
		zap.Bool("mme_number_present", mmeNumberPresent),
	)

	return buildULA(msg, ulr.SessionID, h.originHost, h.originRealm, sd, ulaFlags), nil
}

func (h *Handlers) buildSubscriptionData(ctx context.Context, sub *models.Subscriber) (*SubscriptionData, error) {
	sd := &SubscriptionData{
		SubscriberStatus:              0,
		NetworkAccessMode:             int32(sub.NAM),
		AccessRestrictionData:         ardValue(sub.AccessRestrictionData),
		SubscribedPeriodicRAUTAUTimer: uint32(sub.SubscribedRAUTAUTimer),
		IMSVoiceOverPSSessions:        -1, // omit unless ims_subscriber record exists
		AMBR: AMBR{
			MaxRequestedBandwidthDL: uint32(sub.UEAMBRDown),
			MaxRequestedBandwidthUL: uint32(sub.UEAMBRUp),
		},
	}
	if sub.MSISDN != nil {
		if b, err := encodeMSISDN(*sub.MSISDN); err == nil {
			sd.MSISDN = datatype.OctetString(b)
		}
	}

	// IMS-Voice-Over-PS-Sessions-Support (AVP 1291): set SUPPORTED when an
	// ims_subscriber record exists for this IMSI, NOT_SUPPORTED otherwise.
	_, err := h.store.GetIMSSubscriberByIMSI(ctx, sub.IMSI)
	if err == nil {
		sd.IMSVoiceOverPSSessions = 1 // SUPPORTED
	} else {
		sd.IMSVoiceOverPSSessions = 0 // NOT_SUPPORTED
	}

	defaultStr := strconv.Itoa(sub.DefaultAPN)
	ordered := []string{defaultStr}
	for _, id := range strings.Split(sub.APNList, ",") {
		id = strings.TrimSpace(id)
		if id != defaultStr && id != "" {
			ordered = append(ordered, id)
		}
	}

	// ContextIdentifier in APN-Configuration-Profile must point to the default
	// APN's ContextIdentifier (3GPP TS 29.272 §7.3.34). The default APN is
	// always first in ordered, so its ContextIdentifier will be 1.
	profile := APNConfigurationProfile{ContextIdentifier: 1, AllAPNConfigurationsIncludedIndicator: 0}
	for i, idStr := range ordered {
		apnID, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			continue
		}
		a, err := h.store.GetAPNByID(ctx, apnID)
		if err != nil {
			continue
		}
		profile.APNConfiguration = append(profile.APNConfiguration, APNConfiguration{
			ContextIdentifier: uint32(i + 1),
			ServiceSelection:  a.APN,
			PDNType:           int32(a.IPVersion),
			EPSSubscribedQoSProfile: EPSSubscribedQoSProfile{
				QoSClassIdentifier: int32(a.QCI),
				AllocationRetentionPriority: AllocationRetentionPriority{
					PriorityLevel:           uint32(a.ARPPriority),
					PreemptionCapability:    boolToPreemption(a.ARPPreemptionCapability, false),
					PreemptionVulnerability: boolToPreemption(a.ARPPreemptionVulnerability, true),
				},
			},
			AMBR: AMBR{
				MaxRequestedBandwidthDL: uint32(a.APNAMBRDown),
				MaxRequestedBandwidthUL: uint32(a.APNAMBRUp),
			},
			TGPPChargingCharacteristics: a.ChargingCharacteristics,
		})
	}
	sd.APNConfigurationProfile = profile
	return sd, nil
}

func buildULA(req *diam.Message, sessionID datatype.UTF8String, originHost, originRealm string, sd *SubscriptionData, ulaFlags uint32) *diam.Message {
	ans := avputil.ConstructSuccessAnswer(req, sessionID, originHost, originRealm, AppIDS6a)
	ans.NewAVP(avp.ULAFlags, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(ulaFlags))
	appendSubscriptionDataAVPs(ans, sd)
	return ans
}

func findSMSRegisterRequest(msg *diam.Message) (int32, bool) {
	for _, a := range msg.AVP {
		if a.Code != avpSMSRegisterRequest || a.VendorID != Vendor3GPP {
			continue
		}
		switch v := a.Data.(type) {
		case datatype.Enumerated:
			return int32(v), true
		case datatype.Unsigned32:
			return int32(v), true
		default:
			return 0, false
		}
	}
	return 0, false
}

func findMMENumberForMTSMS(msg *diam.Message) (string, bool) {
	for _, a := range msg.AVP {
		if a.Code != avpMMENumberForMTSMS || a.VendorID != Vendor3GPP {
			continue
		}
		if v, ok := a.Data.(datatype.OctetString); ok {
			return decodeTBCDString([]byte(v)), true
		}
		return "", false
	}
	return "", false
}

func supportsSMSInMME(msg *diam.Message) bool {
	for _, a := range msg.AVP {
		if a.Code != avp.SupportedFeatures || a.VendorID != Vendor3GPP {
			continue
		}
		grouped, ok := a.Data.(*diam.GroupedAVP)
		if !ok {
			continue
		}
		var featureListID uint32
		var featureList uint32
		var haveID bool
		var haveList bool
		for _, child := range grouped.AVP {
			switch child.Code {
			case avp.FeatureListID:
				switch v := child.Data.(type) {
				case datatype.Unsigned32:
					featureListID = uint32(v)
					haveID = true
				case datatype.Enumerated:
					featureListID = uint32(v)
					haveID = true
				}
			case avp.FeatureList:
				switch v := child.Data.(type) {
				case datatype.Unsigned32:
					featureList = uint32(v)
					haveList = true
				case datatype.Enumerated:
					featureList = uint32(v)
					haveList = true
				}
			}
		}
		if haveID && haveList && featureListID == FeatureListIDSMSInMME && (featureList&FeatureBitSMSInMME) != 0 {
			return true
		}
	}
	return false
}

func encodeMSISDN(msisdn string) ([]byte, error) {
	msisdn = strings.TrimPrefix(msisdn, "msisdn-")
	if len(msisdn)%2 != 0 {
		msisdn += "F"
	}
	result := make([]byte, len(msisdn)/2)
	for i := 0; i < len(msisdn); i += 2 {
		lo := digitToNibble(msisdn[i])
		hi := digitToNibble(msisdn[i+1])
		result[i/2] = (hi << 4) | lo
	}
	return result, nil
}

func digitToNibble(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c == 'F' || c == 'f':
		return 0xF
	default:
		return 0
	}
}

func decodeTBCDString(b []byte) string {
	if len(b) < 1 {
		return ""
	}
	digits := make([]byte, 0, len(b)*2)
	for _, octet := range b {
		lo := octet & 0x0F
		hi := (octet >> 4) & 0x0F
		digits = append(digits, nibbleToDigit(lo), nibbleToDigit(hi))
	}
	for len(digits) > 0 && digits[len(digits)-1] == 'F' {
		digits = digits[:len(digits)-1]
	}
	return string(digits)
}

func nibbleToDigit(n byte) byte {
	if n <= 9 {
		return '0' + n
	}
	return 'F'
}

// ardValue returns the Access-Restriction-Data bitmask to include in Subscription-Data.
// When unset (nil) we send 0 -- all RATs allowed (backward-compatible default).
func ardValue(v *uint32) uint32 {
	if v != nil {
		return *v
	}
	return 0
}

func boolToPreemption(b *bool, def bool) int32 {
	v := def
	if b != nil {
		v = *b
	}
	if v {
		return 0
	}
	return 1
}
