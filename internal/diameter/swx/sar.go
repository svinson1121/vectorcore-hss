package swx

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

const (
	sarRegistration                 = 1
	sarReRegistration               = 2
	sarUnregisteredUser             = 3
	sarUserDeregistration           = 5
	sarAdministrativeDeregistration = 8
)

func (h *Handlers) SAR(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
	var sar SAR
	if err := msg.Unmarshal(&sar); err != nil {
		h.log.Error("swx: SAR unmarshal failed", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, "", h.originHost, h.originRealm, diam.UnableToComply), err
	}

	imsi := string(sar.UserName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := h.store.GetSubscriberByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		h.log.Warn("swx: SAR unknown IMSI", zap.String("imsi", imsi))
		return avputil.ConstructFailureAnswer(msg, sar.SessionID, h.originHost, h.originRealm, DiameterErrorUserUnknown), err
	}
	if err != nil {
		return avputil.ConstructFailureAnswer(msg, sar.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
	}

	satType := int(sar.ServerAssignmentType)
	h.log.Debug("swx: SAR", zap.String("imsi", imsi), zap.Int("sat", satType))

	ans := avputil.ConstructSuccessAnswer(msg, sar.SessionID, h.originHost, h.originRealm, AppIDSWx)
	ans.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))

	switch satType {
	case sarRegistration, sarReRegistration, sarUnregisteredUser:
		// Return Non-3GPP-User-Data
		accessStatus := Non3GPPAccessAllowed
		if sub.Enabled != nil && !*sub.Enabled {
			accessStatus = Non3GPPAccessBarred
		}
		ans.NewAVP(avpNon3GPPUserData, avp.Mbit|avp.Vbit, Vendor3GPP,
			&diam.GroupedAVP{AVP: []*diam.AVP{
				diam.NewAVP(avpNon3GPPIPAccess, avp.Mbit|avp.Vbit, Vendor3GPP,
					datatype.Enumerated(accessStatus)),
			}})
	case sarUserDeregistration, sarAdministrativeDeregistration:
		// Deregistration — success only, no profile data
	}

	return ans, nil
}
