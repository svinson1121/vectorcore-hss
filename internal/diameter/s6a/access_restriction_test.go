package s6a

import (
	"testing"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

func u32ptr(v uint32) *uint32 { return &v }

// ardStore builds a subscriber with one APN and the given
// Access-Restriction-Data mask (TS 29.272 §7.3.31), for exercising the shared
// Subscription-Data builder.
func ardStore(ard *uint32) *s6aTestStore {
	return &s6aTestStore{
		sub: &models.Subscriber{
			SubscriberID:          42,
			IMSI:                  "204950000000901",
			AUCID:                 1,
			DefaultAPN:            1,
			APNList:               "1",
			SubscribedRAUTAUTimer: 300,
			AccessRestrictionData: ard,
		},
		apn: &models.APN{APNID: 1, APN: "internet", IPVersion: 0, QCI: 9, ARPPriority: 4, APNAMBRDown: 1000, APNAMBRUp: 1000, ChargingCharacteristics: "0800"},
	}
}

// findAccessRestrictionData decodes the Access-Restriction-Data AVP from a
// ULA's Subscription-Data.
func findAccessRestrictionData(t *testing.T, ula *diam.Message) uint32 {
	t.Helper()

	subData, err := ula.FindAVP(avp.SubscriptionData, Vendor3GPP)
	if err != nil || subData == nil {
		t.Fatal("missing Subscription-Data AVP")
	}
	subGrp, ok := subData.Data.(*diam.GroupedAVP)
	if !ok {
		t.Fatalf("Subscription-Data has unexpected type %T", subData.Data)
	}
	for _, a := range subGrp.AVP {
		if a.Code == avp.AccessRestrictionData {
			if v, ok := a.Data.(datatype.Unsigned32); ok {
				return uint32(v)
			}
		}
	}
	t.Fatal("missing Access-Restriction-Data AVP")
	return 0
}

// TestULA_AccessRestrictionData_BitEncodingMatchesSpecPositions verifies that
// individual and combined Access-Restriction-Data bits reach the wire AVP
// exactly as stored, per TS 29.272 §7.3.31 (handoff §15).
func TestULA_AccessRestrictionData_BitEncodingMatchesSpecPositions(t *testing.T) {
	cases := []struct {
		name string
		mask uint32
		want uint32
	}{
		{"NBIoTNotAllowed", models.ARDNBIoTNotAllowed, 0x00000040},
		{"EnhancedCoverageNotAllowed", models.ARDEnhancedCoverageNotAllowed, 0x00000080},
		{"NRAsSecondaryRATNotAllowed", models.ARDNRAsSecondaryRATNotAllowed, 0x00000100},
		{"LTEMNotAllowed", models.ARDLTEMNotAllowed, 0x00000800},
		{"WBEUTRANExceptLTEMNotAllowed", models.ARDWBEUTRANExceptLTEMNotAllowed, 0x00001000},
		{
			"NBIoT_plus_LTEM_combined",
			models.ARDNBIoTNotAllowed | models.ARDLTEMNotAllowed,
			0x00000840,
		},
		{
			"NBIoT_plus_LTEM_preserves_unrelated_UTRAN_bit",
			models.ARDUTRANNotAllowed | models.ARDNBIoTNotAllowed | models.ARDLTEMNotAllowed,
			0x00000841,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ula := runULA(t, ardStore(u32ptr(tc.mask)))
			got := findAccessRestrictionData(t, ula)
			if got != tc.want {
				t.Fatalf("Access-Restriction-Data = 0x%08x, want 0x%08x", got, tc.want)
			}
		})
	}
}

// TestULA_AccessRestrictionData_NilDefaultsToZero confirms the documented
// backward-compatible default: an unset stored mask sends 0 (no restrictions).
func TestULA_AccessRestrictionData_NilDefaultsToZero(t *testing.T) {
	ula := runULA(t, ardStore(nil))
	got := findAccessRestrictionData(t, ula)
	if got != 0 {
		t.Fatalf("Access-Restriction-Data = 0x%08x, want 0 for unset mask", got)
	}
}
