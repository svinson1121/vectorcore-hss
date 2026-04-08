package zh

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

const DiameterErrorUserUnknown = uint32(5001)

func (h *Handlers) MAR(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
	var mar MAR
	if err := msg.Unmarshal(&mar); err != nil {
		h.log.Error("zh: MAR unmarshal failed", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, "", h.originHost, h.originRealm, diam.UnableToComply), err
	}

	// User-Name carries the IMPI, typically IMSI@realm — strip the realm.
	impi := string(mar.UserName)
	imsi := strings.SplitN(impi, "@", 2)[0]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	auc, err := h.store.GetAUCByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		h.log.Warn("zh: MAR unknown IMSI", zap.String("imsi", imsi), zap.String("impi", impi))
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, DiameterErrorUserUnknown), err
	}
	if err != nil {
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, avputil.DiameterAuthenticationDataUnavailable), err
	}

	profile, err := crypto.LoadProfile(ctx, h.store, auc.AlgorithmProfileID)
	if err != nil {
		h.log.Error("zh: MAR profile load failed", zap.String("imsi", imsi), zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, avputil.DiameterAuthenticationDataUnavailable), err
	}

	vec, err := crypto.GenerateEAPAKAVector(auc, profile, h.store, ctx)
	if err != nil {
		h.log.Error("zh: MAR vector generation failed", zap.String("imsi", imsi), zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, mar.SessionID, h.originHost, h.originRealm, avputil.DiameterAuthenticationDataUnavailable), err
	}

	h.log.Debug("zh: MAR success", zap.String("imsi", imsi))

	// SIP-Authenticate carries RAND || AUTN (32 bytes), same as SWx.
	sipAuthenticate := append(vec.RAND, vec.AUTN...)

	authDataAVPs := []*diam.AVP{
		diam.NewAVP(avpSIPItemNumber, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(1)),
		diam.NewAVP(avpSIPAuthenticationScheme, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.UTF8String(schemeDigestAKAv1)),
		diam.NewAVP(avpSIPAuthenticate, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.OctetString(sipAuthenticate)),
		diam.NewAVP(avpSIPAuthorization, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.OctetString(vec.XRES)),
		diam.NewAVP(avpConfidentialityKey, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.OctetString(vec.CK)),
		diam.NewAVP(avpIntegrityKey, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.OctetString(vec.IK)),
	}

	ans := avputil.ConstructSuccessAnswer(msg, mar.SessionID, h.originHost, h.originRealm, AppIDZh)
	ans.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))
	ans.NewAVP(avpSIPNumberAuthItems, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(1))
	ans.NewAVP(avpSIPAuthDataItem, avp.Mbit|avp.Vbit, Vendor3GPP,
		&diam.GroupedAVP{AVP: authDataAVPs})
	return ans, nil
}
