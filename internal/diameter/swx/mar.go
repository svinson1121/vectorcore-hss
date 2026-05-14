package swx

import (
	"context"
	"strings"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/crypto"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/avputil"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

func (h *Handlers) MAR(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
	var mar MAR
	if err := msg.Unmarshal(&mar); err != nil {
		h.log.Error("swx: MAR unmarshal failed", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, "", h.originHost, h.originRealm, diam.UnableToComply), err
	}

	imsi := string(mar.UserName)
	scheme := requestedSIPAuthScheme(&mar)
	anid := strings.TrimSpace(string(mar.ANID))
	numItems := uint32(mar.SIPNumberAuthItems)
	if numItems == 0 {
		numItems = 1
	}

	logFields := []zap.Field{
		zap.String("imsi", imsi),
		zap.String("scheme", scheme),
		zap.Uint32("rat_type", uint32(mar.RATType)),
		zap.Int32("an_trusted", int32(mar.ANTrusted)),
		zap.String("anid", anid),
		zap.Uint32("auth_items", numItems),
	}
	h.log.Debug("swx: MAR received", logFields...)

	switch scheme {
	case schemeEAPAKA:
		h.log.Debug("swx: MAR selected EAP-AKA vector generation", logFields...)
	case schemeEAPAKAPrime:
		if anid == "" {
			h.log.Warn("swx: MAR rejected: ANID is required for EAP-AKA'", logFields...)
			return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, diam.UnableToComply), nil
		}
		h.log.Debug("swx: MAR selected EAP-AKA' vector generation", logFields...)
	case schemeEAPSIM:
		h.log.Warn("swx: MAR rejected: EAP-SIM is not allowed for EPS non-3GPP access", logFields...)
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, DiameterErrorAuthSchemeNotSupported), nil
	default:
		h.log.Warn("swx: MAR rejected: unsupported SIP-Authentication-Scheme", logFields...)
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, DiameterErrorAuthSchemeNotSupported), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	auc, err := h.store.GetAUCByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		h.log.Warn("swx: MAR unknown IMSI", zap.String("imsi", imsi))
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, DiameterErrorUserUnknown), err
	}
	if err != nil {
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, avputil.DiameterAuthenticationDataUnavailable), err
	}

	profile, err := crypto.LoadProfile(ctx, h.store, auc.AlgorithmProfileID)
	if err != nil {
		h.log.Error("swx: MAR profile load failed", zap.String("imsi", imsi), zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, avputil.DiameterAuthenticationDataUnavailable), err
	}

	vec, err := crypto.GenerateEAPAKAVector(auc, profile, h.store, ctx)
	if err != nil {
		h.log.Error("swx: MAR vector generation failed", zap.String("imsi", imsi), zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, avputil.DiameterAuthenticationDataUnavailable), err
	}

	ck := vec.CK
	ik := vec.IK
	if scheme == schemeEAPAKAPrime {
		var err error
		ck, ik, err = crypto.DeriveEAPAKAPrimeKeys(vec.CK, vec.IK, anid, vec.AUTN[:6])
		if err != nil {
			h.log.Warn("swx: MAR rejected: EAP-AKA' key derivation failed", append(logFields, zap.Error(err))...)
			return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, DiameterAuthenticationDataUnavailable), err
		}
	}

	h.log.Debug("swx: MAR success", logFields...)

	// SIP-Authenticate carries RAND || AUTN (32 bytes)
	sipAuthenticate := append(vec.RAND, vec.AUTN...)

	authDataAVPs := []*diam.AVP{
		diam.NewAVP(avpSIPItemNumber, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(1)),
		diam.NewAVP(avpSIPAuthenticationScheme, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.UTF8String(scheme)),
		diam.NewAVP(avpSIPAuthenticate, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.OctetString(sipAuthenticate)),
		diam.NewAVP(avpSIPAuthorization, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.OctetString(vec.XRES)),
		diam.NewAVP(avpConfidentialityKey, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.OctetString(ck)),
		diam.NewAVP(avpIntegrityKey, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.OctetString(ik)),
	}

	ans := avputil.ConstructSuccessAnswer(msg, mar.SessionID, h.originHost, h.originRealm, AppIDSWx)
	ans.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))
	ans.NewAVP(avpSIPNumberAuthItems, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(1))
	ans.NewAVP(avpSIPAuthDataItem, avp.Mbit|avp.Vbit, Vendor3GPP,
		&diam.GroupedAVP{AVP: authDataAVPs})
	return ans, nil
}

func requestedSIPAuthScheme(mar *MAR) string {
	if mar.SIPAuthDataItem == nil {
		return ""
	}
	scheme := strings.TrimSpace(string(mar.SIPAuthDataItem.SIPAuthenticationScheme))
	return strings.ReplaceAll(scheme, "′", "'")
}
