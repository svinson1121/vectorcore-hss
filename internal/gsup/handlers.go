package gsup

// handlers.go -- GSUP message dispatch: SendAuthInfo, UpdateLocation, PurgeMS.

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/crypto"
	"github.com/svinson1121/vectorcore-hss/internal/geored"
	"github.com/svinson1121/vectorcore-hss/internal/models"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

// handleMessage dispatches an inbound GSUP message.
// proto is the IPA proto byte the peer used and must be echoed in responses.
func (s *Server) handleMessage(conn net.Conn, peerName string, proto byte, msg *Msg) {
	switch msg.Type {
	case MsgSendAuthInfoReq:
		s.handleAIR(conn, peerName, proto, msg)
	case MsgUpdateLocReq:
		s.handleULR(conn, peerName, proto, msg)
	case MsgPurgeMSReq:
		s.handlePUR(conn, peerName, proto, msg)
	case MsgAuthFailureReport, MsgDeleteDataRes, MsgInsertDataRes:
		// No response needed.
	default:
		s.log.Warn("gsup: unhandled message type",
			zap.String("peer", peerName),
			zap.String("type", fmt.Sprintf("0x%02X", msg.Type)),
		)
	}
}

// ── Send Auth Info (AIR) ──────────────────────────────────────────────────────

func (s *Server) handleAIR(conn net.Conn, peerName string, proto byte, msg *Msg) {
	imsi, ok := extractIMSI(msg)
	if !ok {
		s.log.Warn("gsup: AIR missing IMSI", zap.String("peer", peerName))
		sendError(conn, proto, MsgSendAuthInfoErr, "", CauseNetworkFailure)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	auc, err := s.store.GetAUCByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		s.log.Warn("gsup: AIR unknown IMSI", zap.String("imsi", imsi), zap.String("peer", peerName))
		sendError(conn, proto, MsgSendAuthInfoErr, imsi, CauseIMSIUnknown)
		return
	}
	if err != nil {
		s.log.Error("gsup: AIR db error", zap.String("imsi", imsi), zap.Error(err))
		sendError(conn, proto, MsgSendAuthInfoErr, imsi, CauseNetworkFailure)
		return
	}

	profile, err := crypto.LoadProfile(ctx, s.store, auc.AlgorithmProfileID)
	if err != nil {
		s.log.Error("gsup: AIR profile load error", zap.String("imsi", imsi), zap.Error(err))
		sendError(conn, proto, MsgSendAuthInfoErr, imsi, CauseNetworkFailure)
		return
	}

	// Number of requested vectors (IE 0x29); default to 1.
	numVec := 1
	if ie := msg.Get(IENumberOfRequestedVec); ie != nil && len(ie.Data) > 0 {
		numVec = int(ie.Data[0])
		if numVec < 1 {
			numVec = 1
		}
		if numVec > 5 {
			numVec = 5
		}
	}

	resp := NewMsg(MsgSendAuthInfoRes).Add(IEIMSITag, encodeIMSI(imsi))

	// Generate numVec auth tuples. Each EAP-AKA vector carries RAND, XRES,
	// AUTN, CK, IK -- exactly what the GSUP 3G quintuplet needs. The 2G
	// triplet (SRES, KC) is derived from CK/IK per 3GPP TS 55.205.
	for i := 0; i < numVec; i++ {
		v, err := crypto.GenerateEAPAKAVector(auc, profile, s.store, ctx)
		if err != nil {
			s.log.Error("gsup: AIR vector generation failed", zap.String("imsi", imsi), zap.Error(err))
			sendError(conn, proto, MsgSendAuthInfoErr, imsi, CauseNetworkFailure)
			return
		}

		// 2G material for CSFB/GSM auth:
		//   SRES = XRES[0:4]
		//   KC   = CK[0:8] XOR CK[8:16] XOR IK[0:8] XOR IK[8:16]  (3GPP TS 55.205)
		sres := v.XRES[0:4]
		kc := derive2GKc(v.CK, v.IK)

		tuple := NewMsg(0x00). // Builder reused for nested TLV encoding
					Add(IERANDTag, v.RAND).
					Add(IESRESTag, sres).
					Add(IEKcTag, kc).
					Add(IEIKTag, v.IK).
					Add(IECKTag, v.CK).
					Add(IEAUTNTag, v.AUTN).
					Add(IEXRESTag, v.XRES)

		// Strip the leading type byte -- we only want the IEs inside the tuple.
		resp.Add(IEAuthTupleTag, tuple.Bytes()[1:])
	}

	s.pub.PublishSQNUpdate(auc.AUCID, auc.SQN+int64(numVec)*32)

	if err := ipaWriteGSUP(conn, proto, resp.Bytes()); err != nil {
		s.log.Error("gsup: AIR write failed", zap.String("imsi", imsi), zap.Error(err))
		return
	}
	s.log.Info("gsup: AIR success",
		zap.String("imsi", imsi),
		zap.String("peer", peerName),
		zap.Int("vectors", numVec),
	)
}

