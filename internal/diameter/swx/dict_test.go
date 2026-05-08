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

	msg := diam.NewRequest(303, AppIDSWx, dict.Default)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String("mar;1"))
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity("aaa.example.net"))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity("example.net"))
	msg.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity("example.com"))
	msg.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String("001010000000001"))
	msg.NewAVP(avpSIPNumberAuthItems, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(1))
	msg.NewAVP(avpSIPAuthDataItem, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: []*diam.AVP{
		diam.NewAVP(avpSIPAuthenticationScheme, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.UTF8String("EAP-AKA")),
	}})

	var req MAR
	if err := msg.Unmarshal(&req); err != nil {
		t.Fatalf("unmarshal MAR: %v", err)
	}
	if req.SIPAuthDataItem == nil {
		t.Fatal("missing SIP-Auth-Data-Item")
	}
	if req.SIPAuthDataItem.SIPAuthenticationScheme != "EAP-AKA" {
		t.Fatalf("SIP-Authentication-Scheme: got %q", req.SIPAuthDataItem.SIPAuthenticationScheme)
	}
}
