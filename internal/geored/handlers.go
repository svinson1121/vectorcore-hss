package geored

// handlers.go -- Inbound HTTP handlers for the inter-node GeoRed listener.
//
// POST /geored/v1/events  — receive a batch of events from a peer.
// GET  /geored/v1/snapshot — return a full dynamic-state dump to a peer.
// GET  /geored/v1/health   — liveness probe.

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

// eventsHandler handles POST /geored/v1/events.
func eventsHandler(nodeID string, store repository.Repository, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var batch Batch
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Loop prevention: discard batches we sent ourselves.
		if batch.Source == nodeID {
			w.WriteHeader(http.StatusOK)
			return
		}

		log.Debug("geored: received batch",
			zap.String("source", batch.Source),
			zap.Int("events", len(batch.Events)),
		)

		ctx := r.Context()
		for _, e := range batch.Events {
			applyEvent(ctx, store, e, log)
		}

		w.WriteHeader(http.StatusOK)
	}
}

// snapshotHandler handles GET /geored/v1/snapshot.
func snapshotHandler(store repository.Repository, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		snap, err := BuildSnapshot(ctx, store)
		if err != nil {
			log.Error("geored: snapshot build failed", zap.Error(err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(snap)
	}
}

// healthHandler handles GET /geored/v1/health.
func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}

// applyEvent dispatches a single received event to the appropriate store method.
func applyEvent(ctx context.Context, store repository.Repository, e Event, log *zap.Logger) {
	switch e.Type {

	case EventSQNUpdate:
		var p PayloadSQNUpdate
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			log.Warn("geored: bad sqn_update payload", zap.Error(err))
			return
		}
		current, err := store.GetAUCByID(ctx, p.AUCID)
		if err != nil {
			return
		}
		if p.SQN > current.SQN {
			_ = store.ResyncSQN(ctx, p.AUCID, p.SQN)
		}

	case EventServingMME:
		var p PayloadServingMME
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			log.Warn("geored: bad serving_mme payload", zap.Error(err))
			return
		}
		applyServingMME(ctx, store, p)

	case EventServingSGSN:
		var p PayloadServingSGSN
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			log.Warn("geored: bad serving_sgsn payload", zap.Error(err))
			return
		}
		applyServingSGSN(ctx, store, p)

	case EventServingVLR:
		var p PayloadServingVLR
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			log.Warn("geored: bad serving_vlr payload", zap.Error(err))
			return
		}
		applyServingVLR(ctx, store, p)

	case EventServingMSC:
		var p PayloadServingMSC
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			log.Warn("geored: bad serving_msc payload", zap.Error(err))
			return
		}
		applyServingMSC(ctx, store, p)

	case EventIMSSCSCF:
		var p PayloadIMSSCSCF
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			log.Warn("geored: bad ims_scscf payload", zap.Error(err))
			return
		}
		applyIMSSCSCF(ctx, store, p)

	case EventIMSPCSCF:
		var p PayloadIMSPCSCF
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			log.Warn("geored: bad ims_pcscf payload", zap.Error(err))
			return
		}
		applyIMSPCSCF(ctx, store, p)

	case EventGxSessionAdd:
		var p PayloadGxSessionAdd
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			log.Warn("geored: bad gx_session_add payload", zap.Error(err))
			return
		}
		applyGxSessionAdd(ctx, store, p.IMSI, p.PCRFSessionID, p.APNID, p.APNName, p.PGWIP, p.UEIP)

	case EventGxSessionDel:
		var p PayloadGxSessionDel
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			log.Warn("geored: bad gx_session_del payload", zap.Error(err))
			return
		}
		_ = store.DeleteServingAPNBySession(ctx, p.PCRFSessionID)

	// OAM Put events carry a PayloadOAMPut wrapping the JSON-encoded model.
	case EventSubscriberPut, EventAUCPut, EventAPNPut, EventIMSSubPut, EventEIRPut:
		var oam PayloadOAMPut
		if err := json.Unmarshal(e.Payload, &oam); err != nil {
			log.Warn("geored: bad OAM put payload", zap.String("type", string(e.Type)), zap.Error(err))
			return
		}
		applyOAMRecord(ctx, store, e.Type, oam.Record)

	// OAM Del events carry a PayloadOAMDel with the record ID.
	case EventSubscriberDel, EventAUCDel, EventAPNDel, EventIMSSubDel, EventEIRDel:
		applyOAMRecord(ctx, store, e.Type, e.Payload)

	default:
		log.Warn("geored: unknown event type", zap.String("type", string(e.Type)))
	}
}
