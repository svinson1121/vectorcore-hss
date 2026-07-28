package slh

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"testing"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
	"github.com/fiorix/go-diameter/v4/diam/sm"
	"go.uber.org/zap"
	xcontext "golang.org/x/net/context"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/tbcd"
	"github.com/svinson1121/vectorcore-hss/internal/models"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

type testStore struct {
	byIMSI, byMSISDN map[string]*models.Subscriber
	err              error
}

type ceaConn struct {
	bytes.Buffer
	ctx xcontext.Context
}

func (c *ceaConn) WriteStream(b []byte, _ uint) (int, error) { return c.Write(b) }
func (c *ceaConn) Close()                                    {}
func (c *ceaConn) LocalAddr() net.Addr                       { return testAddr("local") }
func (c *ceaConn) RemoteAddr() net.Addr                      { return testAddr("remote") }
func (c *ceaConn) TLS() *tls.ConnectionState                 { return nil }
func (c *ceaConn) Dictionary() *dict.Parser                  { return dict.Default }
func (c *ceaConn) Context() xcontext.Context {
	if c.ctx == nil {
		return xcontext.Background()
	}
	return c.ctx
}
func (c *ceaConn) SetContext(ctx xcontext.Context) { c.ctx = ctx }
func (c *ceaConn) Connection() net.Conn            { return nil }

type testAddr string

func (a testAddr) Network() string { return "tcp" }
func (a testAddr) String() string  { return string(a) }

func (s *testStore) GetSubscriberByIMSI(_ context.Context, v string) (*models.Subscriber, error) {
	if s.err != nil {
		return nil, s.err
	}
	x, ok := s.byIMSI[v]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return x, nil
}
func (s *testStore) GetSubscriberByMSISDN(_ context.Context, v string) (*models.Subscriber, error) {
	if s.err != nil {
		return nil, s.err
	}
	x, ok := s.byMSISDN[v]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return x, nil
}

func ptr(s string) *string { return &s }
func newHandler(s *testStore) *Handlers {
	return NewHandlers(&config.Config{HSS: config.HSSConfig{OriginHost: "hss.example", OriginRealm: "example", SLhAuthorizedRealms: []string{"gmlc.example"}}}, s, zap.NewNop())
}
func request(imsi string, number []byte, realm string) *diam.Message {
	m := diam.NewRequest(8388622, AppIDSLh, dict.Default)
	m.Header.HopByHopID = 11
	m.Header.EndToEndID = 22
	m.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String("session"))
	m.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	m.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity("gmlc"))
	m.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(realm))
	m.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity("example"))
	if imsi != "" {
		m.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))
	}
	if len(number) > 0 {
		m.NewAVP(avpMSISDN, avp.Vbit, Vendor3GPP, datatype.OctetString(number))
	}
	return m
}
func code(t *testing.T, m *diam.Message) (uint32, bool) {
	t.Helper()
	a, e := m.FindAVP(avp.ResultCode, 0)
	if e != nil {
		return 0, false
	}
	return uint32(a.Data.(datatype.Unsigned32)), true
}
func expCode(t *testing.T, m *diam.Message) uint32 {
	t.Helper()
	a, e := m.FindAVP(avp.ExperimentalResult, 0)
	if e != nil {
		t.Fatal(e)
	}
	g := a.Data.(*diam.GroupedAVP)
	for _, x := range g.AVP {
		if x.Code == avp.ExperimentalResultCode {
			return uint32(x.Data.(datatype.Unsigned32))
		}
	}
	t.Fatal("no experimental result code")
	return 0
}

func TestRIRIdentityAndRouting(t *testing.T) {
	if err := LoadDict(); err != nil {
		t.Fatal(err)
	}
	sub := &models.Subscriber{IMSI: "001010000000001", MSISDN: ptr("15551234567"), ServingMME: ptr("mme.example"), ServingMMERealm: ptr("mme.realm")}
	s := &testStore{byIMSI: map[string]*models.Subscriber{sub.IMSI: sub}, byMSISDN: map[string]*models.Subscriber{"15551234567": sub}}
	b, _ := tbcd.EncodeMSISDN(*sub.MSISDN)
	for _, tc := range []struct {
		name, imsi string
		number     []byte
		wantUser   bool
	}{{"imsi", sub.IMSI, nil, false}, {"msisdn", "", b, true}, {"both", sub.IMSI, b, true}} {
		t.Run(tc.name, func(t *testing.T) {
			req := request(tc.imsi, tc.number, "gmlc.example")
			ans, err := newHandler(s).LRR(nil, req)
			if err != nil {
				t.Fatal(err)
			}
			if got, ok := code(t, ans); !ok || got != diam.Success {
				t.Fatalf("Result-Code=%d present=%v", got, ok)
			}
			if _, e := ans.FindAVP(avp.ExperimentalResult, 0); e == nil {
				t.Fatal("unexpected Experimental-Result")
			}
			if _, e := ans.FindAVP(avpServingNode, Vendor3GPP); e != nil {
				t.Fatal("missing Serving-Node")
			}
			if tc.wantUser {
				if _, e := ans.FindAVP(avp.UserName, 0); e != nil {
					t.Fatal("missing User-Name")
				}
			}
			raw := bytes.Buffer{}
			if _, e := ans.WriteTo(&raw); e != nil {
				t.Fatal(e)
			}
			wire, e := diam.ReadMessage(&raw, dict.Default)
			if e != nil {
				t.Fatal(e)
			}
			if wire.Header.HopByHopID != 11 || wire.Header.EndToEndID != 22 || wire.Header.ApplicationID != AppIDSLh || wire.Header.CommandFlags&diam.RequestFlag != 0 {
				t.Fatal("answer header not preserved")
			}
		})
	}
}