// derive2GKc derives the GSM session key KC from Milenage CK and IK.
// KC = CK[0:8] XOR CK[8:16] XOR IK[0:8] XOR IK[8:16]  (3GPP TS 55.205 A.4)
func derive2GKc(ck, ik []byte) []byte {
	kc := make([]byte, 8)
	for i := 0; i < 8; i++ {
		kc[i] = ck[i] ^ ck[i+8] ^ ik[i] ^ ik[i+8]
	}
	return kc
}

// ── Update Location (ULR) ─────────────────────────────────────────────────────

func (s *Server) handleULR(conn net.Conn, peerName string, proto byte, msg *Msg) {
	imsi, ok := extractIMSI(msg)
	if !ok {
		s.log.Warn("gsup: ULR missing IMSI", zap.String("peer", peerName))
		sendError(conn, proto, MsgUpdateLocErr, "", CauseNetworkFailure)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := s.store.GetSubscriberByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		s.log.Warn("gsup: ULR unknown IMSI", zap.String("imsi", imsi), zap.String("peer", peerName))
		sendError(conn, proto, MsgUpdateLocErr, imsi, CauseIMSIUnknown)
		return
	}
	if err != nil {
		s.log.Error("gsup: ULR db error", zap.String("imsi", imsi), zap.Error(err))
		sendError(conn, proto, MsgUpdateLocErr, imsi, CauseNetworkFailure)
		return
	}
	if sub.Enabled != nil && !*sub.Enabled {
		s.log.Warn("gsup: ULR subscriber disabled", zap.String("imsi", imsi))
		sendError(conn, proto, MsgUpdateLocErr, imsi, CauseServiceNotAllowed)
		return
	}

	// Update serving VLR and MSC — in OsmoMSC these are co-located so peerName
	// is the same identity for both.
	ts := time.Now().UTC()
	_ = s.store.UpdateServingVLR(ctx, imsi, &repository.ServingVLRUpdate{
		ServingVLR: &peerName,
		Timestamp:  &ts,
	})
	_ = s.store.UpdateServingMSC(ctx, imsi, &repository.ServingMSCUpdate{
		ServingMSC: &peerName,
		Timestamp:  &ts,
	})
	s.pub.PublishServingVLR(geored.PayloadServingVLR{IMSI: imsi, ServingVLR: &peerName, Timestamp: &ts})
	s.pub.PublishServingMSC(geored.PayloadServingMSC{IMSI: imsi, ServingMSC: &peerName, Timestamp: &ts})

	// Send ULA (UpdateLocationResult) first.
	// Echo back the CN-Domain IE from the ULR; OsmoMSC's VLR requires this to
	// determine which domain (CS=0x02 / PS=0x01) was acknowledged and will not
	// send SGs-AP LU ACCEPT to the MME without seeing it in the ULA.
	ula := NewMsg(MsgUpdateLocRes).Add(IEIMSITag, encodeIMSI(imsi))
	cnDomain := byte(CNDomainCS) // default to CS; GSUP ULRs from OsmoMSC are always CS
	if ie := msg.Get(IECNDomain); ie != nil && len(ie.Data) > 0 {
		cnDomain = ie.Data[0]
	}
	ula.AddByte(IECNDomain, cnDomain)
	if err := ipaWriteGSUP(conn, proto, ula.Bytes()); err != nil {
		s.log.Error("gsup: ULR write ULA failed", zap.String("imsi", imsi), zap.Error(err))
		return
	}

	// Then push InsertSubscriberData with MSISDN and PDP/APN info.
	s.sendISD(conn, peerName, proto, sub, cnDomain)

	s.log.Info("gsup: ULR success",
		zap.String("imsi", imsi),
		zap.String("peer", peerName),
	)
}

