package s6a

import (
	"net"
	"testing"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

func strptr(s string) *string { return &s }

// apnStaticIP walks a decoded ULA and returns the Served-Party-IP-Address found
// inside the APN-Configuration whose Service-Selection equals apnName. The
// second return value reports whether the APN-Configuration itself was found;
// the returned IP is nil when the APN has no Served-Party-IP-Address AVP.
func apnStaticIP(t *testing.T, ula *diam.Message, apnName string) (net.IP, bool) {
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
		var name string
		var ip net.IP
		for _, sub := range apnGrp.AVP {
			switch sub.Code {
			case avp.ServiceSelection:
				// Service-Selection may decode as UTF8String or, depending on
				// dictionary vendor resolution, as raw bytes. Read both forms.
				if v, ok := sub.Data.(datatype.UTF8String); ok {
					name = string(v)
				} else if sub.Data != nil {
					name = string(sub.Data.Serialize())
				}
			case avp.ServedPartyIPAddress:
				if v, ok := sub.Data.(datatype.Address); ok {
					ip = net.IP(v)
				}
			}
		}
		if name == apnName {
			return ip, true
		}
	}
	return nil, false
}

// twoAPNStore builds a subscriber with two APNs (internet=1, camera=2) and lets
// the caller decide which APNs get a static IP via subscriber_routing.
func twoAPNStore(routing map[int]*models.SubscriberRouting) *s6aTestStore {
	return &s6aTestStore{
		sub: &models.Subscriber{
			SubscriberID:          42,
			IMSI:                  "204950000000755",
			AUCID:                 1,
			DefaultAPN:            1,
			APNList:               "1,2",
			SubscribedRAUTAUTimer: 300,
		},
		apns: map[int]*models.APN{
			1: {APNID: 1, APN: "internet", IPVersion: 0, QCI: 9, ARPPriority: 4, APNAMBRDown: 1000, APNAMBRUp: 1000, ChargingCharacteristics: "0800"},
			2: {APNID: 2, APN: "camera", IPVersion: 0, QCI: 9, ARPPriority: 4, APNAMBRDown: 1000, APNAMBRUp: 1000, ChargingCharacteristics: "0800"},
		},
		routing: routing,
	}
}

// runULA drives a ULR through the handler and returns the ULA re-decoded from
// its serialized wire form, so assertions run against the real Diameter bytes.
func runULA(t *testing.T, store *s6aTestStore) *diam.Message {
	t.Helper()
	h := newS6aTestHandlers(store)
	req := decodeMessageForTest(t, buildSMSAwareULR(t, store.sub.IMSI, "15551230001", true))
	ans, err := h.ULR(nil, req)
	if err != nil {
		t.Fatalf("ULR returned error: %v", err)
	}
	return decodeMessageForTest(t, ans)
}

// TestULA_NoStaticIP_OmitsAddressAVP confirms dynamic behavior is preserved:
// with no subscriber_routing record, no Served-Party-IP-Address is emitted.
func TestULA_NoStaticIP_OmitsAddressAVP(t *testing.T) {
	ula := runULA(t, twoAPNStore(nil))

	for _, apn := range []string{"internet", "camera"} {
		ip, found := apnStaticIP(t, ula, apn)
		if !found {
			t.Fatalf("APN %q missing from APN-Configuration-Profile", apn)
		}
		if ip != nil {
			t.Fatalf("APN %q should have no static IP, got %s", apn, ip)
		}
	}
}

// TestULA_StaticIP_EncodedPerAPN verifies each APN receives its own configured
// static IPv4 address and that addresses do not leak between APNs.
func TestULA_StaticIP_EncodedPerAPN(t *testing.T) {
	ula := runULA(t, twoAPNStore(map[int]*models.SubscriberRouting{
		1: {SubscriberID: 42, APNID: 1, IPAddress: strptr("10.45.3.29")},
		2: {SubscriberID: 42, APNID: 2, IPAddress: strptr("172.20.15.18")},
	}))

	want := map[string]string{
		"internet": "10.45.3.29",
		"camera":   "172.20.15.18",
	}
	for apn, expect := range want {
		ip, found := apnStaticIP(t, ula, apn)
		if !found {
			t.Fatalf("APN %q missing from APN-Configuration-Profile", apn)
		}
		if ip == nil {
			t.Fatalf("APN %q expected static IP %s, got none", apn, expect)
		}
		if !ip.Equal(net.ParseIP(expect)) {
			t.Fatalf("APN %q got static IP %s, want %s", apn, ip, expect)
		}
	}
}

// TestULA_StaticIP_DoesNotLeakBetweenAPNs sets a static IP on only one APN and
// asserts the other stays dynamic (no address AVP), even though both are built
// in the same Subscription-Data.
func TestULA_StaticIP_DoesNotLeakBetweenAPNs(t *testing.T) {
	ula := runULA(t, twoAPNStore(map[int]*models.SubscriberRouting{
		1: {SubscriberID: 42, APNID: 1, IPAddress: strptr("10.45.3.29")},
	}))

	ip, found := apnStaticIP(t, ula, "internet")
	if !found || ip == nil {
		t.Fatal("internet APN should carry static IP 10.45.3.29")
	}
	if !ip.Equal(net.ParseIP("10.45.3.29")) {
		t.Fatalf("internet APN got %s, want 10.45.3.29", ip)
	}

	cam, found := apnStaticIP(t, ula, "camera")
	if !found {
		t.Fatal("camera APN missing from APN-Configuration-Profile")
	}
	if cam != nil {
		t.Fatalf("camera APN must stay dynamic, but got static IP %s (leak)", cam)
	}
}

// TestULA_StaticIP_EmptyRoutingStaysDynamic confirms a subscriber_routing row
// with a nil/empty IP string is treated as dynamic, not encoded as an AVP.
func TestULA_StaticIP_EmptyRoutingStaysDynamic(t *testing.T) {
	ula := runULA(t, twoAPNStore(map[int]*models.SubscriberRouting{
		1: {SubscriberID: 42, APNID: 1, IPAddress: strptr("   ")},
		2: {SubscriberID: 42, APNID: 2, IPAddress: nil},
	}))

	for _, apn := range []string{"internet", "camera"} {
		ip, found := apnStaticIP(t, ula, apn)
		if !found {
			t.Fatalf("APN %q missing from APN-Configuration-Profile", apn)
		}
		if ip != nil {
			t.Fatalf("APN %q should stay dynamic, got %s", apn, ip)
		}
	}
}
