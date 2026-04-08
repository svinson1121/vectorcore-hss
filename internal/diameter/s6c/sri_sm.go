package s6c

import (
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
		return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUserUnknown), err
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
	if len(msisdn)%2 != 0 {
		msisdn += "F"
	}
	result := make([]byte, len(msisdn)/2)
	for i := 0; i < len(msisdn); i += 2 {
		lo := msisdnDigitToNibble(msisdn[i])
		hi := msisdnDigitToNibble(msisdn[i+1])
		result[i/2] = (hi << 4) | lo
	}
	return result
}

// decodeMSISDN decodes a TBCD-encoded MSISDN, stripping trailing 0xF filler nibbles.
func decodeMSISDN(b datatype.OctetString) string {
	if len(b) < 1 {
		return ""
	}
	bcd := []byte(b)
	digits := make([]byte, 0, len(bcd)*2)
	for _, octet := range bcd {
		lo := octet & 0x0F
		hi := (octet >> 4) & 0x0F
		digits = append(digits, msisdnNibbleToDigit(lo), msisdnNibbleToDigit(hi))
	}
	for len(digits) > 0 && digits[len(digits)-1] == 'F' {
		digits = digits[:len(digits)-1]
	}
	return string(digits)
}

func msisdnDigitToNibble(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c == 'F' || c == 'f':
		return 0xF
	default:
		return 0
	}
}

func msisdnNibbleToDigit(n byte) byte {
	if n <= 9 {
		return '0' + n
	}
	return 'F'
}
