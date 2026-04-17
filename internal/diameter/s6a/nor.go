package s6a

import (
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/diameter/avputil"
)

const (
	ratTypeUTRAN = datatype.Unsigned32(1000)
	ratTypeGERAN = datatype.Unsigned32(1001)
)

type NOR struct {
	SessionID                 datatype.UTF8String       `avp:"Session-Id"`
	OriginHost                datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	OriginRealm               datatype.DiameterIdentity `avp:"Origin-Realm,omitempty"`
	UserName                  datatype.UTF8String       `avp:"User-Name,omitempty"`
	AlertReason               datatype.Enumerated       `avp:"Alert-Reason,omitempty"`
	NORFlags                  datatype.Unsigned32       `avp:"NOR-Flags,omitempty"`
	MaximumUEAvailabilityTime datatype.Time             `avp:"Maximum-UE-Availability-Time,omitempty"`
}

func (h *Handlers) NOR(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
	var req NOR
	if err := msg.Unmarshal(&req); err != nil {
		h.log.Error("s6a: NOR unmarshal failed", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, "", h.originHost, h.originRealm, diam.UnableToComply), err
	}

	h.log.Info("s6a: NOR received",
		zap.String("imsi", string(req.UserName)),
		zap.String("peer", string(req.OriginHost)),
		zap.Int32("alert_reason", int32(req.AlertReason)),
		zap.Uint32("nor_flags", uint32(req.NORFlags)),
		zap.Time("maximum_ue_availability_time", time.Time(req.MaximumUEAvailabilityTime)),
	)

	if h.onSubscriberReady != nil && string(req.UserName) != "" {
		if trigger, ok := alertTriggerFromReason(int32(req.AlertReason)); ok {
			go h.onSubscriberReady(string(req.UserName), trigger, timePtr(req.MaximumUEAvailabilityTime))
		}
	}

	return avputil.ConstructSuccessAnswer(msg, req.SessionID, h.originHost, h.originRealm, AppIDS6a), nil
}

func alertTriggerFromReason(reason int32) (AlertTrigger, bool) {
	switch reason {
	case AlertReasonUEPresent:
		return AlertTriggerUserAvailable, true
	case AlertReasonUEMemoryAvailable:
		return AlertTriggerMemoryAvailable, true
	default:
		return "", false
	}
}

func timePtr(v datatype.Time) *time.Time {
	t := time.Time(v)
	if t.IsZero() {
		return nil
	}
	return &t
}
