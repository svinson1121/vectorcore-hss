package s6a

import (
	"context"
	"fmt"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/crypto"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/avputil"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

func (h *Handlers) AIR(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
	var air AIR
	if err := msg.Unmarshal(&air); err != nil {
		h.log.Error("s6a: AIR unmarshal failed", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, "", h.originHost, h.originRealm, diam.UnableToComply), err
	}

	imsi := air.UserName
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	peer := conn.RemoteAddr().String()
	visitedPLMN := []byte(air.VisitedPLMNID)

	sub, err := h.store.GetSubscriberByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		h.log.Warn("s6a: AIR unknown IMSI", zap.String("imsi", imsi))
		h.RecordAuthFailure(imsi, peer, "Unknown IMSI", visitedPLMN)
		return avputil.ConstructFailureAnswer(msg, air.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUserUnknown), err
	}
	if err != nil {
		h.RecordAuthFailure(imsi, peer, "Database error (subscriber lookup)", visitedPLMN)
		return avputil.ConstructFailureAnswer(msg, air.SessionID, h.originHost, h.originRealm, avputil.DiameterAuthenticationDataUnavailable), err
	}
	if sub.Enabled != nil && !*sub.Enabled {
		h.log.Warn("s6a: AIR subscriber disabled", zap.String("imsi", imsi))
		h.RecordAuthFailure(imsi, peer, "Subscriber disabled", visitedPLMN)
		return avputil.ConstructFailureAnswer(msg, air.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUnknownEPSSubscription), nil
	}

	if err := h.checkRoaming(ctx, sub, visitedPLMN); err != nil {
		h.log.Warn("s6a: AIR roaming denied", zap.String("imsi", imsi), zap.Error(err))
		h.RecordAuthFailure(imsi, peer, "Roaming not allowed", visitedPLMN)
		return avputil.ConstructFailureAnswer(msg, air.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorRoamingNotAllowed), nil
	}

	auc, err := h.store.GetAUCByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		h.log.Warn("s6a: AIR unknown IMSI (no AUC)", zap.String("imsi", imsi))
		h.RecordAuthFailure(imsi, peer, "Unknown IMSI (no AUC record)", visitedPLMN)
		return avputil.ConstructFailureAnswer(msg, air.SessionID, h.originHost, h.originRealm, avputil.DiameterErrorUserUnknown), err
	}
	if err != nil {
		h.RecordAuthFailure(imsi, peer, "Database error (AUC lookup)", visitedPLMN)
		return avputil.ConstructFailureAnswer(msg, air.SessionID, h.originHost, h.originRealm, avputil.DiameterAuthenticationDataUnavailable), err
	}

	profile, err := crypto.LoadProfile(ctx, h.store, auc.AlgorithmProfileID)
	if err != nil {
		h.log.Error("s6a: AIR profile load failed", zap.String("imsi", imsi), zap.Error(err))
		h.RecordAuthFailure(imsi, peer, "Algorithm profile load failed", visitedPLMN)
		return avputil.ConstructFailureAnswer(msg, air.SessionID, h.originHost, h.originRealm, avputil.DiameterAuthenticationDataUnavailable), err
	}

	// SQN resync — Re-synchronization-Info is 30 bytes: RAND(16) || AUTS(14)
	if air.RequestedEUTRANAuthInfo != nil {
		resync := []byte(air.RequestedEUTRANAuthInfo.ResyncInfo)
		if len(resync) == 30 {
			newSQN, err := crypto.ResyncSQNFull(auc, profile, resync)
			if err != nil {
				h.log.Error("s6a: AIR resync failed", zap.Error(err))
				h.RecordAuthFailure(imsi, peer, "SQN resync failed", visitedPLMN)
				return avputil.ConstructFailureAnswer(msg, air.SessionID, h.originHost, h.originRealm, avputil.DiameterAuthenticationDataUnavailable), err
			}
			if err := h.store.ResyncSQN(ctx, auc.AUCID, newSQN+100); err != nil {
				h.RecordAuthFailure(imsi, peer, "SQN resync update failed", visitedPLMN)
				return avputil.ConstructFailureAnswer(msg, air.SessionID, h.originHost, h.originRealm, avputil.DiameterAuthenticationDataUnavailable), err
			}
			auc, err = h.store.GetAUCByIMSI(ctx, imsi)
			if err != nil {
				h.RecordAuthFailure(imsi, peer, "SQN reload failed", visitedPLMN)
				return avputil.ConstructFailureAnswer(msg, air.SessionID, h.originHost, h.originRealm, avputil.DiameterAuthenticationDataUnavailable), err
			}
		}
	}

	numVectors := uint32(1)
	if air.RequestedEUTRANAuthInfo != nil && uint32(air.RequestedEUTRANAuthInfo.NumVectors) > 0 {
		numVectors = uint32(air.RequestedEUTRANAuthInfo.NumVectors)
		if numVectors > 5 {
			numVectors = 5
		}
	}

	plmn := visitedPLMN
	if len(plmn) != 3 {
		err := fmt.Errorf("bad PLMN length %d", len(plmn))
		h.RecordAuthFailure(imsi, peer, "Invalid PLMN length", visitedPLMN)
		return avputil.ConstructFailureAnswer(msg, air.SessionID, h.originHost, h.originRealm, avputil.DiameterAuthenticationDataUnavailable), err
	}

	vectors, err := crypto.GenerateEUTRANVectors(auc, profile, plmn, numVectors, h.store, ctx)
	if err != nil {
		h.log.Error("s6a: AIR vector generation failed", zap.String("imsi", imsi), zap.Error(err))
		h.RecordAuthFailure(imsi, peer, "Vector generation failed", visitedPLMN)
		return avputil.ConstructFailureAnswer(msg, air.SessionID, h.originHost, h.originRealm, avputil.DiameterAuthenticationDataUnavailable), err
	}

	h.pub.PublishSQNUpdate(auc.AUCID, auc.SQN+int64(numVectors)*32)
	h.log.Info("s6a: AIR success", zap.String("imsi", imsi), zap.Uint32("vectors", numVectors))
	return buildAIA(msg, air.SessionID, h.originHost, h.originRealm, vectors), nil
}

func buildAIA(req *diam.Message, sessionID datatype.UTF8String, originHost, originRealm string, vectors []crypto.EUTRANVector) *diam.Message {
	ans := avputil.ConstructSuccessAnswer(req, sessionID, originHost, originRealm, AppIDS6a)
	evs := make([]*diam.AVP, 0, len(vectors))
	for i, v := range vectors {
		evs = append(evs, diam.NewAVP(avp.EUTRANVector, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{
			AVP: []*diam.AVP{
				diam.NewAVP(avp.ItemNumber, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(i+1)),
				diam.NewAVP(avp.RAND, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.OctetString(v.RAND)),
				diam.NewAVP(avp.XRES, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.OctetString(v.XRES)),
				diam.NewAVP(avp.AUTN, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.OctetString(v.AUTN)),
				diam.NewAVP(avp.KASME, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.OctetString(v.KASME)),
			},
		}))
	}
	ans.NewAVP(avp.AuthenticationInfo, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: evs})
	return ans
}
