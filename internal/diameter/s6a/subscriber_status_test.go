package s6a

import (
	"testing"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

// singleAPNStore builds a subscriber with one APN and the given
// Subscriber-Status / Operator-Determined-Barring values (TS 29.272
// §7.3.29/§7.3.30), for exercising the shared Subscription-Data builder.
func singleAPNStore(status int, odb uint32) *s6aTestStore {
	return &s6aTestStore{
		sub: &models.Subscriber{
			SubscriberID:              42,
			IMSI:                      "204950000000900",
			AUCID:                     1,
			DefaultAPN:                1,
			APNList:                   "1",
			SubscribedRAUTAUTimer:     300,
			SubscriberStatus:          status,
			OperatorDeterminedBarring: odb,
		},
		apn: &models.APN{APNID: 1, APN: "internet", IPVersion: 0, QCI: 9, ARPPriority: 4, APNAMBRDown: 1000, APNAMBRUp: 1000, ChargingCharacteristics: "0800"},
	}
}

// subscriptionDataAVPs decodes the Subscription-Data grouped AVP from a ULA
// and returns Subscriber-Status and Operator-Determined-Barring, plus whether
// the latter was present at all (it must be conditionally omitted).
func subscriptionDataAVPs(t *testing.T, ula *diam.Message) (status int32, odb uint32, odbPresent bool) {
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
		switch a.Code {
		case avp.SubscriberStatus:
			if v, ok := a.Data.(datatype.Enumerated); ok {
				status = int32(v)
			}
		case avp.OperatorDeterminedBarring:
			odbPresent = true
			if v, ok := a.Data.(datatype.Unsigned32); ok {
				odb = uint32(v)
			}
		}
	}
	return status, odb, odbPresent
}

func TestULA_SubscriberStatus_ServiceGranted_EncodesZero_NoODB(t *testing.T) {
	ula := runULA(t, singleAPNStore(models.SubscriberStatusServiceGranted, 0))

	status, _, odbPresent := subscriptionDataAVPs(t, ula)
	if status != 0 {
		t.Fatalf("Subscriber-Status = %d, want 0 (SERVICE_GRANTED)", status)
	}
	if odbPresent {
		t.Fatal("Operator-Determined-Barring AVP present for SERVICE_GRANTED subscriber, want omitted")
	}
}

func TestULA_SubscriberStatus_OperatorDeterminedBarring_EncodesOne(t *testing.T) {
	ula := runULA(t, singleAPNStore(models.SubscriberStatusOperatorDeterminedBarring, models.ODBAllPacketOrientedServicesBarred))

	status, odb, odbPresent := subscriptionDataAVPs(t, ula)
	if status != 1 {
		t.Fatalf("Subscriber-Status = %d, want 1 (OPERATOR_DETERMINED_BARRING)", status)
	}
	if !odbPresent {
		t.Fatal("Operator-Determined-Barring AVP missing for barred subscriber")
	}
	if odb != 0x00000001 {
		t.Fatalf("Operator-Determined-Barring = 0x%08x, want 0x00000001", odb)
	}
}

func TestULA_ODB_BitEncodingMatchesSpecPositions(t *testing.T) {
	cases := []struct {
		name string
		mask uint32
		want uint32
	}{
		{"bit0_AllPacketOrientedServicesBarred", models.ODBAllPacketOrientedServicesBarred, 0x00000001},
		{"bit1_RoamerAccessHPLMNAPBarred", models.ODBRoamerAccessHPLMNAPBarred, 0x00000002},
		{"bit8_InternationalExceptHomePLMNAndInterZonal", models.ODBBarringOfInternationalExceptHomePLMNAndInterZonal, 0x00000100},
		{"bits0and3_combined", models.ODBAllPacketOrientedServicesBarred | models.ODBBarringOfAllOutgoingCalls, 0x00000009},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ula := runULA(t, singleAPNStore(models.SubscriberStatusOperatorDeterminedBarring, tc.mask))
			_, odb, odbPresent := subscriptionDataAVPs(t, ula)
			if !odbPresent {
				t.Fatal("Operator-Determined-Barring AVP missing")
			}
			if odb != tc.want {
				t.Fatalf("Operator-Determined-Barring = 0x%08x, want 0x%08x", odb, tc.want)
			}
		})
	}
}

// TestULA_ODB_NotEmittedWhenServiceGranted_EvenWithStoredMask is a
// defense-in-depth check at the Diameter encoding layer: even if a
// SERVICE_GRANTED subscriber somehow still has a non-zero stored ODB mask,
// the S6a builder must never place it in active Subscription-Data.
func TestULA_ODB_NotEmittedWhenServiceGranted_EvenWithStoredMask(t *testing.T) {
	ula := runULA(t, singleAPNStore(models.SubscriberStatusServiceGranted, models.ODBAllPacketOrientedServicesBarred))

	status, _, odbPresent := subscriptionDataAVPs(t, ula)
	if status != 0 {
		t.Fatalf("Subscriber-Status = %d, want 0 (SERVICE_GRANTED)", status)
	}
	if odbPresent {
		t.Fatal("Operator-Determined-Barring AVP present despite SERVICE_GRANTED status")
	}
}
