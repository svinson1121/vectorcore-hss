package udm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/models"
	pgstore "github.com/svinson1121/vectorcore-hss/internal/repository/postgres"
	"github.com/svinson1121/vectorcore-hss/internal/taccache"
)

func TestHandleSMDataMatchesNormalizedDNN(t *testing.T) {
	s, _ := newTestUDMServer(t)

	r := chi.NewRouter()
	r.Get("/nudm-sdm/v1/{supi}/sm-data", s.handleSMData)

	req := httptest.NewRequest(http.MethodGet, "/nudm-sdm/v1/imsi-001010000000001/sm-data?dnn=internet.5g.mnc001.mcc001.gprs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}

	var items []smDataItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if len(items) != 1 {
		t.Fatalf("expected one smData item, got %#v", items)
	}
	cfg, ok := items[0].DNNConfigs["internet.5g.mnc001.mcc001.gprs"]
	if !ok {
		t.Fatalf("expected requested DNN config key, got %#v", items[0].DNNConfigs)
	}
	if cfg.SessionAMBR == nil || cfg.SessionAMBR.Uplink == "" || cfg.SessionAMBR.Downlink == "" {
		t.Fatalf("expected session AMBR in DNN config, got %#v", cfg)
	}
	if cfg.FiveGQosProfile == nil {
		t.Fatalf("expected 5gQosProfile in DNN config, got %#v", cfg)
	}
	if cfg.FiveGQosProfile.ARP.PreemptCap == "" || cfg.FiveGQosProfile.ARP.PreemptVuln == "" {
		t.Fatalf("expected ARP preemption flags in DNN config, got %#v", cfg.FiveGQosProfile)
	}
}

func TestParseNSSAIFallsBackWhenConfiguredSliceIsInvalid(t *testing.T) {
	slices := parseNSSAI(`[{"sst":0}]`)
	if len(slices) != 1 {
		t.Fatalf("expected one fallback slice, got %#v", slices)
	}
	if slices[0].SST != 1 {
		t.Fatalf("expected fallback SST 1, got %#v", slices)
	}
}

func TestHandleSMFSelectDataUsesOpen5GSSNSSAIMapKeys(t *testing.T) {
	s, _ := newTestUDMServer(t)

	r := chi.NewRouter()
	r.Get("/nudm-sdm/v1/{supi}/smf-select-data", s.handleSMFSelectData)

	req := httptest.NewRequest(http.MethodGet, "/nudm-sdm/v1/imsi-001010000000001/smf-select-data", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}

	var body smfSelectData
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}

	if _, ok := body.SubscribedSnssaiInfos["1"]; !ok {
		t.Fatalf("expected Open5GS key format, got %#v", body.SubscribedSnssaiInfos)
	}
	for key := range body.SubscribedSnssaiInfos {
		if strings.Contains(key, "{") {
			t.Fatalf("unexpected JSON-formatted S-NSSAI key %q", key)
		}
	}
}

func TestHandleAMFRegistrationPutStoresPEIHistory(t *testing.T) {
	s, db := newTestUDMServer(t)
	s.WithEIR(2, true)
	tac := taccache.New()
	tac.Set("35617506", "Apple", "iPhone 15")
	s.WithTAC(tac)

	r := chi.NewRouter()
	r.Put("/nudm-uecm/v1/{supi}/registrations/amf-3gpp-access", s.handleAMFRegistrationPut)

	body := `{"amfInstanceId":"amf-1","pei":"imeisv-3561750601234567"}`
	req := httptest.NewRequest(http.MethodPut, "/nudm-uecm/v1/imsi-001010000000001/registrations/amf-3gpp-access", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}

	var hist models.IMSIIMEIHistory
	if err := db.Where("imsi = ?", "001010000000001").First(&hist).Error; err != nil {
		t.Fatalf("expected eir history row: %v", err)
	}
	if hist.IMEI != "3561750601234567" {
		t.Fatalf("unexpected imei %q", hist.IMEI)
	}
	if hist.Make != "Apple" || hist.Model != "iPhone 15" {
		t.Fatalf("unexpected device info: %+v", hist)
	}
	if hist.MatchResponseCode != 2 {
		t.Fatalf("unexpected match response code %d", hist.MatchResponseCode)
	}
}

type testUDMStore struct {
	*pgstore.Store
	db *gorm.DB
}

func newTestUDMServer(t *testing.T) (*Server, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(models.AllModels()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := &testUDMStore{Store: pgstore.New(db, 2), db: db}
	seedUDMSubscriberData(t, db)
	s := New(config.UDMConfig{}, store, zap.NewNop())
	return s, db
}

func seedUDMSubscriberData(t *testing.T, db *gorm.DB) {
	t.Helper()
	apn := models.APN{
		APN:         "internet",
		APNAMBRDown: 204800,
		APNAMBRUp:   102400,
		QCI:         9,
		ARPPriority: 4,
	}
	mustCreateUDM(t, db, &apn)
	mustCreateUDM(t, db, &models.Subscriber{
		IMSI:       "001010000000001",
		AUCID:      1,
		DefaultAPN: apn.APNID,
		APNList:    strconv.Itoa(apn.APNID),
	})
}

func mustCreateUDM(t *testing.T, db *gorm.DB, model interface{}) {
	t.Helper()
	if err := db.Create(model).Error; err != nil {
		t.Fatalf("create %T: %v", model, err)
	}
}
