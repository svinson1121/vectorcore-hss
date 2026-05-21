package swx

import (
	"bytes"
	"context"
	"crypto/tls"
	"net"
	"testing"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

type capturePeerLookup struct {
	conn *captureConn
}

func (p *capturePeerLookup) GetConn(originHost string) (diam.Conn, bool) {
	if p.conn == nil || originHost != "aaa01.epc.mnc435.mcc311.3gppnetwork.org" {
		return nil, false
	}
	return p.conn, true
}

type captureConn struct {
	bytes.Buffer
}

func (c *captureConn) WriteStream(b []byte, _ uint) (int, error) { return c.Write(b) }
func (c *captureConn) Close()                                    {}
func (c *captureConn) LocalAddr() net.Addr                       { return testAddr("local") }
func (c *captureConn) RemoteAddr() net.Addr                      { return testAddr("remote") }
func (c *captureConn) TLS() *tls.ConnectionState                 { return nil }
func (c *captureConn) Dictionary() *dict.Parser                  { return dict.Default }
func (c *captureConn) Context() context.Context                  { return context.Background() }
func (c *captureConn) SetContext(context.Context)                {}
func (c *captureConn) Connection() net.Conn                      { return nil }

type testAddr string

func (a testAddr) Network() string { return string(a) }
func (a testAddr) String() string  { return string(a) }

func TestSendRTREncodesSWxApplicationAndRequiredAVPs(t *testing.T) {
	loadTestDict(t)

	conn := &captureConn{}
	h := newSendTestHandler(&capturePeerLookup{conn: conn}, &swxSARStore{})
	h.SendRTR("311435000070571", "aaa01.epc.mnc435.mcc311.3gppnetwork.org", "epc.mnc435.mcc311.3gppnetwork.org", RTRReasonPermanentTermination, "subscription withdrawn")

	msg := readCapturedMessage(t, conn)
	requireSWxHeader(t, msg, cmdRTR)
	requireDirectAVP(t, msg, avp.SessionID, 0)
	requireDirectAVP(t, msg, avp.AuthSessionState, 0)
	requireDirectAVP(t, msg, avp.OriginHost, 0)
	requireDirectAVP(t, msg, avp.OriginRealm, 0)
	requireDirectAVP(t, msg, avp.DestinationHost, 0)
	requireDirectAVP(t, msg, avp.DestinationRealm, 0)
	requireDirectAVP(t, msg, avp.UserName, 0)
	requireSWxVSAI(t, msg)

	reason := requireDirectAVP(t, msg, avpDeregistrationReason, Vendor3GPP)
	group, ok := reason.Data.(*diam.GroupedAVP)
	if !ok {
		t.Fatalf("Deregistration-Reason type: %T", reason.Data)
	}
	requireEnumeratedChild(t, group, avpReasonCode, Vendor3GPP, RTRReasonPermanentTermination)
	requireUTF8Child(t, group, avpReasonInfo, Vendor3GPP, "subscription withdrawn")
}

func TestSendPPREncodesSWxApplicationAndRequiredAVPs(t *testing.T) {
	loadTestDict(t)

	conn := &captureConn{}
	h := newSendTestHandler(&capturePeerLookup{conn: conn}, &swxSARStore{})
	h.SendPPR("311435000070571", "aaa01.epc.mnc435.mcc311.3gppnetwork.org", "epc.mnc435.mcc311.3gppnetwork.org", true)

	msg := readCapturedMessage(t, conn)
	requireSWxHeader(t, msg, cmdPPR)
	requireDirectAVP(t, msg, avp.SessionID, 0)
	requireDirectAVP(t, msg, avp.AuthSessionState, 0)
	requireDirectAVP(t, msg, avp.OriginHost, 0)
	requireDirectAVP(t, msg, avp.OriginRealm, 0)
	requireDirectAVP(t, msg, avp.DestinationHost, 0)
	requireDirectAVP(t, msg, avp.DestinationRealm, 0)
	requireDirectAVP(t, msg, avp.UserName, 0)
	requireSWxVSAI(t, msg)

	userData := requireDirectAVP(t, msg, avpNon3GPPUserData, Vendor3GPP)
	group, ok := userData.Data.(*diam.GroupedAVP)
	if !ok {
		t.Fatalf("Non-3GPP-User-Data type: %T", userData.Data)
	}
	requireEnumeratedChild(t, group, avpNon3GPPIPAccess, Vendor3GPP, Non3GPPAccessAllowed)
}

func TestDecodeSWxRTAAndPPAAnswers(t *testing.T) {
	loadTestDict(t)

	for _, tc := range []struct {
		name string
		code uint32
	}{
		{name: "RTA", code: cmdRTR},
		{name: "PPA", code: cmdPPR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded := encodeSWxAnswer(t, tc.code)
			msg, err := diam.ReadMessage(bytes.NewReader(encoded), dict.Default)
			if err != nil {
				t.Fatalf("read %s: %v", tc.name, err)
			}
			if msg.Header.ApplicationID != AppIDSWx {
				t.Fatalf("Application-Id: got %d", msg.Header.ApplicationID)
			}
			if msg.Header.CommandCode != tc.code {
				t.Fatalf("Command-Code: got %d", msg.Header.CommandCode)
			}
			if msg.Header.CommandFlags&diam.RequestFlag != 0 {
				t.Fatalf("%s decoded with request bit set", tc.name)
			}
			cmd, err := msg.Dictionary().FindCommand(msg.Header.ApplicationID, msg.Header.CommandCode)
			if err != nil {
				t.Fatalf("find command: %v", err)
			}
			if cmd.Code != tc.code {
				t.Fatalf("dictionary command code: got %d", cmd.Code)
			}
		})
	}
}

