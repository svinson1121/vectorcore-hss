package slh

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

func (h *Handlers) LRR(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
	var req LRR
	if err := msg.Unmarshal(&req); err != nil {
		h.log.Error("slh: LRR unmarshal failed", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, "", h.originHost, h.originRealm, diam.UnableToComply), err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var sub *models.Subscriber
	var err error
	byIMSI := string(req.UserName) != ""

	if byIMSI {
		imsi := string(req.UserName)
		sub, err = h.store.GetSubscriberByIMSI(ctx, imsi)
	} else {
		msisdn := decodeMSISDN(req.MSISDN)
		sub, err = h.store.GetSubscriberByMSISDN(ctx, msisdn)
	}

	if err == repository.ErrNotFound {
		h.log.Warn("slh: LRR unknown subscriber")
		return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUserUnknown), err
	}
	if err != nil {
		h.log.Error("slh: LRR store error", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, req.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUserUnknown), err
	}

	ans := avputil.ConstructSuccessAnswer(msg, req.SessionID, h.originHost, h.originRealm, AppIDSLh)

	// If queried by IMSI and subscriber has an MSISDN, return it encoded as BCD.
	if byIMSI && sub.MSISDN != nil {
		ans.NewAVP(avpMSISDN, avp.Vbit, Vendor3GPP, datatype.OctetString(encodeMSISDNBytes(*sub.MSISDN)))
	}

	// If queried by MSISDN, return the IMSI as User-Name.
	if !byIMSI {
		ans.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(sub.IMSI))
	}

	// Build Serving-Node grouped AVP.
	var servingAVPs []*diam.AVP
	if sub.ServingMME != nil {
		servingAVPs = append(servingAVPs,
			diam.NewAVP(avpMMEName, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.DiameterIdentity(*sub.ServingMME)))
	}
	if sub.ServingMMERealm != nil {
		servingAVPs = append(servingAVPs,
			diam.NewAVP(avpMMERealm, avp.Vbit, Vendor3GPP, datatype.DiameterIdentity(*sub.ServingMMERealm)))
	}
	if sub.ServingSGSN != nil {
		servingAVPs = append(servingAVPs,
			diam.NewAVP(avpSGSNName, avp.Vbit, Vendor3GPP, datatype.DiameterIdentity(*sub.ServingSGSN)))
	}

	if len(servingAVPs) > 0 {
		ans.NewAVP(avpServingNode, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: servingAVPs})
	}

	h.log.Debug("slh: LRR success", zap.String("imsi", sub.IMSI))
	return ans, nil
}

// encodeMSISDNBytes encodes an MSISDN string as TBCD bytes (pure digits, no TON/NPI prefix).
// Digits are packed two per byte with nibbles swapped; an odd-length number is padded with 0xF.
func encodeMSISDNBytes(msisdn string) []byte {
	if len(msisdn)%2 != 0 {
		msisdn += "F"
	}
	result := make([]byte, len(msisdn)/2)
	for i := 0; i < len(msisdn); i += 2 {
		lo := digitToNibble(msisdn[i])
		hi := digitToNibble(msisdn[i+1])
		result[i/2] = (hi << 4) | lo
	}
	return result
}

// decodeMSISDN decodes a TBCD-encoded MSISDN byte slice, stripping trailing 0xF filler nibbles.
func decodeMSISDN(b datatype.OctetString) string {
	if len(b) < 1 {
		return ""
	}
	bcd := []byte(b)
	digits := make([]byte, 0, len(bcd)*2)
	for _, octet := range bcd {
		lo := octet & 0x0F
		hi := (octet >> 4) & 0x0F
		digits = append(digits, nibbleToDigit(lo), nibbleToDigit(hi))
	}
	// Strip trailing 'F' padding.
	for len(digits) > 0 && digits[len(digits)-1] == 'F' {
		digits = digits[:len(digits)-1]
	}
	return string(digits)
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

func nibbleToDigit(n byte) byte {
	if n <= 9 {
		return '0' + n
	}
	return 'F'
}
