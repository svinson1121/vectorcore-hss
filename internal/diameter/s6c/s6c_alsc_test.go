package s6c

import (
	"bytes"
	"crypto/tls"
	"errors"
	"net"
	"testing"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
	"github.com/svinson1121/vectorcore-hss/internal/models"
	xcontext "golang.org/x/net/context"
)

type captureConn struct {
	buf      bytes.Buffer
	writeErr error
	ctx      xcontext.Context
}

func (c *captureConn) Write(b []byte) (int, error) {
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return c.buf.Write(b)
}

func (c *captureConn) WriteStream(b []byte, _ uint) (int, error) { return c.Write(b) }
func (c *captureConn) Close()                                    {}
func (c *captureConn) LocalAddr() net.Addr                       { return testAddr("local") }
func (c *captureConn) RemoteAddr() net.Addr                      { return testAddr("remote") }
func (c *captureConn) TLS() *tls.ConnectionState                 { return nil }
func (c *captureConn) Dictionary() *dict.Parser                  { return dict.Default }
func (c *captureConn) Context() xcontext.Context {
	if c.ctx == nil {
		return xcontext.Background()
	}
	return c.ctx
}
func (c *captureConn) SetContext(ctx xcontext.Context) { c.ctx = ctx }
func (c *captureConn) Connection() net.Conn            { return nil }

type testAddr string

func (a testAddr) Network() string { return "tcp" }
func (a testAddr) String() string  { return string(a) }

type peerLookupStub struct {
	conns   map[string]diam.Conn
	lookups []string
}

func (p *peerLookupStub) GetConn(originHost string) (diam.Conn, bool) {
	p.lookups = append(p.lookups, originHost)
	conn, ok := p.conns[originHost]
	return conn, ok
}

func buildASA(t *testing.T, sessionID string, resultCode uint32) *diam.Message {
	t.Helper()
	msg := diam.NewRequest(cmdALSC, AppIDS6c, dict.Default).Answer(resultCode)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sessionID))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity("smsc.test.net"))
	return msg
}

func parseWrittenMessage(t *testing.T, conn *captureConn) *diam.Message {
	t.Helper()
	msg, err := diam.ReadMessage(bytes.NewReader(conn.buf.Bytes()), dict.Default)
	if err != nil {
		t.Fatalf("read written Diameter message: %v", err)
	}
	return msg
}

func TestLoadMSISDNSupplement_Idempotent(t *testing.T) {
	if err := LoadMSISDNSupplement(); err != nil {
		t.Fatalf("first LoadMSISDNSupplement call failed: %v", err)
	}
	if err := LoadMSISDNSupplement(); err != nil {
		t.Fatalf("second LoadMSISDNSupplement call failed: %v", err)
	}
}

func TestALSC_SendUsesStoredOriginRealm(t *testing.T) {
	store := newS6cStore()
	msisdn := "12125551234"
	imsi := "001010000000100"
	store.addSubscriber(&models.Subscriber{IMSI: imsi, MSISDN: ptr(msisdn)})
	store.mwds[imsi] = []models.MessageWaitingData{{
		IMSI:          imsi,
		SCAddress:     "12125550000",
		SCOriginHost:  "smsc1.test.net",
		SCOriginRealm: "smsc.realm.net",
	}}

	conn := &captureConn{}
	peers := &peerLookupStub{conns: map[string]diam.Conn{"smsc1.test.net": conn}}
	h := newTestHandlers(store)
	h.peers = peers

	h.SendALSCForIMSI(imsi)

	msg := parseWrittenMessage(t, conn)
	if msg.Header.ApplicationID != AppIDS6c {
		t.Fatalf("Application-ID: got %d, want %d", msg.Header.ApplicationID, AppIDS6c)
	}
	if msg.Header.CommandCode != cmdALSC {
		t.Fatalf("Command-Code: got %d, want %d", msg.Header.CommandCode, cmdALSC)
	}

	dstHost := findAVPDirect(msg, avp.DestinationHost, 0)
	if got := string(dstHost.Data.(datatype.DiameterIdentity)); got != "smsc1.test.net" {
		t.Fatalf("Destination-Host: got %q, want %q", got, "smsc1.test.net")
	}
	dstRealm := findAVPDirect(msg, avp.DestinationRealm, 0)
	if got := string(dstRealm.Data.(datatype.DiameterIdentity)); got != "smsc.realm.net" {
		t.Fatalf("Destination-Realm: got %q, want %q", got, "smsc.realm.net")
	}
	originHost := findAVPDirect(msg, avp.OriginHost, 0)
	if got := string(originHost.Data.(datatype.DiameterIdentity)); got != "hss.test.net" {
		t.Fatalf("Origin-Host: got %q, want %q", got, "hss.test.net")
	}
	originRealm := findAVPDirect(msg, avp.OriginRealm, 0)
	if got := string(originRealm.Data.(datatype.DiameterIdentity)); got != "test.net" {
		t.Fatalf("Origin-Realm: got %q, want %q", got, "test.net")
	}
}

