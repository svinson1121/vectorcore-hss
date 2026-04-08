package testcases

import (
	"fmt"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"
)

// SendAIR sends an Authentication-Information-Request and prints the result.
func SendAIR(cfg *Config, imsi, mcc, mnc string, numVectors uint32) error {
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
	c.mux.HandleFunc(diam.AIA, func(_ diam.Conn, msg *diam.Message) {
		answerCh <- msg
	})

	req := buildAIR(cfg, imsi, plmn, numVectors)
	if err := c.send(req); err != nil {
		return fmt.Errorf("send AIR: %w", err)
	}

	select {
	case ans := <-answerCh:
		return parseAIA(cfg, ans)
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for AIA")
	}
}

func buildAIR(cfg *Config, imsi string, plmn []byte, numVectors uint32) *diam.Message {
	req := diam.NewRequest(diam.AuthenticationInformation, appIDS6a, nil)
	sid := sessionID(cfg.OriginHost)

	req.NewAVP(avp.SessionID, avp.Mbit, 0, sid)
	req.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(cfg.OriginHost))
	req.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(cfg.OriginRealm))
	req.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity("epc.test.net"))
	req.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))
	req.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	req.NewAVP(avp.VisitedPLMNID, avp.Mbit|avp.Vbit, vendor3GPP, datatype.OctetString(plmn))
	req.NewAVP(avp.RequestedEUTRANAuthenticationInfo, avp.Mbit|avp.Vbit, vendor3GPP, &diam.GroupedAVP{
		AVP: []*diam.AVP{
			diam.NewAVP(avp.NumberOfRequestedVectors, avp.Mbit|avp.Vbit, vendor3GPP, datatype.Unsigned32(numVectors)),
			diam.NewAVP(avp.ImmediateResponsePreferred, avp.Mbit|avp.Vbit, vendor3GPP, datatype.Unsigned32(0)),
		},
	})
	req.NewAVP(avp.VendorSpecificApplicationID, avp.Mbit, 0, &diam.GroupedAVP{
		AVP: []*diam.AVP{
			diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(vendor3GPP)),
			diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(appIDS6a)),
		},
	})

	return req
}

func parseAIA(cfg *Config, msg *diam.Message) error {
	rc, ok := getResultCode(msg)
	if !ok {
		return fmt.Errorf("AIA: no result code in response")
	}

	if rc != ResultSuccess {
		cfg.Log.Error("AIA FAILED",
			zap.String("result", resultName(rc)),
			zap.Uint32("code", rc),
		)
		return fmt.Errorf("AIA failed: %s", resultName(rc))
	}

	// Parse Authentication-Info grouped AVP
	authInfoAVP, err := msg.FindAVP(avp.AuthenticationInfo, vendor3GPP)
	if err != nil {
		return fmt.Errorf("AIA: no Authentication-Info AVP")
	}

	grouped, ok := authInfoAVP.Data.(*diam.GroupedAVP)
	if !ok {
		return fmt.Errorf("AIA: Authentication-Info not grouped")
	}

	vectorCount := 0
	for _, child := range grouped.AVP {
		if child.Code == avp.EUTRANVector {
			vectorCount++
			if cfg.Verbose {
				printEUTRANVector(cfg, vectorCount, child)
			}
		}
	}

	cfg.Log.Info("✓ AIA SUCCESS",
		zap.Int("vectors", vectorCount),
		zap.String("result", resultName(rc)),
	)
	return nil
}

func printEUTRANVector(cfg *Config, num int, vectorAVP *diam.AVP) {
	grouped, ok := vectorAVP.Data.(*diam.GroupedAVP)
	if !ok {
		return
	}
	fields := []zap.Field{zap.Int("vector", num)}
	for _, child := range grouped.AVP {
		switch child.Code {
		case avp.RAND:
			if v, ok := child.Data.(datatype.OctetString); ok {
				fields = append(fields, zap.String("RAND", fmt.Sprintf("%X", []byte(v))))
			}
		case avp.XRES:
			if v, ok := child.Data.(datatype.OctetString); ok {
				fields = append(fields, zap.String("XRES", fmt.Sprintf("%X", []byte(v))))
			}
		case avp.AUTN:
			if v, ok := child.Data.(datatype.OctetString); ok {
				fields = append(fields, zap.String("AUTN", fmt.Sprintf("%X", []byte(v))))
			}
		case avp.KASME:
			if v, ok := child.Data.(datatype.OctetString); ok {
				fields = append(fields, zap.String("KASME", fmt.Sprintf("%X", []byte(v))))
			}
		}
	}
	cfg.Log.Debug("E-UTRAN-Vector", fields...)
}
