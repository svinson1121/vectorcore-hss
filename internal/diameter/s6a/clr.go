package s6a

import (
	"context"
	"fmt"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

const (
	// CancellationType values (3GPP TS 29.272 §7.3.24)
	CancellationTypeMMEUpdate              = 0
	CancellationTypeSubscriptionWithdrawal = 1

	// Command code for Cancel-Location-Request (3GPP TS 29.272 §7.2.7)
	cmdCLR = 317
)

// SendCLR sends a Cancel-Location-Request to the given connection.
// Used when a subscriber attaches to a new MME and we need to notify the old one.
func (h *Handlers) SendCLR(conn diam.Conn, imsi, destHost, destRealm string) {
	sid := fmt.Sprintf("%s;%d;clr", h.originHost, time.Now().UnixNano())

	msg := diam.NewRequest(cmdCLR, AppIDS6a, nil)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1)) // NO_STATE_MAINTAINED
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(h.originHost))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(h.originRealm))
	msg.NewAVP(avp.DestinationHost, avp.Mbit, 0, datatype.DiameterIdentity(destHost))
	msg.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity(destRealm))
	msg.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))
	msg.NewAVP(avp.CancellationType, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Enumerated(CancellationTypeMMEUpdate))

	if _, err := msg.WriteTo(conn); err != nil {
		h.log.Error("s6a: CLR send failed",
			zap.String("imsi", imsi),
			zap.String("dest_host", destHost),
			zap.Error(err))
		return
	}
	h.log.Info("s6a: CLR sent",
		zap.String("imsi", imsi),
		zap.String("dest_host", destHost))
}

// SendCLRByIMSI looks up the subscriber's serving MME by IMSI and sends a CLR
// with the given cancellationType. Returns an error if the subscriber is not
// found, has no serving MME, or the peer is not currently connected.
func (h *Handlers) SendCLRByIMSI(ctx context.Context, imsi string, cancellationType int) error {
	sub, err := h.store.GetSubscriberByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		return fmt.Errorf("subscriber not found: %s", imsi)
	}
	if err != nil {
		return fmt.Errorf("db error: %w", err)
	}

	if sub.ServingMME == nil || *sub.ServingMME == "" {
		return fmt.Errorf("subscriber %s has no serving MME", imsi)
	}
	destHost := *sub.ServingMME

	destRealm := ""
	if sub.ServingMMERealm != nil {
		destRealm = *sub.ServingMMERealm
	}

	conn, ok := h.peers.GetConn(destHost)
	if !ok {
		return fmt.Errorf("MME %s is not connected", destHost)
	}

	sid := fmt.Sprintf("%s;%d;clr", h.originHost, time.Now().UnixNano())
	msg := diam.NewRequest(cmdCLR, AppIDS6a, nil)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(h.originHost))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(h.originRealm))
	msg.NewAVP(avp.DestinationHost, avp.Mbit, 0, datatype.DiameterIdentity(destHost))
	msg.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity(destRealm))
	msg.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))
	msg.NewAVP(avp.CancellationType, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Enumerated(cancellationType))

	if _, err := msg.WriteTo(conn); err != nil {
		h.log.Error("s6a: CLR send failed",
			zap.String("imsi", imsi),
			zap.String("dest_host", destHost),
			zap.Error(err))
		return fmt.Errorf("CLR send failed: %w", err)
	}
	h.log.Info("s6a: CLR sent (API)",
		zap.String("imsi", imsi),
		zap.String("dest_host", destHost),
		zap.Int("cancellation_type", cancellationType))
	return nil
}
