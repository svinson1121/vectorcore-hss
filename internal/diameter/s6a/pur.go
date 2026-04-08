package s6a

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

func (h *Handlers) PUR(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
	var pur PUR
	if err := msg.Unmarshal(&pur); err != nil {
		h.log.Error("s6a: PUR unmarshal failed", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, "", h.originHost, h.originRealm, diam.UnableToComply), err
	}

	imsi := string(pur.UserName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.store.GetSubscriberByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		return avputil.ConstructFailureAnswer(msg, pur.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUserUnknown), err
	}
	if err != nil {
		return avputil.ConstructFailureAnswer(msg, pur.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
	}

	smsRegistered := false
	_ = h.store.UpdateServingMME(ctx, imsi, &repository.ServingMMEUpdate{
		ServingMME:               nil,
		MMENumberForMTSMS:        nil,
		MMERegisteredForSMS:      &smsRegistered,
		SMSRegisterRequest:       nil,
		SMSRegistrationTimestamp: nil,
	})

	h.log.Debug("s6a: PUR success", zap.String("imsi", imsi))
	ans := avputil.ConstructSuccessAnswer(msg, pur.SessionID, h.originHost, h.originRealm, AppIDS6a)
	ans.NewAVP(avp.PUAFlags, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(0))
	return ans, nil
}