// sendISD pushes subscriber data (MSISDN) to the peer via ISD.
// cnDomain must match the domain from the ULR (CNDomainCS or CNDomainPS).
func (s *Server) sendISD(conn net.Conn, peerName string, proto byte, sub *models.Subscriber, cnDomain byte) {
	isd := NewMsg(MsgInsertDataReq).
		Add(IEIMSITag, encodeIMSI(sub.IMSI)).
		AddByte(IECNDomain, cnDomain)

	if sub.MSISDN != nil && *sub.MSISDN != "" {
		isd.Add(IEMSISDNTag, encodeMSISDN(*sub.MSISDN))
	}

	// Send ISD -- peer will reply with InsertDataResult (handled as no-op).
	if err := ipaWriteGSUP(conn, proto, isd.Bytes()); err != nil {
		s.log.Error("gsup: ISD write failed",
			zap.String("imsi", sub.IMSI),
			zap.String("peer", peerName),
			zap.Error(err),
		)
	}
}

// ── Purge MS (PUR) ────────────────────────────────────────────────────────────

func (s *Server) handlePUR(conn net.Conn, peerName string, proto byte, msg *Msg) {
	imsi, ok := extractIMSI(msg)
	if !ok {
		s.log.Warn("gsup: PUR missing IMSI", zap.String("peer", peerName))
		sendError(conn, proto, MsgPurgeMSErr, "", CauseNetworkFailure)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = s.store.UpdateServingVLR(ctx, imsi, &repository.ServingVLRUpdate{
		ServingVLR: nil,
		Timestamp:  nil,
	})
	_ = s.store.UpdateServingMSC(ctx, imsi, &repository.ServingMSCUpdate{
		ServingMSC: nil,
		Timestamp:  nil,
	})

	resp := NewMsg(MsgPurgeMSRes).Add(IEIMSITag, encodeIMSI(imsi))
	if err := ipaWriteGSUP(conn, proto, resp.Bytes()); err != nil {
		s.log.Error("gsup: PUR write failed", zap.String("imsi", imsi), zap.Error(err))
		return
	}
	s.log.Info("gsup: PUR success", zap.String("imsi", imsi), zap.String("peer", peerName))
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func extractIMSI(msg *Msg) (string, bool) {
	ie := msg.Get(IEIMSITag)
	if ie == nil || len(ie.Data) == 0 {
		return "", false
	}
	return decodeIMSI(ie.Data), true
}

func sendError(conn net.Conn, proto byte, msgType byte, imsi string, cause byte) {
	b := NewMsg(msgType)
	if imsi != "" {
		b.Add(IEIMSITag, encodeIMSI(imsi))
	}
	b.AddByte(IECause, cause)
	_ = ipaWriteGSUP(conn, proto, b.Bytes())
}

// encodeMSISDN encodes an MSISDN string into semi-octet (BCD) format
// with a leading 0x91 type-of-address byte (international, E.164).
func encodeMSISDN(msisdn string) []byte {
	// Strip leading + if present
	if len(msisdn) > 0 && msisdn[0] == '+' {
		msisdn = msisdn[1:]
	}
	if len(msisdn)%2 != 0 {
		msisdn += "F"
	}
	result := make([]byte, 1+len(msisdn)/2)
	result[0] = 0x91 // TON/NPI: international, E.164
	for i := 0; i < len(msisdn); i += 2 {
		lo := nibble(msisdn[i])
		hi := nibble(msisdn[i+1])
		result[1+i/2] = (hi << 4) | lo
	}
	return result
}

func nibble(c byte) byte {
	if c >= '0' && c <= '9' {
		return c - '0'
	}
	return 0xF
}
