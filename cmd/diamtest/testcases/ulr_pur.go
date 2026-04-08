package testcases

import (
	"fmt"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"
)

// ── ULR ──────────────────────────────────────────────────────────────────────

// SendULR sends an Update-Location-Request and prints the result.
func SendULR(cfg *Config, imsi, mcc, mnc string, ratType uint32) error {
	plmn, err := encodePLMN(mcc, mnc)
	if err != nil {
		return err
	}

	c, err := connect(cfg)
	if err != nil {
		return err
	}
	defer c.close()

	answerCh := make(chan *diam.Message, 1)
	c.mux.HandleFunc(diam.ULA, func(_ diam.Conn, msg *diam.Message) {
		answerCh <- msg
	})

	req := buildULR(cfg, imsi, plmn, ratType)
	if err := c.send(req); err != nil {
		return fmt.Errorf("send ULR: %w", err)
	}

	select {
	case ans := <-answerCh:
		return parseULA(cfg, ans)
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for ULA")
	}
}

func buildULR(cfg *Config, imsi string, plmn []byte, ratType uint32) *diam.Message {
	req := diam.NewRequest(diam.UpdateLocation, appIDS6a, nil)

	req.NewAVP(avp.SessionID, avp.Mbit, 0, sessionID(cfg.OriginHost))
	req.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(cfg.OriginHost))
	req.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(cfg.OriginRealm))
	req.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity("epc.test.net"))
	req.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))
	req.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	req.NewAVP(avp.RATType, avp.Mbit|avp.Vbit, vendor3GPP, datatype.Unsigned32(ratType))
	req.NewAVP(avp.ULRFlags, avp.Mbit|avp.Vbit, vendor3GPP, datatype.Unsigned32(34)) // S6a indicator + Initial attach
	req.NewAVP(avp.VisitedPLMNID, avp.Mbit|avp.Vbit, vendor3GPP, datatype.OctetString(plmn))
	req.NewAVP(avp.VendorSpecificApplicationID, avp.Mbit, 0, &diam.GroupedAVP{
		AVP: []*diam.AVP{
			diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(vendor3GPP)),
			diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(appIDS6a)),
		},
	})

	return req
}

func parseULA(cfg *Config, msg *diam.Message) error {
	rc, ok := getResultCode(msg)
	if !ok {
		return fmt.Errorf("ULA: no result code")
	}
	if rc != ResultSuccess {
		cfg.Log.Error("ULA FAILED",
			zap.String("result", resultName(rc)),
			zap.Uint32("code", rc),
		)
		return fmt.Errorf("ULA failed: %s", resultName(rc))
	}

	// Check ULA-Flags
	ulaFlagsAVP, _ := msg.FindAVP(avp.ULAFlags, vendor3GPP)

	// Count APNs in Subscription-Data
	apnCount := 0
	var msisdn string
	if subDataAVP, err := msg.FindAVP(avp.SubscriptionData, vendor3GPP); err == nil {
		if grouped, ok := subDataAVP.Data.(*diam.GroupedAVP); ok {
			for _, child := range grouped.AVP {
				if child.Code == avp.APNConfigurationProfile {
					if profile, ok := child.Data.(*diam.GroupedAVP); ok {
						for _, profileChild := range profile.AVP {
							if profileChild.Code == avp.APNConfiguration {
								apnCount++
								if cfg.Verbose {
									printAPNConfig(cfg, apnCount, profileChild)
								}
							}
						}
					}
				}
				if child.Code == avp.MSISDN {
					if v, ok := child.Data.(datatype.OctetString); ok {
						msisdn = fmt.Sprintf("%X", []byte(v))
					}
				}
			}
		}
	}

	fields := []zap.Field{
		zap.String("result", resultName(rc)),
		zap.Int("apns", apnCount),
	}
	if ulaFlagsAVP != nil {
		if v, ok := ulaFlagsAVP.Data.(datatype.Unsigned32); ok {
			fields = append(fields, zap.Uint32("ula_flags", uint32(v)))
		}
	}
	if msisdn != "" {
		fields = append(fields, zap.String("msisdn_hex", msisdn))
	}

	cfg.Log.Info("✓ ULA SUCCESS", fields...)
	return nil
}

func printAPNConfig(cfg *Config, num int, apnAVP *diam.AVP) {
	grouped, ok := apnAVP.Data.(*diam.GroupedAVP)
	if !ok {
		return
	}
	fields := []zap.Field{zap.Int("apn_config", num)}
	for _, child := range grouped.AVP {
		switch child.Code {
		case avp.ServiceSelection:
			if v, ok := child.Data.(datatype.UTF8String); ok {
				fields = append(fields, zap.String("apn", string(v)))
			}
		case avp.PDNType:
			if v, ok := child.Data.(datatype.Enumerated); ok {
				fields = append(fields, zap.Int32("pdn_type", int32(v)))
			}
		case avp.ContextIdentifier:
			if v, ok := child.Data.(datatype.Unsigned32); ok {
				fields = append(fields, zap.Uint32("context_id", uint32(v)))
			}
		}
	}
	cfg.Log.Debug("APN-Configuration", fields...)
}

// ── PUR ──────────────────────────────────────────────────────────────────────

// SendPUR sends a Purge-UE-Request and prints the result.
func SendPUR(cfg *Config, imsi string) error {
	c, err := connect(cfg)
	if err != nil {
		return err
	}
	defer c.close()

	answerCh := make(chan *diam.Message, 1)
	c.mux.HandleFunc(diam.PUA, func(_ diam.Conn, msg *diam.Message) {
		answerCh <- msg
	})

	req := buildPUR(cfg, imsi)
	if err := c.send(req); err != nil {
		return fmt.Errorf("send PUR: %w", err)
	}

	select {
	case ans := <-answerCh:
		return parsePUA(cfg, ans)
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for PUA")
	}
}

func buildPUR(cfg *Config, imsi string) *diam.Message {
	req := diam.NewRequest(diam.PurgeUE, appIDS6a, nil)

	req.NewAVP(avp.SessionID, avp.Mbit, 0, sessionID(cfg.OriginHost))
	req.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(cfg.OriginHost))
	req.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(cfg.OriginRealm))
	req.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity("epc.test.net"))
	req.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))
	req.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	req.NewAVP(avp.VendorSpecificApplicationID, avp.Mbit, 0, &diam.GroupedAVP{
		AVP: []*diam.AVP{
			diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(vendor3GPP)),
			diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(appIDS6a)),
		},
	})

	return req
}

func parsePUA(cfg *Config, msg *diam.Message) error {
	rc, ok := getResultCode(msg)
	if !ok {
		return fmt.Errorf("PUA: no result code")
	}
	if rc != ResultSuccess {
		cfg.Log.Error("PUA FAILED",
			zap.String("result", resultName(rc)),
			zap.Uint32("code", rc),
		)
		return fmt.Errorf("PUA failed: %s", resultName(rc))
	}
	cfg.Log.Info("✓ PUA SUCCESS", zap.String("result", resultName(rc)))
	return nil
}
