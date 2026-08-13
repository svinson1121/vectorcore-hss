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
