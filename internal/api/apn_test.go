package api

import (
	"context"
	"testing"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

func boolp(v bool) *bool    { return &v }
func intp(v int) *int       { return &v }
func strp(v string) *string { return &v }

func baseAPN(apnName string) models.APN {
	return models.APN{
		APN:         apnName,
		APNAMBRDown: 1000,
		APNAMBRUp:   1000,
	}
}

func TestCreateAPNRejectsNIDDFieldsWithoutNonIPPDN(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	a := baseAPN("iot1")
	a.CIoTEnabled = boolp(true)
	a.NIDDMechanism = intp(models.NIDDMechanismSCEFBased) // non_ip_pdn left unset
	if _, err := s.createAPN(ctx, &APNCreateInput{Body: &a}); err == nil {
		t.Fatal("expected validation error for nidd_mechanism without non_ip_pdn, got nil")
	}
}

func TestCreateAPNRejectsInvalidNIDDMechanism(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	a := baseAPN("iot2")
	a.CIoTEnabled = boolp(true)
	a.NonIPPDN = boolp(true)
	a.NIDDMechanism = intp(2) // undefined per TS 29.272 §7.3.205
	if _, err := s.createAPN(ctx, &APNCreateInput{Body: &a}); err == nil {
		t.Fatal("expected validation error for out-of-range nidd_mechanism, got nil")
	}
}

func TestCreateAPNRejectsSCEFFieldsWithSGiMechanism(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	a := baseAPN("iot3")
	a.CIoTEnabled = boolp(true)
	a.NonIPPDN = boolp(true)
	a.NIDDMechanism = intp(models.NIDDMechanismSGiBased)
	a.NIDDScefID = strp("scef.example.net")
	if _, err := s.createAPN(ctx, &APNCreateInput{Body: &a}); err == nil {
		t.Fatal("expected validation error for scef_id with SGi-based mechanism, got nil")
	}
}

func TestCreateAPNRejectsInvalidRDS(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	a := baseAPN("iot4")
	a.CIoTEnabled = boolp(true)
	a.NonIPPDN = boolp(true)
	a.NIDDRDS = intp(2) // undefined per TS 29.272 §7.3.222
	if _, err := s.createAPN(ctx, &APNCreateInput{Body: &a}); err == nil {
		t.Fatal("expected validation error for out-of-range nidd_rds, got nil")
	}
}

func TestCreateAPNRejectsZeroPreferredDataMode(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	a := baseAPN("iot5")
	a.CIoTEnabled = boolp(true)
	a.NIDDPreferredDataMode = intp(0) // spec: at least one bit must be set when present
	if _, err := s.createAPN(ctx, &APNCreateInput{Body: &a}); err == nil {
		t.Fatal("expected validation error for zero preferred_data_mode, got nil")
	}
}

func TestCreateAPNRejectsOutOfMaskPreferredDataMode(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	a := baseAPN("iot6")
	a.CIoTEnabled = boolp(true)
	a.NIDDPreferredDataMode = intp(4) // bit 2 undefined per §7.3.209
	if _, err := s.createAPN(ctx, &APNCreateInput{Body: &a}); err == nil {
		t.Fatal("expected validation error for out-of-mask preferred_data_mode, got nil")
	}
}

// TestCreateAPNAcceptsFullValidCIoTConfig is the positive-path counterpart:
// a fully-specified, spec-consistent CIoT configuration must be accepted.
func TestCreateAPNAcceptsFullValidCIoTConfig(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	a := baseAPN("iot7")
	a.CIoTEnabled = boolp(true)
	a.NonIPPDN = boolp(true)
	a.NIDDMechanism = intp(models.NIDDMechanismSCEFBased)
	a.NIDDScefID = strp("scef.example.net")
	a.NIDDScefRealm = strp("example.net")
	a.NIDDRDS = intp(models.RDSEnabled)
	a.NIDDPreferredDataMode = intp(models.PreferredDataModeUserPlane | models.PreferredDataModeControlPlane)
	if _, err := s.createAPN(ctx, &APNCreateInput{Body: &a}); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// TestCreateAPNAcceptsCIoTDisabledWithNoNIDDFields confirms an ordinary
// (non-CIoT) APN is unaffected by the new validation.
func TestCreateAPNAcceptsCIoTDisabledWithNoNIDDFields(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	a := baseAPN("plain1")
	if _, err := s.createAPN(ctx, &APNCreateInput{Body: &a}); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
