package s6c

import (
	"context"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/diameter/avputil"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/tbcd"
	"github.com/svinson1121/vectorcore-hss/internal/models"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

// SRISR handles a Send-Routing-Info-for-SM Request from an SMS-SC.
// It looks up the subscriber by MSISDN (or IMSI), returns the serving MME
// info and IMSI so the SMS-SC can deliver the MT SMS via SGd/T4.
func (h *Handlers) SRISR(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
	var req SRISR
	if err := msg.Unmarshal(&req); err != nil {
		h.log.Error("s6c: SRI-SM unmarshal failed", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, "", h.originHost, h.originRealm, diam.UnableToComply), err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var sub *models.Subscriber
	var err error
	byIMSI := string(req.UserName) != ""

	if byIMSI {
		sub, err = h.store.GetSubscriberByIMSI(ctx, string(req.UserName))
	} else {
		msisdn := decodeMSISDN(req.MSISDN)
		sub, err = h.store.GetSubscriberByMSISDN(ctx, msisdn)
	}

	if err == repository.ErrNotFound {
		h.log.Warn("s6c: SRI-SM unknown subscriber")
		return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUserUnknown), nil
	}
	if err != nil {
		h.log.Error("s6c: SRI-SM store error", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
	}

	ans := avputil.ConstructSuccessAnswer(msg, req.SessionID, h.originHost, h.originRealm, AppIDS6c)

	// Always return the IMSI as User-Name.
	ans.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(sub.IMSI))

	// Return MSISDN BCD-encoded if we have it.
	if sub.MSISDN != nil {
		ans.NewAVP(avpMSISDN, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.OctetString(encodeMSISDNBytes(*sub.MSISDN)))
	}

	smsRegistered := sub.MMERegisteredForSMS != nil && *sub.MMERegisteredForSMS
	if sub.ServingMME != nil && smsRegistered {
		// Subscriber is registered — return Serving-Node.
		var nodeAVPs []*diam.AVP
		nodeAVPs = append(nodeAVPs,
			diam.NewAVP(avpMMEName, avp.Mbit|avp.Vbit, Vendor3GPP,
				datatype.DiameterIdentity(*sub.ServingMME)))
		if sub.MMENumberForMTSMS != nil {
			nodeAVPs = append(nodeAVPs,
				diam.NewAVP(avpMMENumberForMTSMS, avp.Vbit, Vendor3GPP,
					datatype.OctetString(encodeMSISDNBytes(*sub.MMENumberForMTSMS))))
		}
		if sub.ServingMMERealm != nil {
			nodeAVPs = append(nodeAVPs,
				diam.NewAVP(avpMMERealm, avp.Vbit, Vendor3GPP,
					datatype.DiameterIdentity(*sub.ServingMMERealm)))
		}
		ans.NewAVP(avpServingNode, avp.Mbit|avp.Vbit, Vendor3GPP,
			&diam.GroupedAVP{AVP: nodeAVPs})
	} else {
		// Subscriber not attached — set MNRF (Mobile Not Reachable Flag).
		ans.NewAVP(avpMWDStatus, avp.Vbit, Vendor3GPP,
			datatype.Unsigned32(MWDStatusMNRF))
	}

	h.log.Info("s6c: SRI-SM success", zap.String("imsi", sub.IMSI),
		zap.Bool("attached", sub.ServingMME != nil),
		zap.Bool("sms_registered", smsRegistered))
	return ans, nil
}

// encodeMSISDNBytes encodes an MSISDN string as TBCD bytes.
// Digits are packed two per byte, nibbles swapped; odd length is padded with 0xF.
func encodeMSISDNBytes(msisdn string) []byte {
	b, err := tbcd.EncodeMSISDN(msisdn)
	if err != nil {
		return nil
	}
	return b
}

// decodeMSISDN decodes a TBCD-encoded MSISDN, stripping trailing 0xF filler nibbles.
func decodeMSISDN(b datatype.OctetString) string {
	number, err := tbcd.DecodeMSISDN([]byte(b))
	if err != nil {
		return ""
	}
	return number
}