func TestALSC_SendStoresPendingSession(t *testing.T) {
	store := newS6cStore()
	imsi := "001010000000101"
	store.addSubscriber(&models.Subscriber{IMSI: imsi})
	store.mwds[imsi] = []models.MessageWaitingData{{
		IMSI:          imsi,
		SCAddress:     "12125550001",
		SCOriginHost:  "smsc1.test.net",
		SCOriginRealm: "smsc.realm.net",
	}}

	conn := &captureConn{}
	peers := &peerLookupStub{conns: map[string]diam.Conn{"smsc1.test.net": conn}}
	h := newTestHandlers(store)
	h.peers = peers

	h.SendALSCForIMSI(imsi)

	msg := parseWrittenMessage(t, conn)
	sidAVP := findAVPDirect(msg, avp.SessionID, 0)
	if sidAVP == nil {
		t.Fatal("missing Session-Id in ALSC request")
	}
	sid := string(sidAVP.Data.(datatype.UTF8String))

	entry, ok := h.pendingALSC.Load(sid)
	if !ok {
		t.Fatal("pending ALSC session not recorded")
	}
	pending := entry.(pendingALSCEntry)
	if pending.imsi != imsi {
		t.Fatalf("pending IMSI: got %q, want %q", pending.imsi, imsi)
	}
	if pending.scAddr != "12125550001" {
		t.Fatalf("pending SC-Address: got %q, want %q", pending.scAddr, "12125550001")
	}
}

func TestASA_SuccessDeletesMWD(t *testing.T) {
	store := newS6cStore()
	h := newTestHandlers(store)
	h.pendingALSC.Store("sid-success", pendingALSCEntry{imsi: "001010000000102", scAddr: "12125550002"})

	h.ASA(nil, buildASA(t, "sid-success", diamResultSuccess))

	if len(store.deletedMWD) != 1 {
		t.Fatalf("expected 1 DeleteMWD call, got %d", len(store.deletedMWD))
	}
	if got := store.deletedMWD[0]; got.imsi != "001010000000102" || got.scAddr != "12125550002" {
		t.Fatalf("DeleteMWD args = %+v, want imsi=%q scAddr=%q", got, "001010000000102", "12125550002")
	}
	if _, ok := h.pendingALSC.Load("sid-success"); ok {
		t.Fatal("pending ALSC session still present after successful ASA")
	}
}

func TestASA_FailureRetainsMWD(t *testing.T) {
	store := newS6cStore()
	h := newTestHandlers(store)
	h.pendingALSC.Store("sid-failure", pendingALSCEntry{imsi: "001010000000103", scAddr: "12125550003"})

	h.ASA(nil, buildASA(t, "sid-failure", 5005))

	if len(store.deletedMWD) != 0 {
		t.Fatalf("DeleteMWD called %d times, want 0", len(store.deletedMWD))
	}
	if _, ok := h.pendingALSC.Load("sid-failure"); ok {
		t.Fatal("pending ALSC session still present after failure ASA")
	}
}

func TestASA_UnknownSessionDoesNotDeleteMWD(t *testing.T) {
	store := newS6cStore()
	h := newTestHandlers(store)

	h.ASA(nil, buildASA(t, "sid-unknown", diamResultSuccess))

	if len(store.deletedMWD) != 0 {
		t.Fatalf("DeleteMWD called %d times, want 0", len(store.deletedMWD))
	}
}

func TestASA_MissingSessionIDDoesNotDeleteMWD(t *testing.T) {
	store := newS6cStore()
	h := newTestHandlers(store)
	h.pendingALSC.Store("sid-present", pendingALSCEntry{imsi: "001010000000104", scAddr: "12125550004"})

	msg := diam.NewMessage(cmdALSC, 0, AppIDS6c, 0, 0, dict.Default)
	msg.NewAVP(avp.ResultCode, avp.Mbit, 0, datatype.Unsigned32(diamResultSuccess))

	h.ASA(nil, msg)

	if len(store.deletedMWD) != 0 {
		t.Fatalf("DeleteMWD called %d times, want 0", len(store.deletedMWD))
	}
	if _, ok := h.pendingALSC.Load("sid-present"); !ok {
		t.Fatal("pending ALSC session removed for malformed ASA without Session-Id")
	}
}

func TestSendALSCForIMSI_NoMWD_NoSend(t *testing.T) {
	store := newS6cStore()
	imsi := "001010000000106"
	store.addSubscriber(&models.Subscriber{IMSI: imsi})

	peers := &peerLookupStub{conns: map[string]diam.Conn{}}
	h := newTestHandlers(store)
	h.peers = peers

	h.SendALSCForIMSI(imsi)

	if len(peers.lookups) != 0 {
		t.Fatalf("peer lookup calls = %d, want 0", len(peers.lookups))
	}
}

