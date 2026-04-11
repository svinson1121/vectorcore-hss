package pcf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/models"
	pgstore "github.com/svinson1121/vectorcore-hss/internal/repository/postgres"
)

type callbackCapture struct {
	Path      string
	UserAgent string
	Body      json.RawMessage
}

func TestSMPolicyLifecycle(t *testing.T) {
	s, db := newTestPCFServer(t)
	seedPCFPolicyData(t, db)

	r := chi.NewRouter()
	s.mountRoutes(r)

	createBody := smPolicyContextData{
		Supi:            "imsi-001010000000001",
		Dnn:             "internet.5g.mnc001.mcc001.gprs",
		PduSessionID:    10,
		PduSessionType:  "IPV4",
		NotificationURI: "http://smf:7777/callback",
		ServingNetwork:  json.RawMessage(`{"mcc":"001","mnc":"01"}`),
		SliceInfo:       json.RawMessage(`{"sst":1}`),
	}
	raw, _ := json.Marshal(createBody)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/npcf-smpolicycontrol/v1/sm-policies", bytes.NewReader(raw))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status: got %d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "http://example.com/npcf-smpolicycontrol/v1/sm-policies/") {
		t.Fatalf("location: got %q", loc)
	}
	var decision smPolicyDecision
	if err := json.Unmarshal(rec.Body.Bytes(), &decision); err != nil {
		t.Fatalf("decode create decision: %v", err)
	}
	if len(decision.SessRules) == 0 {
		t.Fatal("expected session rules")
	}
	sessRule, ok := decision.SessRules["sess-default"]
	if !ok {
		t.Fatalf("expected session rule sess-default, got %#v", decision.SessRules)
	}
	if sessRule.SessRuleID != "sess-default" {
		t.Fatalf("unexpected session rule id: %q", sessRule.SessRuleID)
	}
	if len(decision.PccRules) == 0 {
		t.Fatal("expected pcc rules")
	}
	if len(decision.ChgDecs) == 0 {
		t.Fatal("expected charging decisions")
	}
	if len(decision.QosDecs) == 0 {
		t.Fatal("expected qos decisions")
	}
	rule, ok := decision.PccRules["internet-default"]
	if !ok {
		t.Fatalf("expected pcc rule internet-default, got %#v", decision.PccRules)
	}
	if rule.PccRuleID != "internet-default" {
		t.Fatalf("unexpected pcc rule id: %q", rule.PccRuleID)
	}
	if len(rule.FlowInfos) != 1 {
		t.Fatalf("unexpected flow infos: %#v", rule.FlowInfos)
	}
	if rule.FlowInfos[0].FlowDirection != "DOWNLINK" {
		t.Fatalf("unexpected flow direction: %#v", rule.FlowInfos[0])
	}
	if len(rule.RefChgData) != 1 || rule.RefChgData[0] != "chg-internet-default" {
		t.Fatalf("unexpected charging references: %#v", rule.RefChgData)
	}
	qos, ok := decision.QosDecs["qos-internet-default"]
	if !ok {
		t.Fatalf("expected qos decision qos-internet-default, got %#v", decision.QosDecs)
	}
	if qos.QosID != "qos-internet-default" {
		t.Fatalf("unexpected qos decision id: %q", qos.QosID)
	}
	if qos.Arp == nil || qos.Arp.PreemptCap == "" || qos.Arp.PreemptVuln == "" {
		t.Fatalf("expected qos arp preemption fields: %#v", qos.Arp)
	}
	chg, ok := decision.ChgDecs["chg-internet-default"]
	if !ok {
		t.Fatalf("expected charging decision chg-internet-default, got %#v", decision.ChgDecs)
	}
	if chg.ChgID != "chg-internet-default" {
		t.Fatalf("unexpected charging decision id: %q", chg.ChgID)
	}
	if chg.RatingGroup == nil || *chg.RatingGroup != 1000 {
		t.Fatalf("unexpected rating group: %#v", chg.RatingGroup)
	}
	if chg.ChargingCharacteristics != "0800" {
		t.Fatalf("unexpected charging characteristics: %q", chg.ChargingCharacteristics)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, loc, nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status: got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, loc+"/update", bytes.NewReader([]byte(`{
		"subsSessAmbr":{"uplink":"777 Kbps","downlink":"888 Kbps"},
		"subsDefQos":{"5qi":7,"arp":{"priorityLevel":2}},
		"notificationUri":"http://smf:7777/new-callback",
		"suppFeat":"1"
	}`)))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status: got %d body=%s", rec.Code, rec.Body.String())
	}
	var updated smPolicyDecision
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update decision: %v", err)
	}
	sess, ok := updated.SessRules["sess-default"]
	if !ok || sess.AuthSessAmbr == nil || sess.DefQos == nil {
		t.Fatalf("missing updated session rule: %#v", updated.SessRules)
	}
	if sess.AuthSessAmbr.Uplink != "777 Kbps" || sess.AuthSessAmbr.Downlink != "888 Kbps" {
		t.Fatalf("unexpected updated AMBR: %#v", sess.AuthSessAmbr)
	}
	if sess.DefQos.Var5qi == nil || *sess.DefQos.Var5qi != 7 {
		t.Fatalf("unexpected updated 5qi: %#v", sess.DefQos.Var5qi)
	}
	if sess.DefQos.Arp == nil || sess.DefQos.Arp.PriorityLevel != 2 {
		t.Fatalf("unexpected updated arp: %#v", sess.DefQos.Arp)
	}
	if sess.DefQos.Arp.PreemptCap == "" || sess.DefQos.Arp.PreemptVuln == "" {
		t.Fatalf("expected updated arp preemption fields: %#v", sess.DefQos.Arp)
	}
	if updated.SuppFeat != "1" {
		t.Fatalf("unexpected updated suppFeat: %q", updated.SuppFeat)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, loc+"/delete", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status: got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, loc, nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get-after-delete status: got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSMPolicyUpdateAndDeleteNotifyCallback(t *testing.T) {
	s, db := newTestPCFServer(t)
	seedPCFPolicyData(t, db)

	callbacks := make(chan callbackCapture, 2)
	cbHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read callback: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		callbacks <- callbackCapture{Path: r.URL.Path, UserAgent: r.Header.Get("User-Agent"), Body: append(json.RawMessage(nil), body...)}
		w.WriteHeader(http.StatusNoContent)
	})
	cbServer := httptest.NewUnstartedServer(h2c.NewHandler(cbHandler, &http2.Server{}))
	cbServer.EnableHTTP2 = true
	cbServer.Start()
	defer cbServer.Close()

	r := chi.NewRouter()
	s.mountRoutes(r)

	createBody := smPolicyContextData{
		Supi:            "imsi-001010000000001",
		Dnn:             "internet",
		PduSessionID:    10,
		NotificationURI: cbServer.URL + "/sm-policy-notify",
	}
	raw, _ := json.Marshal(createBody)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/npcf-smpolicycontrol/v1/sm-policies", bytes.NewReader(raw))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status: got %d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, loc+"/update", bytes.NewReader([]byte(`{"suppFeat":"abc"}`)))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status: got %d body=%s", rec.Code, rec.Body.String())
	}
	updateCB := mustReceiveCallback(t, callbacks)
	if updateCB.Path != "/sm-policy-notify/update" {
		t.Fatalf("unexpected callback path: %q", updateCB.Path)
	}
	if !strings.HasPrefix(updateCB.UserAgent, "PCF-") {
		t.Fatalf("unexpected callback user-agent: %q", updateCB.UserAgent)
	}
	var updateBody smPolicyNotification
	if err := json.Unmarshal(updateCB.Body, &updateBody); err != nil {
		t.Fatalf("decode update callback body: %v body=%s", err, string(updateCB.Body))
	}
	if updateBody.SmPolicyDecision == nil {
		t.Fatalf("expected update callback decision: %#v", updateBody)
	}
	if updateBody.SmPolicyDecision.SuppFeat != "abc" {
		t.Fatalf("unexpected callback suppFeat: %q", updateBody.SmPolicyDecision.SuppFeat)
	}
	if !strings.Contains(updateBody.ResourceURI, "/npcf-smpolicycontrol/v1/sm-policies/") {
		t.Fatalf("unexpected callback resource URI: %q", updateBody.ResourceURI)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, loc+"/delete", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status: got %d body=%s", rec.Code, rec.Body.String())
	}
	deleteCB := mustReceiveCallback(t, callbacks)
	if deleteCB.Path != "/sm-policy-notify/terminate" {
		t.Fatalf("unexpected delete callback path: %q", deleteCB.Path)
	}
	var deleteBody smPolicyTerminationNotification
	if err := json.Unmarshal(deleteCB.Body, &deleteBody); err != nil {
		t.Fatalf("decode delete callback body: %v body=%s", err, string(deleteCB.Body))
	}
	if deleteBody.Cause != "UNSPECIFIED" {
		t.Fatalf("unexpected termination cause: %q", deleteBody.Cause)
	}
	if !strings.Contains(deleteBody.ResourceURI, "/npcf-smpolicycontrol/v1/sm-policies/") {
		t.Fatalf("unexpected delete resource URI: %q", deleteBody.ResourceURI)
	}
}

