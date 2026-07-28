package slh

import (
	"context"
	"fmt"
	"strings"
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

// LRR is the go-diameter dispatch alias for the SLh RIR command.
func (h *Handlers) LRR(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
	var req RIR
	if err := msg.Unmarshal(&req); err != nil {
		h.log.Error("slh: RIR unmarshal failed", zap.Error(err))
		return h.baseFailure(msg, "", diam.UnableToComply), err
	}
	if !h.authorized(string(req.OriginRealm)) {
		h.log.Warn("slh: unauthorized GMLC realm", zap.String("origin_realm", string(req.OriginRealm)))
		return h.experimentalFailure(msg, req.SessionID, DiameterErrorUnauthorizedRequestingNetwork), nil
	}

	imsi := strings.TrimSpace(string(req.UserName))
	var msisdn string
	var err error
	if len(req.MSISDN) != 0 {
		msisdn, err = tbcd.DecodeMSISDN([]byte(req.MSISDN))
		if err != nil {
			return h.baseFailure(msg, req.SessionID, diam.UnableToComply), nil
		}
	}
	if imsi == "" && msisdn == "" {
		return h.baseFailure(msg, req.SessionID, diam.UnableToComply), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sub, lookupErr := h.lookupSubscriber(ctx, imsi, msisdn)
	if lookupErr != nil {
		switch lookupErr {
		case repository.ErrNotFound:
			return h.experimentalFailure(msg, req.SessionID, avputil.DiameterErrorUserUnknown), nil
		case errContradictingIdentities:
			return h.baseFailure(msg, req.SessionID, diam.ContradictingAVPs), nil
		default:
			h.log.Error("slh: RIR repository error", zap.Error(lookupErr))
			return h.baseFailure(msg, req.SessionID, diam.UnableToComply), lookupErr
		}
	}

	// The currently persisted SLh-capable registration state is LTE/S6a MME
	// state. A realm without a non-empty MME name is not a usable serving node.
	if sub.ServingMME == nil || strings.TrimSpace(*sub.ServingMME) == "" || sub.ServingMMERealm == nil || strings.TrimSpace(*sub.ServingMMERealm) == "" {
		return h.experimentalFailure(msg, req.SessionID, DiameterErrorAbsentUser), nil
	}

	ans := avputil.ConstructSuccessAnswer(msg, req.SessionID, h.originHost, h.originRealm, AppIDSLh)
	if imsi != "" {
		// TS 29.173 requires the corresponding MSISDN in an IMSI RIR.  The
		// TS 23.003 dummy MSISDN is used for subscriptions without one.
		number := dummyMSISDN
		if sub.MSISDN != nil && strings.TrimSpace(*sub.MSISDN) != "" {
			number = *sub.MSISDN
		}
		b, encErr := tbcd.EncodeMSISDN(number)
		if encErr != nil {
			return h.baseFailure(msg, req.SessionID, diam.UnableToComply), encErr
		}
		ans.NewAVP(avpMSISDN, avp.Vbit, Vendor3GPP, datatype.OctetString(b))
	}
	if msisdn != "" {
		ans.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(sub.IMSI))
	}
	ans.NewAVP(avpServingNode, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: []*diam.AVP{
		diam.NewAVP(avpMMEName, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.DiameterIdentity(*sub.ServingMME)),
		diam.NewAVP(avpMMERealm, avp.Vbit, Vendor3GPP, datatype.DiameterIdentity(*sub.ServingMMERealm)),
	}})
	h.log.Debug("slh: RIR success", zap.String("imsi", sub.IMSI))
	return ans, nil
}

const dummyMSISDN = "000000000000000"

var errContradictingIdentities = fmt.Errorf("RIR identities identify different subscribers")

func (h *Handlers) lookupSubscriber(ctx context.Context, imsi, msisdn string) (*models.Subscriber, error) {
	if imsi == "" {
		return h.store.GetSubscriberByMSISDN(ctx, msisdn)
	}
	byIMSI, err := h.store.GetSubscriberByIMSI(ctx, imsi)
	if err != nil || msisdn == "" {
		return byIMSI, err
	}
	byMSISDN, err := h.store.GetSubscriberByMSISDN(ctx, msisdn)
	if err != nil {
		return nil, err
	}
	if byIMSI.IMSI != byMSISDN.IMSI {
		return nil, errContradictingIdentities
	}
	return byIMSI, nil
}

func (h *Handlers) authorized(realm string) bool {
	realm = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(realm)), ".")
	if realm == strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h.originRealm)), ".") {
		return true
	}
	for _, allowed := range h.authorizedRealms {
		if realm == strings.TrimSuffix(strings.ToLower(strings.TrimSpace(allowed)), ".") {
			return true
		}
	}
	return false
}

func (h *Handlers) baseFailure(msg *diam.Message, sessionID datatype.UTF8String, code uint32) *diam.Message {
	return avputil.ConstructBaseFailureAnswer(msg, sessionID, h.originHost, h.originRealm, code)
}
func (h *Handlers) experimentalFailure(msg *diam.Message, sessionID datatype.UTF8String, code uint32) *diam.Message {
	return avputil.ConstructFailureAnswer(msg, sessionID, h.originHost, h.originRealm, code)
}
