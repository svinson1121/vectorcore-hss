package s6c

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"net"
	"testing"
	"time"

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

func buildALA(t *testing.T, sessionID string, resultCode uint32) *diam.Message {
	t.Helper()
	msg := diam.NewRequest(cmdALR, AppIDS6c, dict.Default).Answer(resultCode)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sessionID))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity("smsc.test.net"))
	return msg
}

func buildALAWithExperimentalResult(t *testing.T, sessionID string, resultCode uint32) *diam.Message {
	t.Helper()
	msg := diam.NewMessage(cmdALR, 0, AppIDS6c, 0, 0, dict.Default)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sessionID))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity("smsc.test.net"))
	msg.NewAVP(avp.ExperimentalResult, avp.Mbit, 0, &diam.GroupedAVP{AVP: []*diam.AVP{
		diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(Vendor3GPP)),
		diam.NewAVP(avp.ExperimentalResultCode, avp.Mbit, 0, datatype.Unsigned32(resultCode)),
	}})
	return msg
}

func findGroupedChildAVP(group *diam.GroupedAVP, code uint32, vendorID uint32) *diam.AVP {
	if group == nil {
		return nil
	}
	for _, child := range group.AVP {
		if child.Code == code && child.VendorID == vendorID {
			return child
		}
	}
	return nil
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
	if msg.Header.CommandCode != cmdALR {
		t.Fatalf("Command-Code: got %d, want %d", msg.Header.CommandCode, cmdALR)
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
		t.Fatal("missing Session-Id in ALR request")
	}
	sid := string(sidAVP.Data.(datatype.UTF8String))

	entry, ok := h.pendingALR.Load(sid)
	if !ok {
		t.Fatal("pending ALR session not recorded")
	}
	pending := entry.(pendingALREntry)
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
	h.pendingALR.Store("sid-success", pendingALREntry{imsi: "001010000000102", scAddr: "12125550002"})

	h.ALA(nil, buildALA(t, "sid-success", diamResultSuccess))

	if len(store.deletedMWD) != 1 {
		t.Fatalf("expected 1 DeleteMWD call, got %d", len(store.deletedMWD))
	}
	if got := store.deletedMWD[0]; got.imsi != "001010000000102" || got.scAddr != "12125550002" {
		t.Fatalf("DeleteMWD args = %+v, want imsi=%q scAddr=%q", got, "001010000000102", "12125550002")
	}
	if _, ok := h.pendingALR.Load("sid-success"); ok {
		t.Fatal("pending ALR session still present after successful ALA")
	}
}

func TestASA_FailureRetainsMWD(t *testing.T) {
	store := newS6cStore()
	h := newTestHandlers(store)
	h.pendingALR.Store("sid-failure", pendingALREntry{imsi: "001010000000103", scAddr: "12125550003"})

	h.ALA(nil, buildALA(t, "sid-failure", 5005))

	if len(store.deletedMWD) != 0 {
		t.Fatalf("DeleteMWD called %d times, want 0", len(store.deletedMWD))
	}
	if _, ok := h.pendingALR.Load("sid-failure"); ok {
		t.Fatal("pending ALR session still present after failure ALA")
	}
}

func TestASA_ExperimentalResultFailureRetainsMWD(t *testing.T) {
	store := newS6cStore()
	h := newTestHandlers(store)
	h.pendingALR.Store("sid-exp-failure", pendingALREntry{imsi: "001010000000115", scAddr: "12125550015"})

	h.ALA(nil, buildALAWithExperimentalResult(t, "sid-exp-failure", 5001))

	if len(store.deletedMWD) != 0 {
		t.Fatalf("DeleteMWD called %d times, want 0", len(store.deletedMWD))
	}
	if _, ok := h.pendingALR.Load("sid-exp-failure"); ok {
		t.Fatal("pending ALR session still present after Experimental-Result failure ALA")
	}
}

func TestASA_UnknownSessionDoesNotDeleteMWD(t *testing.T) {
	store := newS6cStore()
	h := newTestHandlers(store)

	h.ALA(nil, buildALA(t, "sid-unknown", diamResultSuccess))

	if len(store.deletedMWD) != 0 {
		t.Fatalf("DeleteMWD called %d times, want 0", len(store.deletedMWD))
	}
}

