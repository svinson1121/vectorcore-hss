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

type swxMARStore struct {
	repository.Repository
	auc *models.AUC
}

func (s *swxMARStore) GetAUCByIMSI(_ context.Context, imsi string) (*models.AUC, error) {
	if s.auc != nil && s.auc.IMSI != nil && *s.auc.IMSI == imsi {
		return s.auc, nil
	}
	return nil, repository.ErrNotFound
}

func (s *swxMARStore) AtomicGetAndIncrementSQN(_ context.Context, aucID int, delta int64) (*models.AUC, error) {
	before := *s.auc
	s.auc.SQN += delta
	return &before, nil
}

func TestMARRequestingEAPAKASucceedsWithoutANID(t *testing.T) {
	h := newMARHandler(t)

	ans, err := h.MAR(nil, newMARRequest(testSWxIMSI, schemeEAPAKA, ""))
	if err != nil {
		t.Fatalf("MAR failed: %v", err)
	}

	requireSuccess(t, ans)
	group := requireAuthDataItem(t, ans)
	requireUTF8Child(t, group, avpSIPAuthenticationScheme, Vendor3GPP, schemeEAPAKA)
	requireOctetChildLen(t, group, avpSIPAuthenticate, Vendor3GPP, 32)
	requireOctetChildLen(t, group, avpSIPAuthorization, Vendor3GPP, 8)
	requireOctetChildLen(t, group, avpConfidentialityKey, Vendor3GPP, 16)
	requireOctetChildLen(t, group, avpIntegrityKey, Vendor3GPP, 16)
}

func TestMARRequestingEAPAKAPrimeSucceedsWithANID(t *testing.T) {
	h := newMARHandler(t)

	ans, err := h.MAR(nil, newMARRequest(testSWxIMSI, schemeEAPAKAPrimeUnicode, "wlan.mnc001.mcc001.3gppnetwork.org"))
	if err != nil {
		t.Fatalf("MAR failed: %v", err)
	}

	requireSuccess(t, ans)
	group := requireAuthDataItem(t, ans)
	requireUTF8Child(t, group, avpSIPAuthenticationScheme, Vendor3GPP, schemeEAPAKAPrime)
	requireOctetChildLen(t, group, avpSIPAuthenticate, Vendor3GPP, 32)
	requireOctetChildLen(t, group, avpSIPAuthorization, Vendor3GPP, 8)
	requireOctetChildLen(t, group, avpConfidentialityKey, Vendor3GPP, 16)
	requireOctetChildLen(t, group, avpIntegrityKey, Vendor3GPP, 16)
}

func TestMARTrustedWLANPathRequestsEAPAKAPrime(t *testing.T) {
	h := newMARHandler(t)

	ans, err := h.MAR(nil, newMARRequest(testSWxIMSI, schemeEAPAKAPrime, "twag.example.net"))
	if err != nil {
		t.Fatalf("MAR failed: %v", err)
	}

	requireSuccess(t, ans)
	group := requireAuthDataItem(t, ans)
	requireUTF8Child(t, group, avpSIPAuthenticationScheme, Vendor3GPP, schemeEAPAKAPrime)
}

func TestMARUntrustedEPDGPathRequestsEAPAKA(t *testing.T) {
	h := newMARHandler(t)

	ans, err := h.MAR(nil, newMARRequest(testSWxIMSI, schemeEAPAKA, ""))
	if err != nil {
		t.Fatalf("MAR failed: %v", err)
	}

	requireSuccess(t, ans)
	group := requireAuthDataItem(t, ans)
	requireUTF8Child(t, group, avpSIPAuthenticationScheme, Vendor3GPP, schemeEAPAKA)
}

func TestMARRequestingEAPAKAPrimeWithoutANIDFails(t *testing.T) {
	h := newMARHandler(t)

	ans, err := h.MAR(nil, newMARRequest(testSWxIMSI, schemeEAPAKAPrime, ""))
	if err != nil {
		t.Fatalf("MAR returned transport error: %v", err)
	}

	requireFailure(t, ans)
	if findAVPDirect(ans, avpSIPAuthDataItem, Vendor3GPP) != nil {
		t.Fatal("failure answer must not include auth vector material")
	}
}

