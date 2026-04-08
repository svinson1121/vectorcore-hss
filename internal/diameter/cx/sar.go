package cx

import (
	"context"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/diameter/avputil"
	"github.com/svinson1121/vectorcore-hss/internal/geored"
	"github.com/svinson1121/vectorcore-hss/internal/ims"
	"github.com/svinson1121/vectorcore-hss/internal/models"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

func (h *Handlers) SAR(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
	var sar SAR
	if err := msg.Unmarshal(&sar); err != nil {
		h.log.Error("cx: SAR unmarshal failed", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, "", h.originHost, h.originRealm, diam.UnableToComply), err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Normalize and try lookup by UserName (IMSI) first, then PublicIdentity (MSISDN).
	imsi := normalizeIMSI(string(sar.UserName))
	msisdn := normalizeMSISDN(string(sar.PublicIdentity))
	sub, err := h.store.GetIMSSubscriberByIMSI(ctx, imsi)
	identity := imsi
	if err == repository.ErrNotFound {
		identity = msisdn
		sub, err = h.store.GetIMSSubscriberByMSISDN(ctx, msisdn)
	}
	if err == repository.ErrNotFound {
		h.log.Warn("cx: SAR unknown subscriber", zap.String("user", string(sar.UserName)))
		return avputil.ConstructFailureAnswer(msg, sar.SessionID, h.originHost, h.originRealm, DiameterErrorUserUnknown), err
	}
	if err != nil {
		return avputil.ConstructFailureAnswer(msg, sar.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
	}

	satType := int(sar.ServerAssignmentType)
	now := time.Now()

	switch satType {
	case SATRegistration, SATReRegistration:
		serverName := string(sar.ServerName)
		realm := ""
		if err := h.store.UpdateIMSSCSCF(ctx, sub.MSISDN, &repository.IMSSCSCFUpdate{
			SCSCF:     &serverName,
			Realm:     &realm,
			Timestamp: &now,
		}); err != nil {
			return avputil.ConstructFailureAnswer(msg, sar.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
		}
		h.pub.PublishIMSSCSCF(geored.PayloadIMSSCSCF{MSISDN: sub.MSISDN, SCSCF: &serverName, Realm: &realm, Timestamp: &now})
	case SATUserDeregistration, SATAdministrativeDeregistration, SATTimeoutDeregistration:
		if err := h.store.UpdateIMSSCSCF(ctx, sub.MSISDN, &repository.IMSSCSCFUpdate{
			SCSCF:     nil,
			Timestamp: &now,
		}); err != nil {
			return avputil.ConstructFailureAnswer(msg, sar.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
		}
		h.pub.PublishIMSSCSCF(geored.PayloadIMSSCSCF{MSISDN: sub.MSISDN, SCSCF: nil, Timestamp: &now})
	case SATUnregisteredUser:
		// No SCSCF update — just return profile.
	}

	// Build user profile data.
	var ifc *models.IFCProfile
	if sub.IFCProfileID != nil {
		ifc, _ = h.store.GetIFCProfileByID(ctx, *sub.IFCProfileID)
	}
	profile := ims.BuildCxUserData(sub, ifc, h.mcc, h.mnc)

	h.log.Debug("cx: SAR success", zap.String("user", identity), zap.Int("sat", satType))
	ans := buildCxAnswer(msg, sar.SessionID, h.originHost, h.originRealm)
	if string(sar.UserName) != "" {
		ans.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(sar.UserName))
	}
	ans.NewAVP(avpUserData, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.OctetString(profile))
	ans.NewAVP(avp.ResultCode, avp.Mbit, 0, datatype.Unsigned32(diam.Success))
	return ans, nil
}
