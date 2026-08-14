package testcases

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
	"github.com/fiorix/go-diameter/v4/diam/sm"

	"github.com/svinson1121/vectorcore-hss/internal/diameter/basedict"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/gx"
)

// gxClient is the Gx-application analogue of client (client.go), used to
// simulate a PGW/PCEF peer sending CCR-I during the storm test. It needs its
// own dial function because its CER must advertise the Gx application ID,
// not S6a's — go-diameter's sm.Client is single-application.
type gxClient struct {
	conn diam.Conn
	mux  *sm.StateMachine
}

// gxDictOnce guards dictionary setup so it runs exactly once per process.
// This must be called synchronously, before any goroutine dials a
// connection (S6a or Gx) — see ensureGxDict's doc comment for why.
var (
	gxDictOnce sync.Once
	gxDictErr  error
)

// ensureGxDict prepares dict.Default so a Gx CCR-I can be sent. Two things
// make this non-obvious:
//
//  1. go-diameter's own dict package auto-registers a built-in Gx
//     dictionary (app 16777238, command 272) at package init time, in
//     every process that imports it. internal/diameter/gx.LoadDict()
//     defines its own, different, version of the same (app, command) pair,
//     which always collides with that built-in one ("index exists") unless
//     dict.Default has first been reset. The real hss server works around
//     this by calling basedict.Load() before any other *.LoadDict() call
//     (see internal/diameter/server.go / internal/diameter/basedict) — this
//     tool has to do the same to behave like the real server.
//  2. basedict.Load() replaces the dict.Default package variable outright
//     (dict.Default = fresh), which is an unsynchronized write to a shared
//     global. Every S6a client connection (client.go's connect) reads that
//     same variable. So this function MUST be called to completion before
//     any connect()/dialGx() goroutines start — sync.Once only serializes
//     concurrent callers of this function, it does not protect unrelated
//     readers elsewhere, so callers must still call this from RunStorm
//     before spawning any client goroutines rather than lazily from within
//     one.
func ensureGxDict() error {
	gxDictOnce.Do(func() {
		if err := basedict.Load(); err != nil {
			gxDictErr = fmt.Errorf("reset dict.Default: %w", err)
			return
		}
		gxDictErr = gx.LoadDict()
	})
	if gxDictErr != nil {
		return fmt.Errorf("load gx dictionary: %w", gxDictErr)
	}
	return nil
}

func dialGx(cfg *Config) (*gxClient, error) {
	if err := ensureGxDict(); err != nil {
		return nil, err
	}

	settings := &sm.Settings{
		OriginHost:       datatype.DiameterIdentity(cfg.OriginHost),
		OriginRealm:      datatype.DiameterIdentity(cfg.OriginRealm),
		VendorID:         datatype.Unsigned32(vendor3GPP),
		ProductName:      datatype.UTF8String("diamtest-gx"),
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
					diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(gx.AppIDGx)),
					diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(vendor3GPP)),
				},
			}),
		},
	}

	conn, err := smClient.DialNetwork("tcp", cfg.HSSAddr)
	if err != nil {
		return nil, fmt.Errorf("connect (gx) to %s: %w", cfg.HSSAddr, err)
	}
	return &gxClient{conn: conn, mux: mux}, nil
}

func (c *gxClient) close() { c.conn.Close() }

// buildCCRI assembles a Gx Credit-Control-Request (Initial) matching the AVP
// set internal/diameter/gx.CCR unmarshals (Session-Id, Origin-Host/Realm,
// CC-Request-Type/Number, Subscription-Id/IMSI, Called-Station-Id/APN,
// Framed-IP-Address, RAT-Type).
func buildCCRI(cfg *Config, sid, imsi, apnName string, ueIP net.IP) *diam.Message {
	req := diam.NewRequest(diam.CreditControl, gx.AppIDGx, nil)
	req.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
	req.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(cfg.OriginHost))
	req.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(cfg.OriginRealm))
	req.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity("epc.test.net"))
	req.NewAVP(avp.CCRequestType, avp.Mbit, 0, datatype.Enumerated(gx.CCRequestTypeInitial))
	req.NewAVP(avp.CCRequestNumber, avp.Mbit, 0, datatype.Unsigned32(0))
	req.NewAVP(avp.SubscriptionID, avp.Mbit, 0, &diam.GroupedAVP{
		AVP: []*diam.AVP{
			diam.NewAVP(avp.SubscriptionIDType, avp.Mbit, 0, datatype.Enumerated(gx.SubscriptionIDTypeIMSI)),
			diam.NewAVP(avp.SubscriptionIDData, avp.Mbit, 0, datatype.UTF8String(imsi)),
		},
	})
	req.NewAVP(avp.CalledStationID, avp.Mbit, 0, datatype.UTF8String(apnName))
	req.NewAVP(avp.FramedIPAddress, avp.Mbit, 0, datatype.OctetString(ueIP.To4()))
	req.NewAVP(avp.RATType, avp.Mbit|avp.Vbit, vendor3GPP, datatype.Unsigned32(1004)) // EUTRAN
	req.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(gx.AppIDGx))
	req.NewAVP(avp.VendorSpecificApplicationID, avp.Mbit, 0, &diam.GroupedAVP{
		AVP: []*diam.AVP{
			diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(vendor3GPP)),
			diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(gx.AppIDGx)),
		},
	})
	return req
}

// sendCCRI dials its own Gx connection, sends one CCR-I, and reports the
// Result-Code and round-trip latency. A fresh connection per call mirrors
// how a PGW would establish (or already have) its own peer session distinct
// from the S6a MME/SGSN peers the storm test's AIR/ULR traffic uses.
func sendCCRI(cfg *Config, imsi, apnName string, ueIP net.IP, timeout time.Duration) (uint32, time.Duration, error) {
	c, err := dialGx(cfg)
	if err != nil {
		return 0, 0, err
	}
	defer c.close()

	answerCh := make(chan *diam.Message, 1)
	c.mux.HandleFunc(diam.CCA, func(_ diam.Conn, msg *diam.Message) {
		answerCh <- msg
	})

	sid := sessionID(cfg.OriginHost)
	req := buildCCRI(cfg, string(sid), imsi, apnName, ueIP)

	start := time.Now()
	if _, err := req.WriteTo(c.conn); err != nil {
		return 0, 0, fmt.Errorf("send CCR-I: %w", err)
	}

	select {
	case ans := <-answerCh:
		lat := time.Since(start)
		rc, ok := getResultCode(ans)
		if !ok {
			return 0, lat, fmt.Errorf("CCA: no result code in response")
		}
		return rc, lat, nil
	case <-time.After(timeout):
		return 0, 0, errLoadTimeout
	}
}