func TestMARRequestingEAPSIMFails(t *testing.T) {
	h := newMARHandler(t)

	ans, err := h.MAR(nil, newMARRequest(testSWxIMSI, schemeEAPSIM, ""))
	if err != nil {
		t.Fatalf("MAR returned transport error: %v", err)
	}

	requireFailure(t, ans)
	if findAVPDirect(ans, avpSIPAuthDataItem, Vendor3GPP) != nil {
		t.Fatal("failure answer must not include auth vector material")
	}
}

const testSWxIMSI = "311435000000001"

func newMARHandler(t *testing.T) *Handlers {
	t.Helper()
	loadTestDict(t)

	imsi := testSWxIMSI
	store := &swxMARStore{auc: &models.AUC{
		AUCID: 1,
		Ki:    "465b5ce8b199b49faa5f0a2ee238a6bc",
		OPc:   "cd63cb71954a9f4e48a5994e37a02baf",
		AMF:   "8000",
		SQN:   0,
		IMSI:  &imsi,
	}}
	return NewHandlers(&config.Config{
		HSS: config.HSSConfig{OriginHost: "hss.example.net", OriginRealm: "example.net"},
	}, store, zap.NewNop(), nil)
}

func newMARRequest(imsi, scheme, anid string) *diam.Message {
	msg := diam.NewRequest(303, AppIDSWx, dict.Default)
	msg.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String("mar;1"))
	msg.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	msg.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity("aaa.example.net"))
	msg.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity("example.net"))
	msg.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity("example.net"))
	msg.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))
	msg.NewAVP(avpRATType, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Enumerated(0))
	msg.NewAVP(avpSIPNumberAuthItems, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(1))
	msg.NewAVP(avpSIPAuthDataItem, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: []*diam.AVP{
		diam.NewAVP(avpSIPAuthenticationScheme, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.UTF8String(scheme)),
	}})
	if anid != "" {
		msg.NewAVP(avpANID, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.UTF8String(anid))
		msg.NewAVP(avpANTrusted, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Enumerated(ANTrusted))
	}
	return msg
}

func requireSuccess(t *testing.T, msg *diam.Message) {
	t.Helper()
	a := findAVPDirect(msg, avp.ResultCode, 0)
	if a == nil {
		t.Fatal("missing Result-Code")
	}
	got, ok := a.Data.(datatype.Unsigned32)
	if !ok || uint32(got) != diam.Success {
		t.Fatalf("Result-Code: got %v (%T), want %d", a.Data, a.Data, diam.Success)
	}
}

func requireFailure(t *testing.T, msg *diam.Message) {
	t.Helper()
	if findAVPDirect(msg, avp.ExperimentalResult, 0) == nil {
		t.Fatal("missing Experimental-Result")
	}
	if findAVPDirect(msg, avp.ResultCode, 0) != nil {
		t.Fatal("failure answer unexpectedly included Result-Code")
	}
}

func requireAuthDataItem(t *testing.T, msg *diam.Message) *diam.GroupedAVP {
	t.Helper()
	a := findAVPDirect(msg, avpSIPAuthDataItem, Vendor3GPP)
	if a == nil {
		t.Fatal("missing SIP-Auth-Data-Item")
	}
	group, ok := a.Data.(*diam.GroupedAVP)
	if !ok {
		t.Fatalf("SIP-Auth-Data-Item type: %T", a.Data)
	}
	return group
}

func requireOctetChildLen(t *testing.T, group *diam.GroupedAVP, code uint32, vendor uint32, want int) {
	t.Helper()
	a := findGroupedChildAVP(group, code, vendor)
	if a == nil {
		t.Fatalf("missing AVP %d/%d", code, vendor)
	}
	got, ok := a.Data.(datatype.OctetString)
	if !ok {
		t.Fatalf("AVP %d/%d type: %T", code, vendor, a.Data)
	}
	if len(got) != want {
		t.Fatalf("AVP %d/%d length: got %d want %d", code, vendor, len(got), want)
	}
}