func TestASA_MissingSessionIDDoesNotDeleteMWD(t *testing.T) {
	store := newS6cStore()
	h := newTestHandlers(store)
	h.pendingALR.Store("sid-present", pendingALREntry{imsi: "001010000000104", scAddr: "12125550004"})

	msg := diam.NewMessage(cmdALR, 0, AppIDS6c, 0, 0, dict.Default)
	msg.NewAVP(avp.ResultCode, avp.Mbit, 0, datatype.Unsigned32(diamResultSuccess))

	h.ALA(nil, msg)

	if len(store.deletedMWD) != 0 {
		t.Fatalf("DeleteMWD called %d times, want 0", len(store.deletedMWD))
	}
	if _, ok := h.pendingALR.Load("sid-present"); !ok {
		t.Fatal("pending ALR session removed for malformed ALA without Session-Id")
	}
}

func TestASA_MissingResultKeepsMWDRetryState(t *testing.T) {
	store := newS6cStore()
	h := newTestHandlers(store)
	h.pendingALR.Store("sid-no-result", pendingALREntry{imsi: "001010000000116", scAddr: "12125550016"})

	msg := diam.NewMessage(cmdALR, 0, AppIDS6c, 0, 0, dict.Default)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String("sid-no-result"))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity("smsc.test.net"))

	h.ALA(nil, msg)

	if len(store.deletedMWD) != 0 {
		t.Fatalf("DeleteMWD called %d times, want 0", len(store.deletedMWD))
	}
	if _, ok := h.pendingALR.Load("sid-no-result"); ok {
		t.Fatal("pending ALR session still present after ALA with missing result")
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
		t.Fatal("multiple ALR requests reused the same Session-Id")
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
	h.pendingALR.Range(func(_, _ any) bool {
		pendingCount++
		return true
	})
	if pendingCount != 0 {
		t.Fatalf("pending ALR entries = %d, want 0 after send failure", pendingCount)
	}
}

func TestSendALRForIMSI_RecordsAlertAttemptMetadata(t *testing.T) {
	store := newS6cStore()
	imsi := "001010000000113"
	store.addSubscriber(&models.Subscriber{IMSI: imsi})
	store.mwds[imsi] = []models.MessageWaitingData{{
		IMSI:           imsi,
		SCAddress:      "12125550013",
		SCOriginHost:   "smsc6.test.net",
		SCOriginRealm:  "smsc.realm.net",
		MWDStatusFlags: MWDStatusMNRF,
	}}

	conn := &captureConn{}
	peers := &peerLookupStub{conns: map[string]diam.Conn{"smsc6.test.net": conn}}
	h := newTestHandlers(store)
	h.peers = peers

	h.SendALRForIMSI(imsi, AlertTriggerUserAvailable)

	if len(store.mwds[imsi]) != 1 {
		t.Fatalf("MWD record count = %d, want 1", len(store.mwds[imsi]))
	}
	rec := store.mwds[imsi][0]
	if rec.LastAlertTrigger == nil || *rec.LastAlertTrigger != string(AlertTriggerUserAvailable) {
		t.Fatalf("LastAlertTrigger = %+v, want %q", rec.LastAlertTrigger, AlertTriggerUserAvailable)
	}
	if rec.LastAlertAttemptAt == nil {
		t.Fatal("LastAlertAttemptAt not recorded")
	}
	if rec.AlertAttemptCount != 1 {
		t.Fatalf("AlertAttemptCount = %d, want 1", rec.AlertAttemptCount)
	}
}

