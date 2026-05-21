package swx

import (
	"testing"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"

	"github.com/svinson1121/vectorcore-hss/internal/diameter/basedict"
	"github.com/svinson1121/vectorcore-hss/internal/diameter/cx"
)

func loadTestDict(t *testing.T) {
	t.Helper()

	if err := basedict.Load(); err != nil {
		t.Fatalf("load base dict: %v", err)
	}
	if err := cx.LoadDict(); err != nil {
		t.Fatalf("load cx dict: %v", err)
	}
	if err := LoadDict(); err != nil {
		t.Fatalf("load swx dict: %v", err)
	}
}

func TestLoadDictRegistersServerAssignmentForSWx(t *testing.T) {
	loadTestDict(t)

	cmd, err := dict.Default.FindCommand(AppIDSWx, 301)
	if err != nil {
		t.Fatalf("find SWx SAR command: %v", err)
	}
	if cmd.Name != "Server-Assignment" {
		t.Fatalf("command name: got %q", cmd.Name)
	}
}

func TestLoadDictRegistersMultimediaAuthenticationForSWx(t *testing.T) {
	loadTestDict(t)

	cmd, err := dict.Default.FindCommand(AppIDSWx, 303)
	if err != nil {
		t.Fatalf("find SWx MAR command: %v", err)
	}
	if cmd.Name != "Multimedia-Authentication" {
		t.Fatalf("command name: got %q", cmd.Name)
	}
}

func TestLoadDictRegistersRegistrationTerminationForSWx(t *testing.T) {
	loadTestDict(t)

	cmd, err := dict.Default.FindCommand(AppIDSWx, cmdRTR)
	if err != nil {
		t.Fatalf("find SWx RTR command: %v", err)
	}
	if cmd.Name != "Registration-Termination" || cmd.Short != "RT" {
		t.Fatalf("command: got name=%q short=%q", cmd.Name, cmd.Short)
	}
	requireCommandRule(t, cmd.Request.Rule, "Vendor-Specific-Application-Id", true)
	requireCommandRule(t, cmd.Request.Rule, "Deregistration-Reason", true)
	requireCommandRule(t, cmd.Answer.Rule, "Vendor-Specific-Application-Id", true)
	requireCommandRule(t, cmd.Answer.Rule, "Result-Code", false)
	requireCommandRule(t, cmd.Answer.Rule, "Experimental-Result", false)
}

func TestLoadDictRegistersPushProfileForSWx(t *testing.T) {
	loadTestDict(t)

	cmd, err := dict.Default.FindCommand(AppIDSWx, cmdPPR)
	if err != nil {
		t.Fatalf("find SWx PPR command: %v", err)
	}
	if cmd.Name != "Push-Profile" || cmd.Short != "PP" {
		t.Fatalf("command: got name=%q short=%q", cmd.Name, cmd.Short)
	}
	requireCommandRule(t, cmd.Request.Rule, "Vendor-Specific-Application-Id", true)
	requireCommandRule(t, cmd.Request.Rule, "Non-3GPP-User-Data", false)
	requireCommandRule(t, cmd.Answer.Rule, "Vendor-Specific-Application-Id", true)
	requireCommandRule(t, cmd.Answer.Rule, "Result-Code", false)
	requireCommandRule(t, cmd.Answer.Rule, "Experimental-Result", false)
}

func TestCxAndSWxOperationalCommandsDoNotCollide(t *testing.T) {
	loadTestDict(t)

	cxRTR, err := dict.Default.FindCommand(cx.AppIDCx, cmdRTR)
	if err != nil {
		t.Fatalf("find Cx RTR command: %v", err)
	}
	swxRTR, err := dict.Default.FindCommand(AppIDSWx, cmdRTR)
	if err != nil {
		t.Fatalf("find SWx RTR command: %v", err)
	}
	if cxRTR == swxRTR {
		t.Fatal("Cx and SWx RTR resolved to the same command definition")
	}

	cxPPR, err := dict.Default.FindCommand(cx.AppIDCx, cmdPPR)
	if err != nil {
		t.Fatalf("find Cx PPR command: %v", err)
	}
	swxPPR, err := dict.Default.FindCommand(AppIDSWx, cmdPPR)
	if err != nil {
		t.Fatalf("find SWx PPR command: %v", err)
	}
	if cxPPR == swxPPR {
		t.Fatal("Cx and SWx PPR resolved to the same command definition")
	}
}

func TestSARUnmarshalWithServerAssignmentType(t *testing.T) {
	loadTestDict(t)

	msg := diam.NewRequest(301, AppIDSWx, dict.Default)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String("sar;1"))
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity("aaa.example.net"))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity("example.net"))
	msg.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity("example.com"))
	msg.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String("001010000000001"))
	msg.NewAVP(avpServerAssignmentType, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Enumerated(sarRegistration))

	var req SAR
	if err := msg.Unmarshal(&req); err != nil {
		t.Fatalf("unmarshal SAR: %v", err)
	}
	if req.ServerAssignmentType != datatype.Enumerated(sarRegistration) {
		t.Fatalf("Server-Assignment-Type: got %d", req.ServerAssignmentType)
	}
}

func TestMARUnmarshalWithSIPAuthDataItem(t *testing.T) {
	loadTestDict(t)

	const anid = "wlan.mnc435.mcc311.3gppnetwork.org"

	msg := diam.NewRequest(303, AppIDSWx, dict.Default)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String("mar;1"))
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity("aaa.example.net"))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity("example.net"))
	msg.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity("example.com"))
	msg.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String("311435000070571"))
	msg.NewAVP(avpRATType, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Enumerated(1))
	msg.NewAVP(avpSIPNumberAuthItems, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(1))
	msg.NewAVP(avpSIPAuthDataItem, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: []*diam.AVP{
		diam.NewAVP(avpSIPAuthenticationScheme, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.UTF8String("EAP-AKA'")),
		diam.NewAVP(avpSIPAuthorization, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.OctetString([]byte("identity-response"))),
		diam.NewAVP(avpSIPAuthenticationContext, avp.Vbit, Vendor3GPP, datatype.OctetString([]byte(anid))),
	}})

	var req MAR
	if err := msg.Unmarshal(&req); err != nil {
		t.Fatalf("unmarshal MAR: %v", err)
	}
	if req.SIPAuthDataItem == nil {
		t.Fatal("missing SIP-Auth-Data-Item")
	}
	if req.UserName != "311435000070571" {
		t.Fatalf("User-Name: got %q", req.UserName)
	}
	if req.SIPAuthDataItem.SIPAuthenticationScheme != "EAP-AKA'" {
		t.Fatalf("SIP-Authentication-Scheme: got %q", req.SIPAuthDataItem.SIPAuthenticationScheme)
	}
	if got := string(req.SIPAuthDataItem.SIPAuthenticationContext); got != anid {
		t.Fatalf("SIP-Authentication-Context: got %q want %q", got, anid)
	}
	if req.SIPNumberAuthItems != 1 {
		t.Fatalf("SIP-Number-Auth-Items: got %d", req.SIPNumberAuthItems)
	}
}

func requireCommandRule(t *testing.T, rules []*dict.Rule, avpName string, required bool) {
	t.Helper()
	for _, rule := range rules {
		if rule.AVP == avpName {
			if rule.Required != required {
				t.Fatalf("%s required: got %v want %v", avpName, rule.Required, required)
			}
			return
		}
	}
	t.Fatalf("missing command rule for %s", avpName)
}
