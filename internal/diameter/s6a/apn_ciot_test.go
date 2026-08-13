package s6a

import (
	"encoding/binary"
	"testing"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

// The CIoT/NIDD AVP codes (avpNonIPPDNTypeIndicator etc., define.go) are not
// registered in go-diameter's base dictionary, since they don't exist in the
// library. After a round-trip wire serialize/decode (decodeMessageForTest),
// the dictionary-less decoder can't determine their type and falls back to
// datatype.Unknown (raw bytes) instead of Enumerated/Unsigned32/
// DiameterIdentity. avpUint32/avpString below read the value either way.

func avpUint32(t *testing.T, d datatype.Type) uint32 {
	t.Helper()
	switch v := d.(type) {
	case datatype.Enumerated:
		return uint32(v)
	case datatype.Unsigned32:
		return uint32(v)
	case datatype.Unknown:
		b := v.Serialize()
		if len(b) != 4 {
			t.Fatalf("Unknown AVP data has unexpected length %d, want 4", len(b))
		}
		return binary.BigEndian.Uint32(b)
	default:
		t.Fatalf("unexpected AVP data type %T", d)
		return 0
	}
}

func avpString(t *testing.T, d datatype.Type) string {
	t.Helper()
	switch v := d.(type) {
	case datatype.DiameterIdentity:
		return string(v)
	case datatype.UTF8String:
		return string(v)
	case datatype.OctetString:
		return string(v)
	case datatype.Unknown:
		return string(v.Serialize())
	default:
		t.Fatalf("unexpected AVP data type %T", d)
		return ""
	}
}

func boolp(v bool) *bool { return &v }
func intp(v int) *int    { return &v }

// ciotAPNStore builds a subscriber with a single APN carrying the given CIoT
// configuration, for exercising the shared Subscription-Data builder.
func ciotAPNStore(a *models.APN) *s6aTestStore {
	a.APNID = 1
	a.APN = "iot"
	a.IPVersion = 0
	a.QCI = 9
	a.ARPPriority = 4
	a.APNAMBRDown = 1000
	a.APNAMBRUp = 1000
	a.ChargingCharacteristics = "0800"
	return &s6aTestStore{
		sub: &models.Subscriber{
			SubscriberID:          42,
			IMSI:                  "204950000000902",
			AUCID:                 1,
			DefaultAPN:            1,
			APNList:               "1",
			SubscribedRAUTAUTimer: 300,
		},
		apn: a,
	}
}

// ciotAPNAVPs decodes the single APN-Configuration in a ULA's
// APN-Configuration-Profile and returns the raw AVP codes present, plus a
// lookup of decoded values by code for the ones this test cares about.
func ciotAPNAVPs(t *testing.T, ula *diam.Message) map[uint32]*diam.AVP {
	t.Helper()

	subData, err := ula.FindAVP(avp.SubscriptionData, Vendor3GPP)
	if err != nil || subData == nil {
		t.Fatal("missing Subscription-Data AVP")
	}
	subGrp, ok := subData.Data.(*diam.GroupedAVP)
	if !ok {
		t.Fatalf("Subscription-Data has unexpected type %T", subData.Data)
	}

	var profile *diam.GroupedAVP
	for _, a := range subGrp.AVP {
		if a.Code == avp.APNConfigurationProfile {
			profile, _ = a.Data.(*diam.GroupedAVP)
		}
	}
	if profile == nil {
		t.Fatal("missing APN-Configuration-Profile AVP")
	}

	for _, a := range profile.AVP {
		if a.Code != avp.APNConfiguration {
			continue
		}
		apnGrp, ok := a.Data.(*diam.GroupedAVP)
		if !ok {
			continue
		}
		out := make(map[uint32]*diam.AVP)
		for _, sub := range apnGrp.AVP {
			out[sub.Code] = sub
		}
		return out
	}
	t.Fatal("missing APN-Configuration AVP")
	return nil
}

// TestULA_CIoTDisabled_OmitsAllNIDDAVPs is a defense-in-depth check: even
// with NIDD fields populated in the DB, an APN with CIoT features disabled
// must not leak any of them into Subscription-Data.
func TestULA_CIoTDisabled_OmitsAllNIDDAVPs(t *testing.T) {
	ula := runULA(t, ciotAPNStore(&models.APN{
		CIoTEnabled:           boolp(false),
		NonIPPDN:              boolp(true),
		NIDDMechanism:         intp(models.NIDDMechanismSCEFBased),
		NIDDScefID:            strptr("scef.example.net"),
		NIDDScefRealm:         strptr("example.net"),
		NIDDRDS:               intp(models.RDSEnabled),
		NIDDPreferredDataMode: intp(models.PreferredDataModeUserPlane),
	}))

	got := ciotAPNAVPs(t, ula)
	for _, code := range []uint32{avpNonIPPDNTypeIndicator, avpNonIPDataDeliveryMechanism, avpSCEFID, avpSCEFRealm, avpRDSIndicator, avpPreferredDataMode} {
		if _, present := got[code]; present {
			t.Fatalf("AVP code %d present despite CIoTEnabled=false", code)
		}
	}
}

// TestULA_CIoT_NonIPPDN_SCEFBased verifies the full SCEF-based Non-IP PDN AVP
// set is emitted with the correct values when CIoT features and Non-IP PDN
// are enabled with mechanism=SCEF-based.
func TestULA_CIoT_NonIPPDN_SCEFBased(t *testing.T) {
	ula := runULA(t, ciotAPNStore(&models.APN{
		CIoTEnabled:   boolp(true),
		NonIPPDN:      boolp(true),
		NIDDMechanism: intp(models.NIDDMechanismSCEFBased),
		NIDDScefID:    strptr("scef.example.net"),
		NIDDScefRealm: strptr("example.net"),
		NIDDRDS:       intp(models.RDSEnabled),
	}))

	got := ciotAPNAVPs(t, ula)

	nonIP, ok := got[avpNonIPPDNTypeIndicator]
	if !ok {
		t.Fatal("missing Non-IP-PDN-Type-Indicator AVP")
	}
	if v := avpUint32(t, nonIP.Data); v != 1 {
		t.Fatalf("Non-IP-PDN-Type-Indicator = %d, want TRUE (1)", v)
	}

	mech, ok := got[avpNonIPDataDeliveryMechanism]
	if !ok {
		t.Fatal("missing Non-IP-Data-Delivery-Mechanism AVP")
	}
	if v := avpUint32(t, mech.Data); v != models.NIDDMechanismSCEFBased {
		t.Fatalf("Non-IP-Data-Delivery-Mechanism = %d, want SCEF-BASED (1)", v)
	}

	scefID, ok := got[avpSCEFID]
	if !ok {
		t.Fatal("missing SCEF-ID AVP")
	}
	if v := avpString(t, scefID.Data); v != "scef.example.net" {
		t.Fatalf("SCEF-ID = %q, want scef.example.net", v)
	}

	scefRealm, ok := got[avpSCEFRealm]
	if !ok {
		t.Fatal("missing SCEF-Realm AVP")
	}
	if v := avpString(t, scefRealm.Data); v != "example.net" {
		t.Fatalf("SCEF-Realm = %q, want example.net", v)
	}

	rds, ok := got[avpRDSIndicator]
	if !ok {
		t.Fatal("missing RDS-Indicator AVP")
	}
	if v := avpUint32(t, rds.Data); v != models.RDSEnabled {
		t.Fatalf("RDS-Indicator = %d, want ENABLED (1)", v)
	}
}

// TestULA_CIoT_NonIPPDN_SGiBased_OmitsSCEFFields confirms SCEF-ID/SCEF-Realm
// are only emitted for the SCEF-based delivery mechanism, per TS 29.272
// §7.3.204-207 presence rules, even if stored in the DB.
func TestULA_CIoT_NonIPPDN_SGiBased_OmitsSCEFFields(t *testing.T) {
	ula := runULA(t, ciotAPNStore(&models.APN{
		CIoTEnabled:   boolp(true),
		NonIPPDN:      boolp(true),
		NIDDMechanism: intp(models.NIDDMechanismSGiBased),
		NIDDScefID:    strptr("scef.example.net"), // must still be omitted: wrong mechanism
		NIDDScefRealm: strptr("example.net"),
	}))

	got := ciotAPNAVPs(t, ula)
	mech, ok := got[avpNonIPDataDeliveryMechanism]
	if !ok {
		t.Fatal("missing Non-IP-Data-Delivery-Mechanism AVP")
	}
	if v := avpUint32(t, mech.Data); v != models.NIDDMechanismSGiBased {
		t.Fatalf("Non-IP-Data-Delivery-Mechanism = %d, want SGi-BASED (0)", v)
	}
	if _, present := got[avpSCEFID]; present {
		t.Fatal("SCEF-ID present for SGi-based mechanism, want omitted")
	}
	if _, present := got[avpSCEFRealm]; present {
		t.Fatal("SCEF-Realm present for SGi-based mechanism, want omitted")
	}
}

// TestULA_CIoT_PreferredDataMode covers the Preferred-Data-Mode bitmask
// (TS 29.272 §7.3.209): both single-bit values, the combined mask, and the
// omitted-when-unset case. Unlike Non-IP PDN fields, this applies whenever
// CIoT features are enabled, independent of NonIPPDN.
func TestULA_CIoT_PreferredDataMode(t *testing.T) {
	cases := []struct {
		name    string
		mode    *int
		wantSet bool
		want    uint32
	}{
		{"UserPlaneOnly", intp(models.PreferredDataModeUserPlane), true, 1},
		{"ControlPlaneOnly", intp(models.PreferredDataModeControlPlane), true, 2},
		{"Both", intp(models.PreferredDataModeUserPlane | models.PreferredDataModeControlPlane), true, 3},
		{"Unset_Omitted", nil, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ula := runULA(t, ciotAPNStore(&models.APN{
				CIoTEnabled:           boolp(true),
				NIDDPreferredDataMode: tc.mode,
			}))
			got := ciotAPNAVPs(t, ula)
			pdm, present := got[avpPreferredDataMode]
			if present != tc.wantSet {
				t.Fatalf("Preferred-Data-Mode present=%v, want %v", present, tc.wantSet)
			}
			if !tc.wantSet {
				return
			}
			if v := avpUint32(t, pdm.Data); v != tc.want {
				t.Fatalf("Preferred-Data-Mode = %d, want %d", v, tc.want)
			}
		})
	}
}
