package api

import (
	"context"
	"testing"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

type idrRecorder struct {
	imsis []string
}

func (r *idrRecorder) SendIDRByIMSI(_ context.Context, imsi string) error {
	r.imsis = append(r.imsis, imsi)
	return nil
}

func TestUpdateSubscriberSendsIDROnAccessRestrictionChange(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	idr := &idrRecorder{}
	s.WithIDR(idr)

	enabled := true
	oldARD := uint32(64)
	newARD := uint32(320)
	sub := models.Subscriber{
		IMSI:                  "001010000000101",
		Enabled:               &enabled,
		AUCID:                 1,
		DefaultAPN:            1,
		APNList:               "1",
		AccessRestrictionData: &oldARD,
	}
	mustCreate(t, s.db, &sub)

	updated := sub
	updated.AccessRestrictionData = &newARD
	if _, err := s.updateSubscriber(ctx, &SubscriberUpdateInput{ID: sub.SubscriberID, Body: &updated}); err != nil {
		t.Fatalf("update subscriber: %v", err)
	}

	if len(idr.imsis) != 1 || idr.imsis[0] != sub.IMSI {
		t.Fatalf("IDR calls = %#v, want [%q]", idr.imsis, sub.IMSI)
	}
}

func TestUpdateSubscriberSkipsIDRWhenAccessRestrictionUnchanged(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	idr := &idrRecorder{}
	s.WithIDR(idr)

	enabled := true
	ard := uint32(64)
	sub := models.Subscriber{
		IMSI:                  "001010000000102",
		Enabled:               &enabled,
		AUCID:                 1,
		DefaultAPN:            1,
		APNList:               "1",
		AccessRestrictionData: &ard,
	}
	mustCreate(t, s.db, &sub)

	updated := sub
	if _, err := s.updateSubscriber(ctx, &SubscriberUpdateInput{ID: sub.SubscriberID, Body: &updated}); err != nil {
		t.Fatalf("update subscriber: %v", err)
	}

	if len(idr.imsis) != 0 {
		t.Fatalf("unexpected IDR calls: %#v", idr.imsis)
	}
}

func TestUpdateSubscriberSkipsIDRWhenDisabled(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	idr := &idrRecorder{}
	s.WithIDR(idr)

	enabled := true
	oldARD := uint32(64)
	newARD := uint32(320)
	sub := models.Subscriber{
		IMSI:                  "001010000000103",
		Enabled:               &enabled,
		AUCID:                 1,
		DefaultAPN:            1,
		APNList:               "1",
		AccessRestrictionData: &oldARD,
	}
	mustCreate(t, s.db, &sub)

	disabled := false
	updated := sub
	updated.Enabled = &disabled
	updated.AccessRestrictionData = &newARD
	if _, err := s.updateSubscriber(ctx, &SubscriberUpdateInput{ID: sub.SubscriberID, Body: &updated}); err != nil {
		t.Fatalf("update subscriber: %v", err)
	}

	if len(idr.imsis) != 0 {
		t.Fatalf("unexpected IDR calls: %#v", idr.imsis)
	}
}

// TestCreateSubscriberRejectsWBEUTRANLTEMConflict verifies the TS 29.272
// §7.3.31 NOTE 2 rule: bit 4 (WB-E-UTRAN Not Allowed) cannot be combined with
// bit 11 (LTE-M Not Allowed) or bit 12 (WB-E-UTRAN Except LTE-M Not Allowed).
func TestCreateSubscriberRejectsWBEUTRANLTEMConflict(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	cases := []struct {
		name string
		imsi string
		ard  uint32
	}{
		{"bit4_plus_bit11", "001010000000230", models.ARDWBEUTRANNotAllowed | models.ARDLTEMNotAllowed},
		{"bit4_plus_bit12", "001010000000231", models.ARDWBEUTRANNotAllowed | models.ARDWBEUTRANExceptLTEMNotAllowed},
		{"bit4_plus_both", "001010000000232", models.ARDWBEUTRANNotAllowed | models.ARDLTEMNotAllowed | models.ARDWBEUTRANExceptLTEMNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enabled := true
			ard := tc.ard
			sub := models.Subscriber{
				IMSI:                  tc.imsi,
				Enabled:               &enabled,
				AUCID:                 1,
				DefaultAPN:            1,
				APNList:               "1",
				AccessRestrictionData: &ard,
			}
			if _, err := s.createSubscriber(ctx, &SubscriberCreateInput{Body: &sub}); err == nil {
				t.Fatalf("expected validation error for ARD 0x%08x, got nil", tc.ard)
			}
		})
	}
}