func TestSendALRForIMSI_IncludesAbsentUserDiagnosticSM(t *testing.T) {
	store := newS6cStore()
	imsi := "001010000000117"
	diag := uint32(77)
	store.addSubscriber(&models.Subscriber{IMSI: imsi})
	store.mwds[imsi] = []models.MessageWaitingData{{
		IMSI:                   imsi,
		SCAddress:              "12125550015",
		SCOriginHost:           "smsc8.test.net",
		SCOriginRealm:          "smsc.realm.net",
		MWDStatusFlags:         MWDStatusMNRF,
		AbsentUserDiagnosticSM: &diag,
	}}

	conn := &captureConn{}
	peers := &peerLookupStub{conns: map[string]diam.Conn{"smsc8.test.net": conn}}
	h := newTestHandlers(store)
	h.peers = peers

	h.SendALRForIMSI(imsi, AlertTriggerUserAvailable)

	msg := parseWrittenMessage(t, conn)
	diagAVP := findAVPDirect(msg, avpAbsentUserDiagnosticSM, Vendor3GPP)
	if diagAVP == nil {
		t.Fatal("missing Absent-User-Diagnostic-SM AVP in ALR")
	}
	if got := uint32(diagAVP.Data.(datatype.Unsigned32)); got != diag {
		t.Fatalf("Absent-User-Diagnostic-SM: got %d, want %d", got, diag)
	}
}

func TestSendALRForIMSI_IncludesUserIdentifier(t *testing.T) {
	store := newS6cStore()
	imsi := "001010000000118"
	msisdn := "12125550016"
	store.addSubscriber(&models.Subscriber{IMSI: imsi, MSISDN: ptr(msisdn)})
	store.mwds[imsi] = []models.MessageWaitingData{{
		IMSI:           imsi,
		SCAddress:      "12125550017",
		SCOriginHost:   "smsc9.test.net",
		SCOriginRealm:  "smsc.realm.net",
		MWDStatusFlags: MWDStatusMNRF,
	}}

	conn := &captureConn{}
	peers := &peerLookupStub{conns: map[string]diam.Conn{"smsc9.test.net": conn}}
	h := newTestHandlers(store)
	h.peers = peers

	h.SendALRForIMSI(imsi, AlertTriggerUserAvailable)

	msg := parseWrittenMessage(t, conn)
	userIDAVP := findAVPDirect(msg, avpUserIdentifier, Vendor3GPP)
	if userIDAVP == nil {
		t.Fatal("missing User-Identifier AVP in ALR")
	}
	group, ok := userIDAVP.Data.(*diam.GroupedAVP)
	if !ok {
		t.Fatalf("User-Identifier type = %T, want *diam.GroupedAVP", userIDAVP.Data)
	}
	userNameAVP := findGroupedChildAVP(group, avp.UserName, 0)
	if userNameAVP == nil {
		t.Fatal("User-Identifier missing User-Name child")
	}
	if got := string(userNameAVP.Data.(datatype.UTF8String)); got != imsi {
		t.Fatalf("User-Identifier User-Name: got %q, want %q", got, imsi)
	}
	msisdnAVP := findGroupedChildAVP(group, avpMSISDN, Vendor3GPP)
	if msisdnAVP == nil {
		t.Fatal("User-Identifier missing MSISDN child")
	}
	if got := decodeMSISDN(msisdnAVP.Data.(datatype.OctetString)); got != msisdn {
		t.Fatalf("User-Identifier MSISDN: got %q, want %q", got, msisdn)
	}
}

