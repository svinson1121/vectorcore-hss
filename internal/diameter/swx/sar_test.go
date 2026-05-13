package swx

import (
	"context"
	"testing"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/models"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

type swxSARStore struct {
	repository.Repository
	sub  *models.Subscriber
	apns map[int]*models.APN
}

func (s *swxSARStore) GetSubscriberByIMSI(_ context.Context, imsi string) (*models.Subscriber, error) {
	if s.sub != nil && s.sub.IMSI == imsi {
		return s.sub, nil
	}
	return nil, repository.ErrNotFound
}

func (s *swxSARStore) GetAPNByID(_ context.Context, apnID int) (*models.APN, error) {
	if apn, ok := s.apns[apnID]; ok {
		return apn, nil
	}
	return nil, repository.ErrNotFound
}

func TestSARIncludesNon3GPPAPNAuthorizationData(t *testing.T) {
	loadTestDict(t)

	enabled := true
	store := &swxSARStore{
		sub: &models.Subscriber{
			IMSI:       "001010000000001",
			Enabled:    &enabled,
			DefaultAPN: 1,
			APNList:    "1",
		},
		apns: map[int]*models.APN{
			1: {
				APNID:                   1,
				APN:                     "internet",
				IPVersion:               0,
				APNAMBRDown:             1000000,
				APNAMBRUp:               500000,
				QCI:                     9,
				ARPPriority:             8,
				ChargingCharacteristics: "0800",
			},
		},
	}
	h := NewHandlers(&config.Config{
		HSS: config.HSSConfig{OriginHost: "hss.example.net", OriginRealm: "example.net"},
	}, store, zap.NewNop(), nil)

	ans, err := h.SAR(nil, newSARRequest("001010000000001"))
	if err != nil {
		t.Fatalf("SAR failed: %v", err)
	}

	userData := findAVPDirect(ans, avpNon3GPPUserData, Vendor3GPP)
	if userData == nil {
		t.Fatal("missing Non-3GPP-User-Data")
	}
	group, ok := userData.Data.(*diam.GroupedAVP)
	if !ok {
		t.Fatalf("Non-3GPP-User-Data type: %T", userData.Data)
	}

	requireEnumeratedChild(t, group, avpNon3GPPIPAccess, Vendor3GPP, Non3GPPAccessAllowed)
	requireEnumeratedChild(t, group, avpNon3GPPIPAccessAPN, Vendor3GPP, Non3GPPAPNsEnable)

	apnConfig := findGroupedChildAVP(group, avp.APNConfiguration, Vendor3GPP)
	if apnConfig == nil {
		t.Fatal("missing APN-Configuration")
	}
	apnGroup, ok := apnConfig.Data.(*diam.GroupedAVP)
	if !ok {
		t.Fatalf("APN-Configuration type: %T", apnConfig.Data)
	}
	requireUnsigned32Child(t, apnGroup, avp.ContextIdentifier, Vendor3GPP, 1)
	requireEnumeratedChild(t, apnGroup, avp.PDNType, Vendor3GPP, 0)
	requireUTF8Child(t, apnGroup, avp.ServiceSelection, 0, "internet")
	if findGroupedChildAVP(apnGroup, avp.AMBR, Vendor3GPP) == nil {
		t.Fatal("missing APN AMBR")
	}
	if findGroupedChildAVP(apnGroup, avp.EPSSubscribedQoSProfile, Vendor3GPP) == nil {
		t.Fatal("missing EPS-Subscribed-QoS-Profile")
	}
}

func newSARRequest(imsi string) *diam.Message {
	msg := diam.NewRequest(301, AppIDSWx, dict.Default)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String("sar;1"))
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity("aaa.example.net"))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity("example.net"))
	msg.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity("example.net"))
	msg.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))
	msg.NewAVP(avpServerAssignmentType, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Enumerated(sarRegistration))
	return msg
}

func findAVPDirect(msg *diam.Message, code uint32, vendor uint32) *diam.AVP {
	for _, a := range msg.AVP {
		if a.Code == code && a.VendorID == vendor {
			return a
		}
	}
	return nil
}

func findGroupedChildAVP(group *diam.GroupedAVP, code uint32, vendor uint32) *diam.AVP {
	for _, a := range group.AVP {
		if a.Code == code && a.VendorID == vendor {
			return a
		}
	}
	return nil
}

func requireEnumeratedChild(t *testing.T, group *diam.GroupedAVP, code uint32, vendor uint32, want int) {
	t.Helper()
	a := findGroupedChildAVP(group, code, vendor)
	if a == nil {
		t.Fatalf("missing AVP %d/%d", code, vendor)
	}
	got, ok := a.Data.(datatype.Enumerated)
	if !ok {
		t.Fatalf("AVP %d/%d type: %T", code, vendor, a.Data)
	}
	if int(got) != want {
		t.Fatalf("AVP %d/%d: got %d want %d", code, vendor, got, want)
	}
}

func requireUnsigned32Child(t *testing.T, group *diam.GroupedAVP, code uint32, vendor uint32, want uint32) {
	t.Helper()
	a := findGroupedChildAVP(group, code, vendor)
	if a == nil {
		t.Fatalf("missing AVP %d/%d", code, vendor)
	}
	got, ok := a.Data.(datatype.Unsigned32)
	if !ok {
		t.Fatalf("AVP %d/%d type: %T", code, vendor, a.Data)
	}
	if uint32(got) != want {
		t.Fatalf("AVP %d/%d: got %d want %d", code, vendor, got, want)
	}
}

func requireUTF8Child(t *testing.T, group *diam.GroupedAVP, code uint32, vendor uint32, want string) {
	t.Helper()
	a := findGroupedChildAVP(group, code, vendor)
	if a == nil {
		t.Fatalf("missing AVP %d/%d", code, vendor)
	}
	got, ok := a.Data.(datatype.UTF8String)
	if !ok {
		t.Fatalf("AVP %d/%d type: %T", code, vendor, a.Data)
	}
	if string(got) != want {
		t.Fatalf("AVP %d/%d: got %q want %q", code, vendor, got, want)
	}
}
