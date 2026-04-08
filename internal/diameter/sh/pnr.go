package sh

import (
	"fmt"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"
)

const cmdPNR = uint32(309)

type pendingPNREntry struct {
	publicIdentity string
	destHost       string
	destRealm      string
}

// SendPNR sends a Push-Notification-Request to an Application Server,
// notifying it of a change to subscriber data it has subscribed to.
func (h *Handlers) SendPNR(publicIdentity, destHost, destRealm, userData string) {
	conn, ok := h.peers.GetConn(destHost)
	if !ok {
		h.log.Warn("sh: PNR skipped — AS not connected",
			zap.String("identity", publicIdentity), zap.String("as", destHost))
		return
	}

	sid := fmt.Sprintf("%s;%d;pnr", h.originHost, time.Now().UnixNano())
	msg := diam.NewRequest(cmdPNR, AppIDSh, nil)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(h.originHost))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(h.originRealm))
	msg.NewAVP(avp.DestinationHost, avp.Mbit, 0, datatype.DiameterIdentity(destHost))
	msg.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity(destRealm))

	identityAVPs := []*diam.AVP{
		diam.NewAVP(avpPublicIdentity, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.UTF8String(publicIdentity)),
	}
	msg.NewAVP(avpUserIdentity, avp.Mbit|avp.Vbit, Vendor3GPP,
		&diam.GroupedAVP{AVP: identityAVPs})
	msg.NewAVP(avpUserData, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.OctetString(userData))

	if _, err := msg.WriteTo(conn); err != nil {
		h.log.Error("sh: PNR send failed",
			zap.String("identity", publicIdentity), zap.String("dest_host", destHost), zap.Error(err))
		return
	}
	h.pendingPNR.Store(sid, pendingPNREntry{
		publicIdentity: publicIdentity,
		destHost:       destHost,
		destRealm:      destRealm,
	})
	h.log.Info("sh: PNR sent - awaiting PNA",
		zap.String("identity", publicIdentity),
		zap.String("dest_host", destHost),
		zap.String("session_id", sid))
}

// PNA handles a Push-Notification-Answer returned by the Application Server.
// The PNR is fire-and-forget on send, but answers are still tracked so the HSS
// can distinguish accepted notifications from rejects or routing failures.
func (h *Handlers) PNA(conn diam.Conn, msg *diam.Message) {
	sidAVP, err := msg.FindAVP(avp.SessionID, 0)
	if err != nil {
		h.log.Warn("sh: PNA missing Session-Id")
		return
	}
	sid, ok := sidAVP.Data.(datatype.UTF8String)
	if !ok {
		h.log.Warn("sh: PNA Session-Id has unexpected type")
		return
	}

	entry, found := h.pendingPNR.LoadAndDelete(string(sid))
	if !found {
		h.log.Warn("sh: PNA for unknown session", zap.String("session_id", string(sid)))
		return
	}
	pending := entry.(pendingPNREntry)

	resultCode, ok := shAnswerResultCode(msg)
	if !ok {
		h.log.Warn("sh: PNA missing Result-Code / Experimental-Result",
			zap.String("identity", pending.publicIdentity),
			zap.String("dest_host", pending.destHost),
			zap.String("session_id", string(sid)))
		return
	}

	fields := []zap.Field{
		zap.String("identity", pending.publicIdentity),
		zap.String("dest_host", pending.destHost),
		zap.String("dest_realm", pending.destRealm),
		zap.String("session_id", string(sid)),
		zap.Uint32("result_code", resultCode),
	}
	if conn != nil {
		fields = append(fields, zap.String("peer", conn.RemoteAddr().String()))
	}

	if resultCode != diam.Success {
		h.log.Warn("sh: PNA failure", fields...)
		return
	}

	h.log.Info("sh: PNA success", fields...)
}

func shAnswerResultCode(msg *diam.Message) (uint32, bool) {
	if a, err := msg.FindAVP(avp.ResultCode, 0); err == nil {
		if rc, ok := a.Data.(datatype.Unsigned32); ok {
			return uint32(rc), true
		}
	}

	if a, err := msg.FindAVP(avp.ExperimentalResult, 0); err == nil {
		if grp, ok := a.Data.(*diam.GroupedAVP); ok {
			for _, child := range grp.AVP {
				if child.Code != avp.ExperimentalResultCode {
					continue
				}
				if rc, ok := child.Data.(datatype.Unsigned32); ok {
					return uint32(rc), true
				}
			}
		}
	}

	return 0, false
}