func TestSendALRForIMSI_IncludesServingNodeAndAlertEvent(t *testing.T) {
	store := newS6cStore()
	imsi := "001010000000119"
	mme := "mme1.epc.test.net"
	mmeRealm := "epc.test.net"
	mmeNumber := "12125550018"
	smsRegistered := true
	store.addSubscriber(&models.Subscriber{
		IMSI:                imsi,
		ServingMME:          ptr(mme),
		ServingMMERealm:     ptr(mmeRealm),
		MMENumberForMTSMS:   ptr(mmeNumber),
		MMERegisteredForSMS: ptr(smsRegistered),
	})
	store.mwds[imsi] = []models.MessageWaitingData{{
		IMSI:           imsi,
		SCAddress:      "12125550019",
		SCOriginHost:   "smsc10.test.net",
		SCOriginRealm:  "smsc.realm.net",
		MWDStatusFlags: MWDStatusMNRF,
	}}

	conn := &captureConn{}
	peers := &peerLookupStub{conns: map[string]diam.Conn{"smsc10.test.net": conn}}
	h := newTestHandlers(store)
	h.peers = peers

	h.SendALRForIMSI(imsi, AlertTriggerUserAvailable)

	msg := parseWrittenMessage(t, conn)
	nodeAVP := findAVPDirect(msg, avpServingNode, Vendor3GPP)
	if nodeAVP == nil {
		t.Fatal("missing Serving-Node AVP in ALR")
	}
	group, ok := nodeAVP.Data.(*diam.GroupedAVP)
	if !ok {
		t.Fatalf("Serving-Node type = %T, want *diam.GroupedAVP", nodeAVP.Data)
	}
	mmeNameAVP := findGroupedChildAVP(group, avpMMEName, Vendor3GPP)
	if mmeNameAVP == nil {
		t.Fatal("Serving-Node missing MME-Name")
	}
	if got := string(mmeNameAVP.Data.(datatype.DiameterIdentity)); got != mme {
		t.Fatalf("Serving-Node MME-Name: got %q, want %q", got, mme)
	}
	alertEventAVP := findAVPDirect(msg, avpSMSGMSCAlertEvent, Vendor3GPP)
	if alertEventAVP == nil {
		t.Fatal("missing SMS-GMSC-Alert-Event AVP in ALR")
	}
	if got := uint32(alertEventAVP.Data.(datatype.Unsigned32)); got != SMSGMSCAlertEventUEAvailableForMTSMS {
		t.Fatalf("SMS-GMSC-Alert-Event: got 0x%x, want 0x%x", got, SMSGMSCAlertEventUEAvailableForMTSMS)
	}
}

func TestSendALRForIMSI_IncludesSMSMICorrelationID(t *testing.T) {
	store := newS6cStore()
	imsi := "001010000000120"
	rawPayload := []byte{
		0x00, 0x00, 0x0c, 0xfd, 0x80, 0x00, 0x00, 0x14, 0x00, 0x00, 0x28, 0xaf,
		'h', 's', 's', '1',
	}
	encoded := base64.StdEncoding.EncodeToString(rawPayload)
	store.addSubscriber(&models.Subscriber{IMSI: imsi})
	store.mwds[imsi] = []models.MessageWaitingData{{
		IMSI:               imsi,
		SCAddress:          "12125550020",
		SCOriginHost:       "smsc11.test.net",
		SCOriginRealm:      "smsc.realm.net",
		MWDStatusFlags:     MWDStatusMNRF,
		SMSMICorrelationID: &encoded,
	}}

	conn := &captureConn{}
	peers := &peerLookupStub{conns: map[string]diam.Conn{"smsc11.test.net": conn}}
	h := newTestHandlers(store)
	h.peers = peers

	h.SendALRForIMSI(imsi, AlertTriggerUserAvailable)

	msg := parseWrittenMessage(t, conn)
	smsmiAVP := findAVPDirect(msg, avpSMSMICorrelationID, Vendor3GPP)
	if smsmiAVP == nil {
		t.Fatal("missing SMSMI-Correlation-ID AVP in ALR")
	}
	if got := []byte(smsmiAVP.Data.(datatype.OctetString)); !bytes.Equal(got, rawPayload) {
		t.Fatalf("SMSMI-Correlation-ID payload = %x, want %x", got, rawPayload)
	}
}

func TestSendALRForIMSI_IncludesMaximumUEAvailabilityTime(t *testing.T) {
	store := newS6cStore()
	imsi := "001010000000121"
	store.addSubscriber(&models.Subscriber{IMSI: imsi})
	store.mwds[imsi] = []models.MessageWaitingData{{
		IMSI:           imsi,
		SCAddress:      "12125550021",
		SCOriginHost:   "smsc12.test.net",
		SCOriginRealm:  "smsc.realm.net",
		MWDStatusFlags: MWDStatusMCEF,
	}}

	conn := &captureConn{}
	peers := &peerLookupStub{conns: map[string]diam.Conn{"smsc12.test.net": conn}}
	h := newTestHandlers(store)
	h.peers = peers
	maxAvail := time.Unix(1735689600, 0).UTC()

	h.SendALRForIMSIWithMaximumAvailability(imsi, AlertTriggerMemoryAvailable, &maxAvail)

	msg := parseWrittenMessage(t, conn)
	a := findAVPDirect(msg, avpMaximumUEAvailabilityTime, Vendor3GPP)
	if a == nil {
		t.Fatal("missing Maximum-UE-Availability-Time AVP in ALR")
	}
	got := time.Time(a.Data.(datatype.Time)).UTC()
	if !got.Equal(maxAvail) {
		t.Fatalf("Maximum-UE-Availability-Time: got %s, want %s", got, maxAvail)
	}
}