func TestSWxAnswerCompletesPendingTransaction(t *testing.T) {
	loadTestDict(t)

	h := newSendTestHandler(&capturePeerLookup{}, &swxSARStore{})
	req := diam.NewRequest(cmdRTR, AppIDSWx, dict.Default)
	req.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String("swx-rtr;1"))
	h.trackSWxTransaction(req, "swx-rtr;1", "311435000070571")

	ans := diam.NewMessage(cmdRTR, 0, AppIDSWx, req.Header.HopByHopID, req.Header.EndToEndID, dict.Default)
	ans.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String("swx-rtr;1"))
	ans.AddAVP(newSWxVendorSpecificApplicationID())
	ans.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	ans.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity("aaa.example.net"))
	ans.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity("example.net"))
	ans.NewAVP(avp.ResultCode, avp.Mbit, 0, datatype.Unsigned32(diam.Success))

	h.RTA(nil, ans)

	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()
	if len(h.pending) != 0 {
		t.Fatalf("pending transactions: got %d want 0", len(h.pending))
	}
}

func newSendTestHandler(peers PeerLookup, store repository.Repository) *Handlers {
	return NewHandlers(&config.Config{
		HSS: config.HSSConfig{OriginHost: "hss.example.net", OriginRealm: "example.net"},
	}, store, zap.NewNop(), peers)
}

func readCapturedMessage(t *testing.T, conn *captureConn) *diam.Message {
	t.Helper()
	msg, err := diam.ReadMessage(bytes.NewReader(conn.Bytes()), dict.Default)
	if err != nil {
		t.Fatalf("read captured message: %v", err)
	}
	return msg
}

func requireSWxHeader(t *testing.T, msg *diam.Message, commandCode uint32) {
	t.Helper()
	if msg.Header.ApplicationID != AppIDSWx {
		t.Fatalf("Application-Id: got %d want %d", msg.Header.ApplicationID, AppIDSWx)
	}
	if msg.Header.CommandCode != commandCode {
		t.Fatalf("Command-Code: got %d want %d", msg.Header.CommandCode, commandCode)
	}
	if msg.Header.CommandFlags&diam.RequestFlag == 0 {
		t.Fatal("request bit is not set")
	}
	if msg.Header.CommandFlags&diam.ProxiableFlag == 0 {
		t.Fatal("proxiable bit is not set")
	}
}

func requireDirectAVP(t *testing.T, msg *diam.Message, code uint32, vendor uint32) *diam.AVP {
	t.Helper()
	a := findAVPDirect(msg, code, vendor)
	if a == nil {
		t.Fatalf("missing AVP code=%d vendor=%d", code, vendor)
	}
	return a
}

func requireSWxVSAI(t *testing.T, msg *diam.Message) {
	t.Helper()
	vsai := requireDirectAVP(t, msg, avp.VendorSpecificApplicationID, 0)
	group, ok := vsai.Data.(*diam.GroupedAVP)
	if !ok {
		t.Fatalf("Vendor-Specific-Application-Id type: %T", vsai.Data)
	}
	requireUnsigned32Child(t, group, avp.VendorID, 0, Vendor3GPP)
	requireUnsigned32Child(t, group, avp.AuthApplicationID, 0, AppIDSWx)
}

func encodeSWxAnswer(t *testing.T, commandCode uint32) []byte {
	t.Helper()
	msg := diam.NewMessage(commandCode, 0, AppIDSWx, 0x01020304, 0x05060708, dict.Default)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String("swx-answer;1"))
	msg.AddAVP(newSWxVendorSpecificApplicationID())
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity("aaa.example.net"))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity("example.net"))
	msg.NewAVP(avp.ResultCode, avp.Mbit, 0, datatype.Unsigned32(diam.Success))

	var buf bytes.Buffer
	if _, err := msg.WriteTo(&buf); err != nil {
		t.Fatalf("encode SWx answer: %v", err)
	}
	return buf.Bytes()
}