func TestSendALSCForIMSI_PeerNotConnected_MWDRetained(t *testing.T) {
	store := newS6cStore()
	imsi := "001010000000107"
	store.addSubscriber(&models.Subscriber{IMSI: imsi})
	store.mwds[imsi] = []models.MessageWaitingData{{
		IMSI:          imsi,
		SCAddress:     "12125550007",
		SCOriginHost:  "smsc2.test.net",
		SCOriginRealm: "smsc.realm.net",
	}}

	peers := &peerLookupStub{conns: map[string]diam.Conn{}}
	h := newTestHandlers(store)
	h.peers = peers

	h.SendALSCForIMSI(imsi)

	if len(peers.lookups) != 1 {
		t.Fatalf("peer lookup calls = %d, want 1", len(peers.lookups))
	}
	if len(store.deletedMWD) != 0 {
		t.Fatalf("DeleteMWD called %d times, want 0", len(store.deletedMWD))
	}
	if len(store.mwds[imsi]) != 1 {
		t.Fatalf("MWD records remaining = %d, want 1", len(store.mwds[imsi]))
	}
}

func TestSendALSCForIMSI_MultipleMWDRecords(t *testing.T) {
	store := newS6cStore()
	imsi := "001010000000108"
	store.addSubscriber(&models.Subscriber{IMSI: imsi})
	store.mwds[imsi] = []models.MessageWaitingData{
		{IMSI: imsi, SCAddress: "12125550008", SCOriginHost: "smsc-a.test.net", SCOriginRealm: "realm-a.net"},
		{IMSI: imsi, SCAddress: "12125550009", SCOriginHost: "smsc-b.test.net", SCOriginRealm: "realm-b.net"},
	}

	connA := &captureConn{}
	connB := &captureConn{}
	peers := &peerLookupStub{conns: map[string]diam.Conn{
		"smsc-a.test.net": connA,
		"smsc-b.test.net": connB,
	}}
	h := newTestHandlers(store)
	h.peers = peers

	h.SendALSCForIMSI(imsi)

	msgA := parseWrittenMessage(t, connA)
	msgB := parseWrittenMessage(t, connB)
	if got := decodeMSISDN(findAVPDirect(msgA, avpSCAddress, Vendor3GPP).Data.(datatype.OctetString)); got != "12125550008" {
		t.Fatalf("connA SC-Address: got %q, want %q", got, "12125550008")
	}
	if got := decodeMSISDN(findAVPDirect(msgB, avpSCAddress, Vendor3GPP).Data.(datatype.OctetString)); got != "12125550009" {
		t.Fatalf("connB SC-Address: got %q, want %q", got, "12125550009")
	}
	if got := string(findAVPDirect(msgA, avp.DestinationRealm, 0).Data.(datatype.DiameterIdentity)); got != "realm-a.net" {
		t.Fatalf("connA Destination-Realm: got %q, want %q", got, "realm-a.net")
	}
	if got := string(findAVPDirect(msgB, avp.DestinationRealm, 0).Data.(datatype.DiameterIdentity)); got != "realm-b.net" {
		t.Fatalf("connB Destination-Realm: got %q, want %q", got, "realm-b.net")
	}
	sidA := string(findAVPDirect(msgA, avp.SessionID, 0).Data.(datatype.UTF8String))
	sidB := string(findAVPDirect(msgB, avp.SessionID, 0).Data.(datatype.UTF8String))
	if sidA == sidB {
		t.Fatal("multiple ALSC requests reused the same Session-Id")
	}
}

func TestSendALSCForIMSI_SubscriberMissing(t *testing.T) {
	store := newS6cStore()
	peers := &peerLookupStub{conns: map[string]diam.Conn{}}
	h := newTestHandlers(store)
	h.peers = peers

	h.SendALSCForIMSI("001010000000109")

	if len(peers.lookups) != 0 {
		t.Fatalf("peer lookup calls = %d, want 0", len(peers.lookups))
	}
	if len(store.deletedMWD) != 0 {
		t.Fatalf("DeleteMWD called %d times, want 0", len(store.deletedMWD))
	}
}

func TestSendALSCForIMSI_SendFailureRetainsMWD(t *testing.T) {
	store := newS6cStore()
	imsi := "001010000000110"
	store.addSubscriber(&models.Subscriber{IMSI: imsi})
	store.mwds[imsi] = []models.MessageWaitingData{{
		IMSI:          imsi,
		SCAddress:     "12125550010",
		SCOriginHost:  "smsc3.test.net",
		SCOriginRealm: "smsc.realm.net",
	}}

	conn := &captureConn{writeErr: errors.New("write failed")}
	peers := &peerLookupStub{conns: map[string]diam.Conn{"smsc3.test.net": conn}}
	h := newTestHandlers(store)
	h.peers = peers

	h.SendALSCForIMSI(imsi)

	if len(store.deletedMWD) != 0 {
		t.Fatalf("DeleteMWD called %d times, want 0", len(store.deletedMWD))
	}
	var pendingCount int
	h.pendingALSC.Range(func(_, _ any) bool {
		pendingCount++
		return true
	})
	if pendingCount != 0 {
		t.Fatalf("pending ALSC entries = %d, want 0 after send failure", pendingCount)
	}
}