// TestCreateSubscriberAllowsLTEMWithoutWBEUTRANConflict confirms the LTE-M
// bits are accepted on their own (bit 4 clear) — only the combination with
// bit 4 is invalid.
func TestCreateSubscriberAllowsLTEMWithoutWBEUTRANConflict(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	enabled := true
	ard := models.ARDLTEMNotAllowed | models.ARDWBEUTRANExceptLTEMNotAllowed
	sub := models.Subscriber{
		IMSI:                  "001010000000233",
		Enabled:               &enabled,
		AUCID:                 1,
		DefaultAPN:            1,
		APNList:               "1",
		AccessRestrictionData: &ard,
	}
	if _, err := s.createSubscriber(ctx, &SubscriberCreateInput{Body: &sub}); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// TestAccessRestrictionAudit_FlagsReinterpretedBits verifies the audit
// endpoint lists only subscribers whose stored mask includes a bit that was
// mislabeled in the pre-fix UI (models.ARDReinterpretedBitsMask), not
// subscribers with only unrelated bits set.
func TestAccessRestrictionAudit_FlagsReinterpretedBits(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	enabled := true
	flaggedARD := models.ARDNBIoTNotAllowed // 0x40, previously mislabeled
	cleanARD := models.ARDUTRANNotAllowed   // 0x1, always correctly labeled
	flagged := models.Subscriber{IMSI: "001010000000234", Enabled: &enabled, AUCID: 1, DefaultAPN: 1, APNList: "1", AccessRestrictionData: &flaggedARD}
	clean := models.Subscriber{IMSI: "001010000000235", Enabled: &enabled, AUCID: 1, DefaultAPN: 1, APNList: "1", AccessRestrictionData: &cleanARD}
	mustCreate(t, s.db, &flagged)
	mustCreate(t, s.db, &clean)

	out, err := s.getAccessRestrictionAudit(ctx, &struct{}{})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	found := false
	for _, item := range out.Body {
		if item.IMSI == clean.IMSI {
			t.Fatalf("audit incorrectly flagged subscriber %s with only unrelated bit 0x1 set", clean.IMSI)
		}
		if item.IMSI == flagged.IMSI {
			found = true
			if item.AccessRestrictionData != flaggedARD {
				t.Fatalf("audit access_restriction_data = 0x%08x, want 0x%08x", item.AccessRestrictionData, flaggedARD)
			}
		}
	}
	if !found {
		t.Fatalf("audit did not flag subscriber %s with bit 0x40 set", flagged.IMSI)
	}
}

func TestCreateSubscriberDefaultsToServiceGranted(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	enabled := true
	sub := models.Subscriber{
		IMSI:       "001010000000200",
		Enabled:    &enabled,
		AUCID:      1,
		DefaultAPN: 1,
		APNList:    "1",
	}
	out, err := s.createSubscriber(ctx, &SubscriberCreateInput{Body: &sub})
	if err != nil {
		t.Fatalf("create subscriber: %v", err)
	}
	if out.Body.SubscriberStatus != models.SubscriberStatusServiceGranted {
		t.Fatalf("subscriber_status = %d, want SERVICE_GRANTED (0)", out.Body.SubscriberStatus)
	}
	if out.Body.OperatorDeterminedBarring != 0 {
		t.Fatalf("operator_determined_barring = %d, want 0", out.Body.OperatorDeterminedBarring)
	}
}

func TestCreateSubscriberRejectsInvalidSubscriberStatus(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	enabled := true
	sub := models.Subscriber{
		IMSI:             "001010000000201",
		Enabled:          &enabled,
		AUCID:            1,
		DefaultAPN:       1,
		APNList:          "1",
		SubscriberStatus: 2, // undefined per TS 29.272 §7.3.29
	}
	if _, err := s.createSubscriber(ctx, &SubscriberCreateInput{Body: &sub}); err == nil {
		t.Fatal("expected validation error for undefined subscriber_status, got nil")
	}
}

func TestCreateSubscriberRejectsReservedODBBits(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	enabled := true
	sub := models.Subscriber{
		IMSI:                      "001010000000202",
		Enabled:                   &enabled,
		AUCID:                     1,
		DefaultAPN:                1,
		APNList:                   "1",
		SubscriberStatus:          models.SubscriberStatusOperatorDeterminedBarring,
		OperatorDeterminedBarring: 1 << 9, // reserved bit, not defined in §7.3.30
	}
	if _, err := s.createSubscriber(ctx, &SubscriberCreateInput{Body: &sub}); err == nil {
		t.Fatal("expected validation error for reserved ODB bit, got nil")
	}
}

// TestCreateSubscriberODBBitmaskCombinesCorrectly verifies bits 0+3 encode to
// 0x9 per the handoff's worked example, and round-trips through create/read.
func TestCreateSubscriberODBBitmaskCombinesCorrectly(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	enabled := true
	sub := models.Subscriber{
		IMSI:             "001010000000203",
		Enabled:          &enabled,
		AUCID:            1,
		DefaultAPN:       1,
		APNList:          "1",
		SubscriberStatus: models.SubscriberStatusOperatorDeterminedBarring,
		OperatorDeterminedBarring: models.ODBAllPacketOrientedServicesBarred |
			models.ODBBarringOfAllOutgoingCalls,
	}
	out, err := s.createSubscriber(ctx, &SubscriberCreateInput{Body: &sub})
	if err != nil {
		t.Fatalf("create subscriber: %v", err)
	}
	if out.Body.OperatorDeterminedBarring != 0x00000009 {
		t.Fatalf("operator_determined_barring = 0x%08x, want 0x00000009", out.Body.OperatorDeterminedBarring)
	}
}

func TestUpdateSubscriberClearsODBWhenServiceGranted(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	enabled := true
	sub := models.Subscriber{
		IMSI:                      "001010000000204",
		Enabled:                   &enabled,
		AUCID:                     1,
		DefaultAPN:                1,
		APNList:                   "1",
		SubscriberStatus:          models.SubscriberStatusOperatorDeterminedBarring,
		OperatorDeterminedBarring: models.ODBAllPacketOrientedServicesBarred,
	}
	mustCreate(t, s.db, &sub)

	updated := sub
	updated.SubscriberStatus = models.SubscriberStatusServiceGranted
	// Simulate a stale/leftover mask still present in the request body — the
	// server must clear it rather than trust the client.
	updated.OperatorDeterminedBarring = models.ODBAllPacketOrientedServicesBarred
	out, err := s.updateSubscriber(ctx, &SubscriberUpdateInput{ID: sub.SubscriberID, Body: &updated})
	if err != nil {
		t.Fatalf("update subscriber: %v", err)
	}
	if out.Body.OperatorDeterminedBarring != 0 {
		t.Fatalf("operator_determined_barring = %d, want 0 after reverting to SERVICE_GRANTED", out.Body.OperatorDeterminedBarring)
	}
}

func TestUpdateSubscriberSendsIDROnSubscriberStatusChange(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	idr := &idrRecorder{}
	s.WithIDR(idr)

	enabled := true
	sub := models.Subscriber{
		IMSI:       "001010000000205",
		Enabled:    &enabled,
		AUCID:      1,
		DefaultAPN: 1,
		APNList:    "1",
	}
	mustCreate(t, s.db, &sub)

	updated := sub
	updated.SubscriberStatus = models.SubscriberStatusOperatorDeterminedBarring
	updated.OperatorDeterminedBarring = models.ODBBarringOfAllOutgoingCalls
	if _, err := s.updateSubscriber(ctx, &SubscriberUpdateInput{ID: sub.SubscriberID, Body: &updated}); err != nil {
		t.Fatalf("update subscriber: %v", err)
	}

	if len(idr.imsis) != 1 || idr.imsis[0] != sub.IMSI {
		t.Fatalf("IDR calls = %#v, want [%q]", idr.imsis, sub.IMSI)
	}
}
