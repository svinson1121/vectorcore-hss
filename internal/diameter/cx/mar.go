package cx

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
		h.log.Error("cx: MAR unmarshal failed", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, "", h.originHost, h.originRealm, diam.UnableToComply), err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	imsi := normalizeIMSI(string(mar.UserName))

	// Look up IMS subscriber by IMSI.
	_, err := h.store.GetIMSSubscriberByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		h.log.Warn("cx: MAR unknown IMSI", zap.String("imsi", imsi))
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, DiameterErrorUserUnknown), err
	}
	if err != nil {
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
	}

	// Look up AUC record for auth material.
	auc, err := h.store.GetAUCByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		h.log.Warn("cx: MAR no AUC for IMSI", zap.String("imsi", imsi))
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUserUnknown), err
	}
	if err != nil {
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
	}

	// Determine auth scheme.
	// Per 3GPP TS 29.229 §6.3.4, the S-CSCF sends "unknown" on the first MAR
	// to let the HSS choose the scheme. Treat "unknown" (and empty) as the
	// default Digest-AKAv1-MD5.
	scheme := "Digest-AKAv1-MD5"
	if mar.SIPAuthDataItem != nil {
		s := string(mar.SIPAuthDataItem.SIPAuthenticationScheme)
		if s != "" && !strings.EqualFold(s, "unknown") {
			scheme = s
		}
	}

	isAKA := strings.Contains(scheme, "AKA") ||
		strings.EqualFold(scheme, "Digest-AKAv1-MD5") ||
		strings.EqualFold(scheme, "Digest-AKAv2-SHA-256")

	if !isAKA {
		h.log.Warn("cx: MAR unsupported auth scheme", zap.String("scheme", scheme))
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, DiameterErrorAuthSchemeNotSupported), nil
	}

	profile, err := crypto.LoadProfile(ctx, h.store, auc.AlgorithmProfileID)
	if err != nil {
		h.log.Error("cx: MAR profile load failed", zap.String("imsi", imsi), zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
	}

	v, err := crypto.GenerateEAPAKAVector(auc, profile, h.store, ctx)
	if err != nil {
		h.log.Error("cx: MAR vector generation failed", zap.String("imsi", imsi), zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
	}

	// SIP-Authenticate = RAND(16) || AUTN(16)
	sipAuth := append(v.RAND, v.AUTN...)

	ans := buildCxAnswer(msg, mar.SessionID, h.originHost, h.originRealm)
	// Echo back the private identity exactly as received (e.g. "IMSI@ims.domain"),
	// matching PyHSS behaviour — S-CSCF uses this to correlate the response.
	ans.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(mar.UserName))
	ans.NewAVP(avpPublicIdentity, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.UTF8String(mar.PublicIdentity))
	ans.NewAVP(avpSIPNumberAuthItems, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(1))
	ans.NewAVP(avpSIPAuthDataItem, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{
		AVP: []*diam.AVP{
			diam.NewAVP(avpSIPItemNumber, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(0)),
			diam.NewAVP(avpSIPAuthenticationScheme, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.UTF8String(scheme)),
			diam.NewAVP(avpSIPAuthenticate, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.OctetString(sipAuth)),
			diam.NewAVP(avpSIPAuthorization, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.OctetString(v.XRES)),
			diam.NewAVP(avpConfidentialityKey, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.OctetString(v.CK)),
			diam.NewAVP(avpIntegrityKey, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.OctetString(v.IK)),
		},
	})
	ans.NewAVP(avp.ResultCode, avp.Mbit, 0, datatype.Unsigned32(diam.Success))
	return ans, nil
}
