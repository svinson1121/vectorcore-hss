package udm

// uecm.go — Nudm_UEContextManagement handlers.
//
// PUT    /nudm-uecm/v1/{supi}/registrations/amf-3gpp-access    — AMF registers/updates
// PATCH  /nudm-uecm/v1/{supi}/registrations/amf-3gpp-access    — AMF partial update
// DELETE /nudm-uecm/v1/{supi}/registrations/amf-3gpp-access    — AMF deregisters
// GET    /nudm-uecm/v1/{supi}/registrations                     — read all registrations
// PUT    /nudm-uecm/v1/{supi}/registrations/smf-registrations/{pduSessionId}
// DELETE /nudm-uecm/v1/{supi}/registrations/smf-registrations/{pduSessionId}

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/models"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
	"github.com/svinson1121/vectorcore-hss/internal/sbi"
)

// ── AMF 3GPP access registration ─────────────────────────────────────────────

type amfRegistration struct {
	AMFInstanceID    string `json:"amfInstanceId"`
	GUAMI            *guami `json:"guami,omitempty"`
	RATType          string `json:"ratType,omitempty"`
	IMSVoPS3GPP      bool   `json:"imsVoPs3gpp,omitempty"`
	DeregCallbackURI string `json:"deregCallbackUri,omitempty"`
}

type guami struct {
	PLMNId plmnID `json:"plmnId"`
	AMFId  string `json:"amfId"`
}

type plmnID struct {
	MCC string `json:"mcc"`
	MNC string `json:"mnc"`
}

func (s *Server) handleAMFRegistrationPut(w http.ResponseWriter, r *http.Request) {
	metaFields := sbi.RequestMetaFromContext(r.Context()).LogFields()
	imsi, err := resolveIMSI(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_supi")
		return
	}

	var reg amfRegistration
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		jsonError(w, http.StatusBadRequest, "bad_request")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	ts := time.Now().UTC()
	amfAddr := reg.AMFInstanceID
	if reg.GUAMI != nil {
		amfAddr = reg.GUAMI.PLMNId.MCC + "-" + reg.GUAMI.PLMNId.MNC + "-" + reg.GUAMI.AMFId
	}
	err = s.store.UpdateServingAMF(ctx, imsi, &repository.ServingAMFUpdate{
		ServingAMF:    &amfAddr,
		AMFInstanceID: &reg.AMFInstanceID,
		Timestamp:     &ts,
	})
	if err == repository.ErrNotFound {
		jsonError(w, http.StatusNotFound, "user_not_found")
		return
	}
	if err != nil {
		s.log.Error("udm: amf reg db error", append(metaFields, zap.String("imsi", imsi), zap.Error(err))...)
		jsonError(w, http.StatusInternalServerError, "db_error")
		return
	}

	s.log.Info("udm: AMF registered", append(metaFields, zap.String("imsi", imsi), zap.String("amf", amfAddr))...)
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleAMFRegistrationPatch(w http.ResponseWriter, r *http.Request) {
	// Treat PATCH same as PUT for context update.
	s.handleAMFRegistrationPut(w, r)
}

func (s *Server) handleAMFRegistrationDelete(w http.ResponseWriter, r *http.Request) {
	metaFields := sbi.RequestMetaFromContext(r.Context()).LogFields()
	imsi, err := resolveIMSI(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_supi")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_ = s.store.UpdateServingAMF(ctx, imsi, &repository.ServingAMFUpdate{
		ServingAMF:    nil,
		AMFInstanceID: nil,
		Timestamp:     nil,
	})

	s.log.Info("udm: AMF deregistered", append(metaFields, zap.String("imsi", imsi))...)
	w.WriteHeader(http.StatusNoContent)
}

// ── GET /registrations ───────────────────────────────────────────────────────

type registrations struct {
	AMF3GPPAccess *amfRegistrationResp     `json:"amf3GppAccessRegistration,omitempty"`
	PDUSessions   []pduSessionRegistration `json:"smfRegistrations,omitempty"`
}

type amfRegistrationResp struct {
	AMFInstanceID string `json:"amfInstanceId"`
}

