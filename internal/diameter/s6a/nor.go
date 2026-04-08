package s6a

import (
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
	SessionID   datatype.UTF8String       `avp:"Session-Id"`
	OriginHost  datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	OriginRealm datatype.DiameterIdentity `avp:"Origin-Realm,omitempty"`
	UserName    datatype.UTF8String       `avp:"User-Name,omitempty"`
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
	)

	return avputil.ConstructSuccessAnswer(msg, req.SessionID, h.originHost, h.originRealm, AppIDS6a), nil
}
