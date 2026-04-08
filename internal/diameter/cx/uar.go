package cx

import (
	"context"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/diameter/avputil"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

func (h *Handlers) UAR(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
	var uar UAR
	if err := msg.Unmarshal(&uar); err != nil {
		h.log.Error("cx: UAR unmarshal failed", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, "", h.originHost, h.originRealm, diam.UnableToComply), err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Normalize identities — strip SIP URI scheme and @domain before DB lookup.
	msisdn := normalizeMSISDN(string(uar.PublicIdentity))
	imsi := normalizeIMSI(string(uar.UserName))

	// Try lookup by MSISDN (from Public-Identity) first, then by IMSI (from User-Name).
	sub, err := h.store.GetIMSSubscriberByMSISDN(ctx, msisdn)
	if err == repository.ErrNotFound {
		sub, err = h.store.GetIMSSubscriberByIMSI(ctx, imsi)
	}
	identity := msisdn
	if identity == "" {
		identity = imsi
	}
	if err == repository.ErrNotFound {
		h.log.Warn("cx: UAR unknown identity", zap.String("identity", identity))
		return avputil.ConstructFailureAnswer(msg, uar.SessionID, h.originHost, h.originRealm, DiameterErrorUserUnknown), err
	}
	if err != nil {
		return avputil.ConstructFailureAnswer(msg, uar.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
	}

	// Treat empty string the same as nil — a leftover from a failed SAR.
	scscf := ""
	if sub.SCSCF != nil && *sub.SCSCF != "" {
		scscf = *sub.SCSCF
	}

	h.log.Debug("cx: UAR success", zap.String("identity", identity), zap.String("scscf", scscf))
	ans := buildCxAnswer(msg, uar.SessionID, h.originHost, h.originRealm)

	uatType := int(uar.UserAuthorizationType)

	// De-registration: return current S-CSCF (if any) with base Result-Code.
	if uatType == UATDeRegistration {
		if scscf != "" {
			ans.NewAVP(avpServerName, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.UTF8String(scscf))
		}
		ans.NewAVP(avp.ResultCode, avp.Mbit, 0, datatype.Unsigned32(diam.Success))
		return ans, nil
	}

	if scscf != "" {
		// Subscriber already has an S-CSCF — subsequent registration.
		ans.NewAVP(avpServerName, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.UTF8String(scscf))
		ans.NewAVP(avp.ExperimentalResult, avp.Mbit, 0, &diam.GroupedAVP{AVP: []*diam.AVP{
			diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(Vendor3GPP)),
			diam.NewAVP(avp.ExperimentalResultCode, avp.Mbit, 0, datatype.Unsigned32(DiameterSubsequentRegistration)),
		}})
		return ans, nil
	}

	// No S-CSCF assigned yet — first registration.
	// If the pool has an entry, recommend it so the I-CSCF can route directly
	// (mirrors PyHSS scscf_pool behaviour). Otherwise fall back to empty
	// Server-Capabilities and let the I-CSCF select on its own.
	if poolEntry := h.pickSCSCF(); poolEntry != "" {
		ans.NewAVP(avpServerName, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.UTF8String(poolEntry))
	} else {
		ans.NewAVP(avpServerCapabilities, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: []*diam.AVP{}})
	}
	ans.NewAVP(avp.ExperimentalResult, avp.Mbit, 0, &diam.GroupedAVP{AVP: []*diam.AVP{
		diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(Vendor3GPP)),
		diam.NewAVP(avp.ExperimentalResultCode, avp.Mbit, 0, datatype.Unsigned32(DiameterFirstRegistration)),
	}})
	return ans, nil
}