type pduSessionRegistration struct {
	SMFInstanceID string `json:"smfInstanceId"`
	PDUSessionID  int    `json:"pduSessionId"`
	DNN           string `json:"dnn,omitempty"`
	SNSSAI        string `json:"singleNssai,omitempty"`
}

func (s *Server) handleGetRegistrations(w http.ResponseWriter, r *http.Request) {
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
		jsonError(w, http.StatusInternalServerError, "db_error")
		return
	}

	resp := registrations{}
	if sub.ServingAMFInstanceID != nil && *sub.ServingAMFInstanceID != "" {
		resp.AMF3GPPAccess = &amfRegistrationResp{AMFInstanceID: *sub.ServingAMFInstanceID}
	}

	sessions, _ := s.store.ListServingPDUSessions(ctx, imsi)
	for _, sess := range sessions {
		resp.PDUSessions = append(resp.PDUSessions, pduSessionRegistration{
			SMFInstanceID: sess.SMFInstanceID,
			PDUSessionID:  sess.PDUSessionID,
			DNN:           sess.DNN,
			SNSSAI:        sess.SNSSAI,
		})
	}

	jsonOK(w, resp)
}

// ── SMF PDU session registration ─────────────────────────────────────────────

type smfPDURegistration struct {
	SMFInstanceID string          `json:"smfInstanceId"`
	SMFSetID      string          `json:"smfSetId,omitempty"`
	DNN           string          `json:"dnn,omitempty"`
	SingleNSSAI   json.RawMessage `json:"singleNssai,omitempty"`
	PLMNId        json.RawMessage `json:"plmnId,omitempty"`
}

func (s *Server) handleSMFRegistrationPut(w http.ResponseWriter, r *http.Request) {
	metaFields := sbi.RequestMetaFromContext(r.Context()).LogFields()
	imsi, err := resolveIMSI(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_supi")
		return
	}
	pduSessionIDStr := chi.URLParam(r, "pduSessionId")
	pduSessionID, err := strconv.Atoi(pduSessionIDStr)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_pdu_session_id")
		return
	}

	var reg smfPDURegistration
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		jsonError(w, http.StatusBadRequest, "bad_request")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rec := &models.ServingPDUSession{
		IMSI:          imsi,
		PDUSessionID:  pduSessionID,
		SMFInstanceID: reg.SMFInstanceID,
		SMFSetID:      reg.SMFSetID,
		DNN:           reg.DNN,
		SNSSAI:        canonicalJSON(reg.SingleNSSAI),
		PLMNIDStr:     canonicalPLMN(reg.PLMNId),
	}
	if err := s.store.UpsertServingPDUSession(ctx, rec); err != nil {
		s.log.Error("udm: smf reg db error", append(metaFields, zap.String("imsi", imsi), zap.Error(err))...)
		jsonError(w, http.StatusInternalServerError, "db_error")
		return
	}

	s.log.Info("udm: SMF PDU session registered", append(metaFields,
		zap.String("imsi", imsi),
		zap.Int("pdu_session_id", pduSessionID),
		zap.String("smf", reg.SMFInstanceID),
	)...)
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleSMFRegistrationDelete(w http.ResponseWriter, r *http.Request) {
	metaFields := sbi.RequestMetaFromContext(r.Context()).LogFields()
	imsi, err := resolveIMSI(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_supi")
		return
	}
	pduSessionID, err := strconv.Atoi(chi.URLParam(r, "pduSessionId"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid_pdu_session_id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_ = s.store.DeleteServingPDUSession(ctx, imsi, pduSessionID)
	s.log.Info("udm: SMF PDU session deregistered", append(metaFields,
		zap.String("imsi", imsi),
		zap.Int("pdu_session_id", pduSessionID),
	)...)
	w.WriteHeader(http.StatusNoContent)
}

func canonicalJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(out)
}

func canonicalPLMN(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var plmn struct {
		MCC string `json:"mcc"`
		MNC string `json:"mnc"`
	}
	if err := json.Unmarshal(raw, &plmn); err == nil && (plmn.MCC != "" || plmn.MNC != "") {
		if plmn.MCC != "" && plmn.MNC != "" {
			return plmn.MCC + "-" + plmn.MNC
		}
		if plmn.MCC != "" {
			return plmn.MCC
		}
		return plmn.MNC
	}
	return canonicalJSON(raw)
}
