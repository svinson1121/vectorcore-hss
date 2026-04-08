// Package testcases implements Diameter S6a test cases for VectorCore HSS.
package testcases

import (
	"fmt"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
	"github.com/fiorix/go-diameter/v4/diam/sm"
	"go.uber.org/zap"
)

const (
	vendor3GPP = uint32(10415)
	appIDS6a   = uint32(16777251)

	ResultSuccess                         = uint32(2001)
	ExperimentalResultUserUnknown         = uint32(5001)
	ExperimentalResultUnknownEPS          = uint32(5004)
	ExperimentalResultAuthDataUnavailable = uint32(4181)
)

// Config holds connection settings for the test client.
type Config struct {
	HSSAddr     string
	OriginHost  string
	OriginRealm string
	MCC         string
	MNC         string
	Log         *zap.Logger
	Verbose     bool
}

// client wraps a go-diameter connection to the HSS.
type client struct {
	cfg  *Config
	conn diam.Conn
	mux  *sm.StateMachine
}

func connect(cfg *Config) (*client, error) {
	settings := &sm.Settings{
		OriginHost:       datatype.DiameterIdentity(cfg.OriginHost),
		OriginRealm:      datatype.DiameterIdentity(cfg.OriginRealm),
		VendorID:         datatype.Unsigned32(vendor3GPP),
		ProductName:      datatype.UTF8String("diamtest"),
		OriginStateID:    datatype.Unsigned32(uint32(time.Now().Unix())),
		FirmwareRevision: 1,
	}

	mux := sm.New(settings)

	smClient := &sm.Client{
		Dict:               dict.Default,
		Handler:            mux,
		MaxRetransmits:     1,
		RetransmitInterval: time.Second,
		EnableWatchdog:     false,
		SupportedVendorID: []*diam.AVP{
			diam.NewAVP(avp.SupportedVendorID, avp.Mbit, 0, datatype.Unsigned32(vendor3GPP)),
		},
		VendorSpecificApplicationID: []*diam.AVP{
			diam.NewAVP(avp.VendorSpecificApplicationID, avp.Mbit, 0, &diam.GroupedAVP{
				AVP: []*diam.AVP{
					diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(appIDS6a)),
					diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(vendor3GPP)),
				},
			}),
		},
	}

	conn, err := smClient.DialNetwork("tcp", cfg.HSSAddr)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", cfg.HSSAddr, err)
	}

	cfg.Log.Info("connected to HSS", zap.String("addr", cfg.HSSAddr))
	return &client{cfg: cfg, conn: conn, mux: mux}, nil
}

func (c *client) close() { c.conn.Close() }

func (c *client) send(msg *diam.Message) error {
	if c.cfg.Verbose {
		c.cfg.Log.Debug("sending", zap.String("msg", msg.String()))
	}
	_, err := msg.WriteTo(c.conn)
	return err
}

func sessionID(originHost string) datatype.UTF8String {
	return datatype.UTF8String(fmt.Sprintf("%s;%d;%d",
		originHost, time.Now().Unix(), time.Now().UnixNano()&0xFFFF))
}

// encodePLMN encodes MCC and MNC strings into the 3-byte BCD format
// defined in 3GPP TS 24.008 §10.5.1.13.
//
//   MCC=001, MNC=01  → [0x00, 0xF1, 0x10]  (2-digit MNC, F is filler)
//   MCC=001, MNC=001 → [0x00, 0x11, 0x00]  (3-digit MNC)
//   MCC=234, MNC=30  → [0x32, 0xF4, 0x03]
func encodePLMN(mcc, mnc string) ([]byte, error) {
	if len(mcc) != 3 {
		return nil, fmt.Errorf("MCC must be 3 digits, got %q", mcc)
	}
	if len(mnc) != 2 && len(mnc) != 3 {
		return nil, fmt.Errorf("MNC must be 2 or 3 digits, got %q", mnc)
	}
	for _, s := range []string{mcc, mnc} {
		for _, c := range s {
			if c < '0' || c > '9' {
				return nil, fmt.Errorf("MCC/MNC must be digits only, got %q", s)
			}
		}
	}

	// nibble returns the digit at position i, or 0xF (filler) if out of range
	nibble := func(s string, i int) byte {
		if i >= len(s) {
			return 0xF
		}
		return s[i] - '0'
	}

	b := make([]byte, 3)
	b[0] = nibble(mcc, 1)<<4 | nibble(mcc, 0) // MCC digit2 | MCC digit1
	b[1] = nibble(mnc, 2)<<4 | nibble(mcc, 2) // MNC digit3 | MCC digit3
	b[2] = nibble(mnc, 1)<<4 | nibble(mnc, 0) // MNC digit2 | MNC digit1
	return b, nil
}

func getResultCode(msg *diam.Message) (uint32, bool) {
	if a, err := msg.FindAVP(avp.ResultCode, 0); err == nil {
		if rc, ok := a.Data.(datatype.Unsigned32); ok {
			return uint32(rc), true
		}
	}
	if a, err := msg.FindAVP(avp.ExperimentalResult, 0); err == nil {
		if g, ok := a.Data.(*diam.GroupedAVP); ok {
			for _, child := range g.AVP {
				if child.Code == avp.ExperimentalResultCode {
					if rc, ok := child.Data.(datatype.Unsigned32); ok {
						return uint32(rc), true
					}
				}
			}
		}
	}
	return 0, false
}

func resultName(code uint32) string {
	switch code {
	case 2001:
		return "DIAMETER_SUCCESS"
	case 5001:
		return "DIAMETER_ERROR_USER_UNKNOWN"
	case 5004:
		return "DIAMETER_ERROR_UNKNOWN_EPS_SUBSCRIPTION"
	case 4181:
		return "DIAMETER_AUTHENTICATION_DATA_UNAVAILABLE"
	case 3001:
		return "DIAMETER_COMMAND_UNSUPPORTED"
	case 5012:
		return "DIAMETER_UNABLE_TO_COMPLY"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", code)
	}
}
