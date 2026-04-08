package udm

// sdm.go — Nudm_SubscriberDataManagement handlers.
//
// GET /nudm-sdm/v{1,2}/{supi}/am-data          → AMF subscription data
// GET /nudm-sdm/v{1,2}/{supi}/sm-data          → SMF/DNN data
// GET /nudm-sdm/v{1,2}/{supi}/smf-select-data  → SMF selection info
// GET /nudm-sdm/v{1,2}/{supi}/nssai            → Allowed NSSAI
// POST/DELETE /nudm-sdm/v{1,2}/{supi}/sdm-subscriptions → change notifications (stub)
//
// AMBR format: Open5GS expects strings like "1 Gbps", "100 Mbps", "512 Kbps".
// Our DB stores raw kbps integers.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

// defaultNSSAI is returned when a subscriber has no NSSAI configured.
const defaultNSSAI = `[{"sst":1}]`

// ── AM-DATA ──────────────────────────────────────────────────────────────────

type amData struct {
	GPSIS              []string       `json:"gpsis,omitempty"`
	SubscribedUEAMBR   *ambrData      `json:"subscribedUeAmbr,omitempty"`
	NSSAI              *nssaiData     `json:"nssai,omitempty"`
	RatRestrictions    []string       `json:"ratRestrictions"`
	ForbiddenAreas     []interface{}  `json:"forbiddenAreas"`
	ServiceAreaRestriction interface{} `json:"serviceAreaRestriction"`
}

type ambrData struct {
	Uplink   string `json:"uplink"`
	Downlink string `json:"downlink"`
}

type nssaiData struct {
	DefaultSingleNssais []snssai `json:"defaultSingleNssais"`
	SingleNssais        []snssai `json:"singleNssais"`
}

type snssai struct {
	SST int    `json:"sst"`
	SD  string `json:"sd,omitempty"`
}

func (s *Server) handleAMData(w http.ResponseWriter, r *http.Request) {
	imsi, err := resolveIMSI(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_supi")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	sub, err := s.store.GetSubscriberByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		jsonError(w, http.StatusNotFound, "user_not_found")
		return
	}
	if err != nil {
		s.log.Error("udm: am-data db error", zap.String("imsi", imsi), zap.Error(err))
		jsonError(w, http.StatusInternalServerError, "db_error")
		return
	}

	nssaiJSON := defaultNSSAI
	if sub.NSSAI != nil && *sub.NSSAI != "" {
		nssaiJSON = *sub.NSSAI
	}
	slices := parseNSSAI(nssaiJSON)

	resp := amData{
		SubscribedUEAMBR: &ambrData{
			Uplink:   kbpsToString(sub.UEAMBRUp),
			Downlink: kbpsToString(sub.UEAMBRDown),
		},
		NSSAI: &nssaiData{
			DefaultSingleNssais: slices,
			SingleNssais:        slices,
		},
		RatRestrictions:        []string{},
		ForbiddenAreas:         []interface{}{},
		ServiceAreaRestriction: struct{}{},
	}
	if sub.MSISDN != nil && *sub.MSISDN != "" {
		resp.GPSIS = []string{"msisdn-" + *sub.MSISDN}
	}

	jsonOK(w, resp)
}

// ── SM-DATA ──────────────────────────────────────────────────────────────────

type smDataItem struct {
	SingleNssai snssai       `json:"singleNssai"`
	DNNConfigs  map[string]*dnnConfig `json:"dnnConfigurations"`
}

type dnnConfig struct {
	PDUSessionTypes  pduSessionTypes  `json:"pduSessionTypes"`
	SSCModes         sscModes         `json:"sscModes"`
	SessionAMBR      *ambrData        `json:"sessionAmbr,omitempty"`
	FiveQI           int              `json:"5gQosProfile,omitempty"`
}

type pduSessionTypes struct {
	DefaultSessionType  string   `json:"defaultSessionType"`
	AllowedSessionTypes []string `json:"allowedSessionTypes"`
}

type sscModes struct {
	DefaultSSCMode  string   `json:"defaultSscMode"`
	AllowedSSCModes []string `json:"allowedSscModes"`
}

func (s *Server) handleSMData(w http.ResponseWriter, r *http.Request) {
	imsi, err := resolveIMSI(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_supi")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	sub, err := s.store.GetSubscriberByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		jsonError(w, http.StatusNotFound, "user_not_found")
		return
	}
	if err != nil {
		s.log.Error("udm: sm-data db error", zap.String("imsi", imsi), zap.Error(err))
		jsonError(w, http.StatusInternalServerError, "db_error")
		return
	}

	nssaiJSON := defaultNSSAI
	if sub.NSSAI != nil && *sub.NSSAI != "" {
		nssaiJSON = *sub.NSSAI
	}
	slices := parseNSSAI(nssaiJSON)

	// Build one smDataItem per slice; each item contains all the subscriber's DNNs.
	// Query-param filtering by single-nssai / dnn is handled below.
	filterSST := 0
	filterDNN := r.URL.Query().Get("dnn")
	if v := r.URL.Query().Get("single-nssai"); v != "" {
		var sn snssai
		if json.Unmarshal([]byte(v), &sn) == nil {
			filterSST = sn.SST
		}
	}

	// Fetch APNs assigned to this subscriber.
	apnIDs := parseAPNList(sub.APNList)
	dnnCfgs := make(map[string]*dnnConfig)
	for _, id := range apnIDs {
		apn, err := s.store.GetAPNByID(ctx, id)
		if err != nil {
			continue
		}
		if filterDNN != "" && apn.APN != filterDNN {
			continue
		}
		dnnCfgs[apn.APN] = &dnnConfig{
			PDUSessionTypes: pduSessionTypes{
				DefaultSessionType:  "IPV4",
				AllowedSessionTypes: []string{"IPV4"},
			},
			SSCModes: sscModes{
				DefaultSSCMode:  "SSC_MODE_1",
				AllowedSSCModes: []string{"SSC_MODE_1", "SSC_MODE_2"},
			},
			SessionAMBR: &ambrData{
				Uplink:   kbpsToString(apn.APNAMBRUp),
				Downlink: kbpsToString(apn.APNAMBRDown),
			},
		}
	}

	var result []smDataItem
	for _, sl := range slices {
		if filterSST != 0 && sl.SST != filterSST {
			continue
		}
		result = append(result, smDataItem{
			SingleNssai: sl,
			DNNConfigs:  dnnCfgs,
		})
	}
	if result == nil {
		result = []smDataItem{}
	}

	jsonOK(w, result)
}