func TestSendALRForIMSI_SendFailureDoesNotRecordAlertAttemptMetadata(t *testing.T) {
	store := newS6cStore()
	imsi := "001010000000114"
	store.addSubscriber(&models.Subscriber{IMSI: imsi})
	store.mwds[imsi] = []models.MessageWaitingData{{
		IMSI:           imsi,
		SCAddress:      "12125550014",
		SCOriginHost:   "smsc7.test.net",
		SCOriginRealm:  "smsc.realm.net",
		MWDStatusFlags: MWDStatusMNRF,
	}}

	conn := &captureConn{writeErr: errors.New("write failed")}
	peers := &peerLookupStub{conns: map[string]diam.Conn{"smsc7.test.net": conn}}
	h := newTestHandlers(store)
	h.peers = peers

	h.SendALRForIMSI(imsi, AlertTriggerUserAvailable)

	rec := store.mwds[imsi][0]
	if rec.LastAlertTrigger != nil {
		t.Fatalf("LastAlertTrigger = %+v, want nil on send failure", rec.LastAlertTrigger)
	}
	if rec.LastAlertAttemptAt != nil {
		t.Fatal("LastAlertAttemptAt recorded on send failure")
	}
	if rec.AlertAttemptCount != 0 {
		t.Fatalf("AlertAttemptCount = %d, want 0 on send failure", rec.AlertAttemptCount)
	}
}

func TestSendAlertForIMSI_UserAvailableSkipsMemoryCapacityMWD(t *testing.T) {
	store := newS6cStore()
	imsi := "001010000000111"
	store.addSubscriber(&models.Subscriber{IMSI: imsi})
	store.mwds[imsi] = []models.MessageWaitingData{{
		IMSI:           imsi,
		SCAddress:      "12125550011",
		SCOriginHost:   "smsc4.test.net",
		SCOriginRealm:  "smsc.realm.net",
		MWDStatusFlags: MWDStatusMCEF,
	}}

	conn := &captureConn{}
	peers := &peerLookupStub{conns: map[string]diam.Conn{"smsc4.test.net": conn}}
	h := newTestHandlers(store)
	h.peers = peers

	h.SendALRForIMSI(imsi, AlertTriggerUserAvailable)

	if len(peers.lookups) != 0 {
		t.Fatalf("peer lookup calls = %d, want 0", len(peers.lookups))
	}
	if conn.buf.Len() != 0 {
		t.Fatal("unexpected alert sent for memory-capacity MWD on user-available trigger")
	}
}

func TestSendAlertForIMSI_MemoryAvailableSendsForMCEF(t *testing.T) {
	store := newS6cStore()
	imsi := "001010000000112"
	store.addSubscriber(&models.Subscriber{IMSI: imsi})
	store.mwds[imsi] = []models.MessageWaitingData{{
		IMSI:           imsi,
		SCAddress:      "12125550012",
		SCOriginHost:   "smsc5.test.net",
		SCOriginRealm:  "smsc.realm.net",
		MWDStatusFlags: MWDStatusMCEF,
	}}

	conn := &captureConn{}
	peers := &peerLookupStub{conns: map[string]diam.Conn{"smsc5.test.net": conn}}
	h := newTestHandlers(store)
	h.peers = peers

	h.SendALRForIMSI(imsi, AlertTriggerMemoryAvailable)

	if len(peers.lookups) != 1 {
		t.Fatalf("peer lookup calls = %d, want 1", len(peers.lookups))
	}
	if conn.buf.Len() == 0 {
		t.Fatal("expected alert to be sent for memory-available trigger")
	}
}
