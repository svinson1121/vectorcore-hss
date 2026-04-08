package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

func TestAuditUpdatePreservesZeroValuesInAfterSnapshot(t *testing.T) {
	s := newTestServer(t)
	ctx := WithAudit(context.Background())

	sub := models.Subscriber{
		IMSI:       "001010000000099",
		AUCID:      1,
		DefaultAPN: 1,
		APNList:    "1,2",
		NAM:        0,
	}
	if err := s.db.WithContext(ctx).Create(&sub).Error; err != nil {
		t.Fatalf("create subscriber: %v", err)
	}

	sub.APNList = "1,2,4"
	if err := s.db.WithContext(ctx).Save(&sub).Error; err != nil {
		t.Fatalf("update subscriber: %v", err)
	}

	var entry models.OperationLog
	if err := s.db.Where("table_name = ? AND operation = ?", "subscriber", "update").Order("id DESC").First(&entry).Error; err != nil {
		t.Fatalf("load operation log: %v", err)
	}
	if entry.Changes == nil {
		t.Fatalf("expected operation log changes")
	}

	var cr changeRecord
	if err := json.Unmarshal([]byte(*entry.Changes), &cr); err != nil {
		t.Fatalf("unmarshal changes: %v", err)
	}

	if got := cr.Before["nam"]; got != int64(0) && got != float64(0) && got != 0 {
		t.Fatalf("unexpected before nam: %#v", got)
	}
	if got := cr.After["nam"]; got != int64(0) && got != float64(0) && got != 0 {
		t.Fatalf("unexpected after nam: %#v", got)
	}
	if got := cr.After["apn_list"]; got != "1,2,4" {
		t.Fatalf("unexpected after apn_list: %#v", got)
	}
}
