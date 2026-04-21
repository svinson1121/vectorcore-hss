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
