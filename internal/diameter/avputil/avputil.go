package avputil

import (
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
)

const Vendor3GPP = uint32(10415)

const (
	DiameterErrorUserUnknown              = uint32(5001)
	DiameterErrorUnknownEPSSubscription   = uint32(5004)
	DiameterErrorRoamingNotAllowed        = uint32(5006) // 3GPP TS 29.272 — PLMN not allowed
	DiameterAuthenticationDataUnavailable = uint32(4181)
)

func ConstructSuccessAnswer(req *diam.Message, sessionID datatype.UTF8String, originHost, originRealm string, appID uint32) *diam.Message {
	ans := diam.NewMessage(req.Header.CommandCode, req.Header.CommandFlags&^diam.RequestFlag, appID, req.Header.HopByHopID, req.Header.EndToEndID, req.Dictionary())
	ans.InsertAVP(diam.NewAVP(avp.SessionID, avp.Mbit, 0, sessionID))
	ans.NewAVP(avp.ResultCode, avp.Mbit, 0, datatype.Unsigned32(diam.Success))
	ans.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	ans.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(originHost))
	ans.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(originRealm))
	ans.NewAVP(avp.OriginStateID, avp.Mbit, 0, datatype.Unsigned32(uint32(time.Now().Unix())))
	return ans
}

func ConstructFailureAnswer(req *diam.Message, sessionID datatype.UTF8String, originHost, originRealm string, resultCode uint32) *diam.Message {
	ans := diam.NewMessage(req.Header.CommandCode, req.Header.CommandFlags&^diam.RequestFlag, req.Header.ApplicationID, req.Header.HopByHopID, req.Header.EndToEndID, req.Dictionary())
	ans.InsertAVP(diam.NewAVP(avp.SessionID, avp.Mbit, 0, sessionID))
	ans.NewAVP(avp.ExperimentalResult, avp.Mbit, 0, &diam.GroupedAVP{AVP: []*diam.AVP{
		diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(Vendor3GPP)),
		diam.NewAVP(avp.ExperimentalResultCode, avp.Mbit, 0, datatype.Unsigned32(resultCode)),
	}})
	ans.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	ans.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(originHost))
	ans.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(originRealm))
	ans.NewAVP(avp.OriginStateID, avp.Mbit, 0, datatype.Unsigned32(uint32(time.Now().Unix())))
	return ans
}
