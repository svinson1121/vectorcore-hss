package udm

// auth.go — Nudm_UEAuthentication handler.
//
// POST /nudm-ueau/v{1,2}/{supi}/security-information/generate-auth-data
//
// Called by Open5GS AUSF when a UE initiates a 5G registration.
// We run Milenage + 5G-AKA key derivation and return the auth vector.

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/crypto"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

// authRequest is the JSON body sent by the AUSF.
type authRequest struct {
	ServingNetworkName  string               `json:"servingNetworkName"`
	AUSFInstanceID      string               `json:"ausfInstanceId"`
	ResynchronizationInfo *resyncInfo        `json:"resynchronizationInfo,omitempty"`
}

type resyncInfo struct {
	Rand string `json:"rand"`
	Auts string `json:"auts"`
}

// authResponse is the JSON body returned to the AUSF.
type authResponse struct {
	AuthType             string             `json:"authType"`
	SUPI                 string             `json:"supi"`
	AuthenticationVector *authVector5GAKA   `json:"authenticationVector"`
}

type authVector5GAKA struct {
	AVType   string `json:"avType"`
	Rand     string `json:"rand"`
	Autn     string `json:"autn"`
	XresStar string `json:"xresStar"`
	Kausf    string `json:"kausf"`
}

func (s *Server) handleGenerateAuthData(w http.ResponseWriter, r *http.Request) {
	supiRaw := chi.URLParam(r, "supi")
	imsi, err := ParseSUPI(supiRaw)
	if err != nil {
		s.log.Warn("udm: auth bad SUPI", zap.String("supi", supiRaw), zap.Error(err))
		jsonError(w, http.StatusBadRequest, "invalid_supi")
		return
	}

	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "bad_request")
		return
	}

	snn := req.ServingNetworkName
	if snn == "" {
		jsonError(w, http.StatusBadRequest, "missing_serving_network_name")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	auc, err := s.store.GetAUCByIMSI(ctx, imsi)
	if err == repository.ErrNotFound {
		s.log.Warn("udm: auth unknown IMSI", zap.String("imsi", imsi))
		jsonError(w, http.StatusNotFound, "user_not_found")
		return
	}
	if err != nil {
		s.log.Error("udm: auth db error", zap.String("imsi", imsi), zap.Error(err))
		jsonError(w, http.StatusInternalServerError, "db_error")
		return
	}

	profile, err := crypto.LoadProfile(ctx, s.store, auc.AlgorithmProfileID)
	if err != nil {
		s.log.Error("udm: auth profile load error", zap.String("imsi", imsi), zap.Error(err))
		jsonError(w, http.StatusInternalServerError, "profile_error")
		return
	}

	var vec *crypto.FiveGAKAVector
	if req.ResynchronizationInfo != nil {
		// AUTS resync: UE rejected our challenge — recover SQN and regenerate.
		randBytes, err1 := hex.DecodeString(req.ResynchronizationInfo.Rand)
		autsBytes, err2 := hex.DecodeString(req.ResynchronizationInfo.Auts)
		if err1 != nil || err2 != nil {
			jsonError(w, http.StatusBadRequest, "invalid_resync_info")
			return
		}
		vec, err = crypto.ResyncAnd5GAKAVector(auc, profile, snn, randBytes, autsBytes, s.store, ctx)
	} else {
		vec, err = crypto.Generate5GAKAVector(auc, profile, snn, s.store, ctx)
	}
	if err != nil {
		s.log.Error("udm: auth vector generation failed", zap.String("imsi", imsi), zap.Error(err))
		jsonError(w, http.StatusInternalServerError, "vector_generation_failed")
		return
	}

	resp := authResponse{
		AuthType: "5G_AKA",
		SUPI:     "imsi-" + imsi,
		AuthenticationVector: &authVector5GAKA{
			AVType:   "5G_HE_AKA",
			Rand:     hex.EncodeToString(vec.RAND),
			Autn:     hex.EncodeToString(vec.AUTN),
			XresStar: hex.EncodeToString(vec.XRESStar),
			Kausf:    hex.EncodeToString(vec.KAUSF),
		},
	}

	s.log.Info("udm: auth success",
		zap.String("imsi", imsi),
		zap.String("snn", snn),
	)
	jsonOK(w, resp)
}

// handleAuthEvent receives auth success/failure notifications from the AUSF.
// POST /nudm-ueau/v{1,2}/{supi}/auth-events — no action needed, just 201.
func (s *Server) handleAuthEvent(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
}