func TestRIRFailures(t *testing.T) {
	if err := LoadDict(); err != nil {
		t.Fatal(err)
	}
	base := &models.Subscriber{IMSI: "1", MSISDN: ptr("1"), ServingMME: ptr("mme"), ServingMMERealm: ptr("realm")}
	other := &models.Subscriber{IMSI: "2", MSISDN: ptr("2"), ServingMME: ptr("mme"), ServingMMERealm: ptr("realm")}
	for _, tc := range []struct {
		name, realm, imsi string
		b                 []byte
		store             *testStore
		result            uint32
		experimental      bool
	}{
		{"unknown", "gmlc.example", "x", nil, &testStore{byIMSI: map[string]*models.Subscriber{}, byMSISDN: map[string]*models.Subscriber{}}, 5001, true},
		{"absent", "gmlc.example", "1", nil, &testStore{byIMSI: map[string]*models.Subscriber{"1": {IMSI: "1"}}, byMSISDN: map[string]*models.Subscriber{}}, 4201, true},
		{"unauthorized", "outside", "1", nil, &testStore{byIMSI: map[string]*models.Subscriber{"1": base}, byMSISDN: map[string]*models.Subscriber{}}, 5490, true},
		{"database", "gmlc.example", "1", nil, &testStore{err: errors.New("database down")}, diam.UnableToComply, false},
		{"missing", "gmlc.example", "", nil, &testStore{}, diam.UnableToComply, false},
		{"invalid tbcd", "gmlc.example", "", []byte{0x1a}, &testStore{}, diam.UnableToComply, false},
		{"contradicting", "gmlc.example", "1", mustTBCD(t, "2"), &testStore{byIMSI: map[string]*models.Subscriber{"1": base}, byMSISDN: map[string]*models.Subscriber{"2": other}}, diam.ContradictingAVPs, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ans, _ := newHandler(tc.store).LRR(nil, request(tc.imsi, tc.b, tc.realm))
			if tc.experimental {
				if got := expCode(t, ans); got != tc.result {
					t.Fatalf("experimental=%d", got)
				}
				if _, ok := code(t, ans); ok {
					t.Fatal("both result forms")
				}
			} else if got, ok := code(t, ans); !ok || got != tc.result {
				t.Fatalf("result=%d", got)
			}
			if _, e := ans.FindAVP(avpServingNode, Vendor3GPP); e == nil {
				t.Fatal("routing data in failure")
			}
		})
	}
}
func mustTBCD(t *testing.T, s string) []byte {
	t.Helper()
	b, e := tbcd.EncodeMSISDN(s)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestTBCDRoundTripAndRejectsInvalid(t *testing.T) {
	for _, n := range []string{"1", "12", "15551234567"} {
		b, e := tbcd.EncodeMSISDN(n)
		if e != nil {
			t.Fatal(e)
		}
		got, e := tbcd.DecodeMSISDN(b)
		if e != nil || got != n {
			t.Fatalf("%q -> %q %v", n, got, e)
		}
	}
	if _, e := tbcd.DecodeMSISDN([]byte{0xf1, 0x21}); e == nil {
		t.Fatal("accepted invalid filler")
	}
}

func TestCEAAdvertisesSLh(t *testing.T) {
	if err := LoadDict(); err != nil {
		t.Fatal(err)
	}
	machine := sm.New(&sm.Settings{OriginHost: "hss.example", OriginRealm: "example", VendorID: datatype.Unsigned32(Vendor3GPP), ProductName: "test", HostIPAddresses: []datatype.Address{datatype.Address(net.ParseIP("127.0.0.1"))}})
	c := &ceaConn{}
	cer := diam.NewRequest(diam.CapabilitiesExchange, 0, dict.Default)
	cer.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity("gmlc.example"))
	cer.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity("example"))
	cer.NewAVP(avp.HostIPAddress, avp.Mbit, 0, datatype.Address(net.ParseIP("127.0.0.1")))
	cer.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(Vendor3GPP))
	cer.NewAVP(avp.ProductName, 0, 0, datatype.UTF8String("test"))
	cer.NewAVP(avp.VendorSpecificApplicationID, avp.Mbit, 0, &diam.GroupedAVP{AVP: []*diam.AVP{
		diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(Vendor3GPP)),
		diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(AppIDSLh)),
	}})
	machine.ServeDIAM(c, cer)
	cea, err := diam.ReadMessage(bytes.NewReader(c.Bytes()), dict.Default)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cea.FindAVP(avp.SupportedVendorID, 0); err != nil {
		t.Fatal("CEA lacks 3GPP Supported-Vendor-Id")
	}
	vs, err := cea.FindAVPs(avp.VendorSpecificApplicationID, 0)
	if err != nil {
		t.Fatal("CEA lacks vendor applications")
	}
	found := false
	for _, v := range vs {
		for _, child := range v.Data.(*diam.GroupedAVP).AVP {
			if child.Code == avp.AuthApplicationID && uint32(child.Data.(datatype.Unsigned32)) == AppIDSLh {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("CEA lacks SLh Auth-Application-Id")
	}
}