// ── SMF-SELECT-DATA ──────────────────────────────────────────────────────────

type smfSelectData struct {
	SubscribedSnssaiInfos map[string]snssaiInfo `json:"subscribedSnssaiInfos,omitempty"`
}

type snssaiInfo struct {
	DNNInfos []dnnInfo `json:"dnnInfos"`
}

type dnnInfo struct {
	DNN string `json:"dnn"`
}

func (s *Server) handleSMFSelectData(w http.ResponseWriter, r *http.Request) {
	imsi, err := resolveIMSI(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_supi")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	sub, err := s.store.GetSubscriberByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		jsonError(w, http.StatusNotFound, "user_not_found")
		return
	}
	if err != nil {
		s.log.Error("udm: smf-select db error", zap.String("imsi", imsi), zap.Error(err))
		jsonError(w, http.StatusInternalServerError, "db_error")
		return
	}

	apnIDs := parseAPNList(sub.APNList)
	var dnns []dnnInfo
	for _, id := range apnIDs {
		apn, err := s.store.GetAPNByID(ctx, id)
		if err == nil {
			dnns = append(dnns, dnnInfo{DNN: apn.APN})
		}
	}
	if dnns == nil {
		dnns = []dnnInfo{}
	}

	nssaiJSON := defaultNSSAI
	if sub.NSSAI != nil && *sub.NSSAI != "" {
		nssaiJSON = *sub.NSSAI
	}
	slices := parseNSSAI(nssaiJSON)

	snssaiInfos := make(map[string]snssaiInfo)
	for _, sl := range slices {
		key := fmt.Sprintf(`{"sst":%d}`, sl.SST)
		if sl.SD != "" {
			key = fmt.Sprintf(`{"sst":%d,"sd":"%s"}`, sl.SST, sl.SD)
		}
		snssaiInfos[key] = snssaiInfo{DNNInfos: dnns}
	}

	jsonOK(w, smfSelectData{SubscribedSnssaiInfos: snssaiInfos})
}

// ── NSSAI ────────────────────────────────────────────────────────────────────

func (s *Server) handleNSSAI(w http.ResponseWriter, r *http.Request) {
	imsi, err := resolveIMSI(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_supi")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	sub, err := s.store.GetSubscriberByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		jsonError(w, http.StatusNotFound, "user_not_found")
		return
	}
	if err != nil {
		s.log.Error("udm: nssai db error", zap.String("imsi", imsi), zap.Error(err))
		jsonError(w, http.StatusInternalServerError, "db_error")
		return
	}

	nssaiJSON := defaultNSSAI
	if sub.NSSAI != nil && *sub.NSSAI != "" {
		nssaiJSON = *sub.NSSAI
	}
	slices := parseNSSAI(nssaiJSON)

	jsonOK(w, nssaiData{
		DefaultSingleNssais: slices,
		SingleNssais:        slices,
	})
}

// ── SDM SUBSCRIPTIONS (stub) ─────────────────────────────────────────────────

type sdmSubscription struct {
	NfInstanceID   string   `json:"nfInstanceId"`
	CallbackURI    string   `json:"callbackReference"`
	MonitoredData  []string `json:"monitoredResourceUris,omitempty"`
	SubscriptionID string   `json:"subscriptionId,omitempty"`
}

func (s *Server) handleSDMSubscribe(w http.ResponseWriter, r *http.Request) {
	// Store subscriptions in memory only — they expire and are low-stakes.
	// Return a synthetic subscription ID.
	var sub sdmSubscription
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		jsonError(w, http.StatusBadRequest, "bad_request")
		return
	}
	sub.SubscriptionID = "sub-" + fmt.Sprintf("%d", time.Now().UnixNano())
	w.Header().Set("Location", r.URL.Path+"/"+sub.SubscriptionID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sub)
}

func (s *Server) handleSDMUnsubscribe(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func resolveIMSI(r *http.Request) (string, error) {
	return ParseSUPI(chi.URLParam(r, "supi"))
}

// kbpsToString converts a kbps integer to Open5GS-format AMBR string.
func kbpsToString(kbps int) string {
	if kbps >= 1000000 {
		return fmt.Sprintf("%d Gbps", kbps/1000000)
	}
	if kbps >= 1000 {
		return fmt.Sprintf("%d Mbps", kbps/1000)
	}
	return fmt.Sprintf("%d Kbps", kbps)
}

// parseNSSAI unmarshals a JSON NSSAI array, defaulting to SST=1 on error.
func parseNSSAI(raw string) []snssai {
	var slices []snssai
	if err := json.Unmarshal([]byte(raw), &slices); err != nil || len(slices) == 0 {
		return []snssai{{SST: 1}}
	}
	return slices
}

// parseAPNList parses the comma-separated APN ID list stored in subscriber.apn_list.
func parseAPNList(list string) []int {
	var ids []int
	for _, s := range strings.Split(list, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		var id int
		if _, err := fmt.Sscanf(s, "%d", &id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}