func TestSMPolicyCreateRejectsMissingMandatoryFields(t *testing.T) {
	s, db := newTestPCFServer(t)
	seedPCFPolicyData(t, db)

	r := chi.NewRouter()
	s.mountRoutes(r)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/npcf-smpolicycontrol/v1/sm-policies", bytes.NewReader([]byte(`{"supi":"imsi-001010000000001"}`)))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSMPolicyUpdateRejectsInvalidJSON(t *testing.T) {
	s, db := newTestPCFServer(t)
	seedPCFPolicyData(t, db)

	r := chi.NewRouter()
	s.mountRoutes(r)

	createBody := smPolicyContextData{
		Supi:         "imsi-001010000000001",
		Dnn:          "internet",
		PduSessionID: 10,
	}
	raw, _ := json.Marshal(createBody)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/npcf-smpolicycontrol/v1/sm-policies", bytes.NewReader(raw))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status: got %d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, loc+"/update", bytes.NewReader([]byte(`{`)))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
}

func newTestPCFServer(t *testing.T) (*Server, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(models.AllModels()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store := pgstore.New(db, 2)
	s := New(config.PCFConfig{}, store, zap.NewNop())
	return s, db
}

func seedPCFPolicyData(t *testing.T, db *gorm.DB) {
	t.Helper()
	rule := models.ChargingRule{
		RuleName:    "internet-default",
		QCI:         9,
		ARPPriority: 4,
		MBRDown:     102400,
		MBRUp:       51200,
		GBRDown:     0,
		GBRUp:       0,
		TFTGroupID:  intPtr(88),
		Precedence:  intPtr(100),
		RatingGroup: intPtr(1000),
	}
	mustCreatePCF(t, db, &rule)
	mustCreatePCF(t, db, &models.TFT{TFTGroupID: 88, TFTString: "permit out ip from any to any", Direction: 1})
	apn := models.APN{
		APN:              "internet",
		APNAMBRDown:      204800,
		APNAMBRUp:        102400,
		QCI:              9,
		ARPPriority:      4,
		ChargingRuleList: strPtr(strconv.Itoa(rule.ChargingRuleID)),
	}
	mustCreatePCF(t, db, &apn)
	mustCreatePCF(t, db, &models.Subscriber{
		IMSI:       "001010000000001",
		AUCID:      1,
		DefaultAPN: apn.APNID,
		APNList:    strconv.Itoa(apn.APNID),
	})
}

func mustCreatePCF(t *testing.T, db *gorm.DB, model interface{}) {
	t.Helper()
	if err := db.Create(model).Error; err != nil {
		t.Fatalf("create %T: %v", model, err)
	}
}

func mustReceiveCallback(t *testing.T, callbacks <-chan callbackCapture) callbackCapture {
	t.Helper()
	select {
	case cb := <-callbacks:
		return cb
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for callback")
		return callbackCapture{}
	}
}

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }
