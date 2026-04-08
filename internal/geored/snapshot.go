package geored

// snapshot.go -- Full dynamic-state dump used for peer resync on reconnect.
// Only dynamic state is included (SQN, serving state, IMS registrations, Gx
// sessions). Static provisioned data (subscriber, AUC, APN definitions) is
// assumed to be identical on all nodes — operators write to all nodes via the
// OAM channel.

import (
	"context"
	"encoding/json"
	"time"

	"github.com/svinson1121/vectorcore-hss/internal/models"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

// Snapshot is the wire format for a full dynamic-state dump.
type Snapshot struct {
	GeneratedAt  time.Time              `json:"generated_at"`
	SQNs         []SnapshotSQN          `json:"sqns"`
	ServingMMEs  []PayloadServingMME    `json:"serving_mmes"`
	ServingSGSNs []PayloadServingSGSN   `json:"serving_sgsns"`
	ServingVLRs  []PayloadServingVLR    `json:"serving_vlrs"`
	ServingMSCs  []PayloadServingMSC    `json:"serving_mscs"`
	IMSSCSCFs    []PayloadIMSSCSCF      `json:"ims_scscfs"`
	IMSPCSCFs    []PayloadIMSPCSCF      `json:"ims_pcscfs"`
	GxSessions   []SnapshotGxSession    `json:"gx_sessions"`
}

type SnapshotSQN struct {
	AUCID int   `json:"auc_id"`
	SQN   int64 `json:"sqn"`
}

type SnapshotGxSession struct {
	PCRFSessionID string  `json:"pcrf_session_id"`
	IMSI          string  `json:"imsi"`
	MSISDN        *string `json:"msisdn,omitempty"`
	APNID         *int    `json:"apn_id,omitempty"`
	APNName       *string `json:"apn_name,omitempty"`
	PGWIP         *string `json:"pgw_ip,omitempty"`
	UEIP          *string `json:"ue_ip,omitempty"`
}

// BuildSnapshot queries the local DB and assembles a full Snapshot.
func BuildSnapshot(ctx context.Context, store repository.Repository) (*Snapshot, error) {
	snap := &Snapshot{GeneratedAt: time.Now().UTC()}

	// SQNs — iterate all AUC records via a raw DB query exposed by the store.
	aucs, err := store.ListAllAUC(ctx)
	if err == nil {
		for _, a := range aucs {
			snap.SQNs = append(snap.SQNs, SnapshotSQN{AUCID: a.AUCID, SQN: a.SQN})
		}
	}

	// Serving MME / SGSN / VLR — iterate all subscribers.
	subs, err := store.ListAllSubscribers(ctx)
	if err == nil {
		for _, s := range subs {
			if s.ServingMME != nil {
				ts := ptrTime(s.ServingMMETimestamp)
				snap.ServingMMEs = append(snap.ServingMMEs, PayloadServingMME{
					IMSI:              s.IMSI,
					ServingMME:        s.ServingMME,
					ServingMMERealm:   s.ServingMMERealm,
					ServingMMEPeer:    s.ServingMMEPeer,
					Timestamp:         ts,
					MCC:               s.LastSeenMCC,
					MNC:               s.LastSeenMNC,
					TAC:               s.LastSeenTAC,
					ENodeBID:          s.LastSeenENodeBID,
					CellID:            s.LastSeenCellID,
					ECI:               s.LastSeenECI,
					LocationTimestamp: s.LastLocationUpdateTimestamp,
				})
			}
			if s.ServingSGSN != nil {
				snap.ServingSGSNs = append(snap.ServingSGSNs, PayloadServingSGSN{
					IMSI:        s.IMSI,
					ServingSGSN: s.ServingSGSN,
					Timestamp:   ptrTime(s.ServingSGSNTimestamp),
				})
			}
			if s.ServingVLR != nil {
				snap.ServingVLRs = append(snap.ServingVLRs, PayloadServingVLR{
					IMSI:       s.IMSI,
					ServingVLR: s.ServingVLR,
					Timestamp:  ptrTime(s.ServingVLRTimestamp),
				})
			}
			if s.ServingMSC != nil {
				snap.ServingMSCs = append(snap.ServingMSCs, PayloadServingMSC{
					IMSI:       s.IMSI,
					ServingMSC: s.ServingMSC,
					Timestamp:  ptrTime(s.ServingMSCTimestamp),
				})
			}
		}
	}

	// IMS SCSCF / PCSCF — iterate all IMS subscribers.
	imsSubs, err := store.ListAllIMSSubscribers(ctx)
	if err == nil {
		for _, s := range imsSubs {
			if s.SCSCF != nil {
				ts := ptrTime(s.SCSCFTimestamp)
				snap.IMSSCSCFs = append(snap.IMSSCSCFs, PayloadIMSSCSCF{
					MSISDN:    s.MSISDN,
					SCSCF:     s.SCSCF,
					Realm:     s.SCSCFRealm,
					Peer:      s.SCSCFPeer,
					Timestamp: ts,
				})
			}
			if s.PCSCF != nil {
				ts := ptrTime(s.PCSCFTimestamp)
				snap.IMSPCSCFs = append(snap.IMSPCSCFs, PayloadIMSPCSCF{
					MSISDN:    s.MSISDN,
					PCSCF:     s.PCSCF,
					Realm:     s.PCSCFRealm,
					Peer:      s.PCSCFPeer,
					Timestamp: ts,
				})
			}
		}
	}

	// Active Gx sessions.
	sessions, err := store.ListAllServingAPN(ctx)
	if err == nil {
		for _, s := range sessions {
			snap.GxSessions = append(snap.GxSessions, SnapshotGxSession{
				PCRFSessionID: s.PCRFSessionID,
				IMSI:          s.IMSI,
				MSISDN:        s.MSISDN,
				APNID:         s.APNID,
				APNName:       s.APNName,
				PGWIP:         s.PGWIP,
				UEIP:          s.UEIP,
			})
		}
	}

	return snap, nil
}

// ── apply helpers (also used by applySnapshot in manager.go) ─────────────────

func applyServingMME(ctx context.Context, store repository.Repository, p PayloadServingMME) {
	_ = store.UpdateServingMME(ctx, p.IMSI, &repository.ServingMMEUpdate{
		ServingMME:        p.ServingMME,
		Realm:             p.ServingMMERealm,
		Peer:              p.ServingMMEPeer,
		Timestamp:         p.Timestamp,
		MCC:               p.MCC,
		MNC:               p.MNC,
		TAC:               p.TAC,
		ENodeBID:          p.ENodeBID,
		CellID:            p.CellID,
		ECI:               p.ECI,
		LocationTimestamp: p.LocationTimestamp,
	})
}

func applyServingSGSN(ctx context.Context, store repository.Repository, p PayloadServingSGSN) {
	_ = store.UpdateServingSGSN(ctx, p.IMSI, &repository.ServingSGSNUpdate{
		ServingSGSN: p.ServingSGSN,
		Timestamp:   p.Timestamp,
	})
}

func applyServingVLR(ctx context.Context, store repository.Repository, p PayloadServingVLR) {
	_ = store.UpdateServingVLR(ctx, p.IMSI, &repository.ServingVLRUpdate{
		ServingVLR: p.ServingVLR,
		Timestamp:  p.Timestamp,
	})
}

func applyServingMSC(ctx context.Context, store repository.Repository, p PayloadServingMSC) {
	_ = store.UpdateServingMSC(ctx, p.IMSI, &repository.ServingMSCUpdate{
		ServingMSC: p.ServingMSC,
		Timestamp:  p.Timestamp,
	})
}

func applyIMSSCSCF(ctx context.Context, store repository.Repository, p PayloadIMSSCSCF) {
	_ = store.UpdateIMSSCSCF(ctx, p.MSISDN, &repository.IMSSCSCFUpdate{
		SCSCF:     p.SCSCF,
		Realm:     p.Realm,
		Peer:      p.Peer,
		Timestamp: p.Timestamp,
	})
}

func applyGxSessionAdd(ctx context.Context, store repository.Repository, imsi, pcrfSessionID string, apnID *int, apnName *string, pgwIP *string, ueIP *string) {
	sub, err := store.GetSubscriberByIMSI(ctx, imsi)
	if err != nil {
		return
	}
	apnIDInt := 0
	if apnID != nil {
		apnIDInt = *apnID
	}
	apnNameStr := ""
	if apnName != nil {
		apnNameStr = *apnName
	}
	_ = store.UpsertServingAPN(ctx, &models.ServingAPN{
		SubscriberID:  sub.SubscriberID,
		APNID:         apnIDInt,
		APNName:       apnNameStr,
		PCRFSessionID: &pcrfSessionID,
		ServingPGW:    pgwIP,
		UEIP:          ueIP,
	})
}

func applyIMSPCSCF(ctx context.Context, store repository.Repository, p PayloadIMSPCSCF) {
	_ = store.UpdateIMSPCSCF(ctx, p.MSISDN, &repository.IMSPCSCFUpdate{
		PCSCF:     p.PCSCF,
		Realm:     p.Realm,
		Peer:      p.Peer,
		Timestamp: p.Timestamp,
	})
}

func applyOAMRecord(ctx context.Context, store repository.Repository, evType EventType, raw []byte) {
	switch evType {
	case EventSubscriberPut:
		var rec models.Subscriber
		if err := unmarshalInto(raw, &rec); err == nil {
			_ = store.UpsertSubscriber(ctx, &rec)
		}
	case EventSubscriberDel:
		var p PayloadOAMDel
		if err := unmarshalInto(raw, &p); err == nil {
			if imsi, ok := p.ID.(string); ok {
				_ = store.DeleteSubscriberByIMSI(ctx, imsi)
			}
		}
	case EventAUCPut:
		var rec models.AUC
		if err := unmarshalInto(raw, &rec); err == nil {
			_ = store.UpsertAUC(ctx, &rec)
		}
	case EventAUCDel:
		var p PayloadOAMDel
		if err := unmarshalInto(raw, &p); err == nil {
			if id, ok := toInt(p.ID); ok {
				_ = store.DeleteAUCByID(ctx, id)
			}
		}
	case EventAPNPut:
		var rec models.APN
		if err := unmarshalInto(raw, &rec); err == nil {
			_ = store.UpsertAPN(ctx, &rec)
		}
	case EventAPNDel:
		var p PayloadOAMDel
		if err := unmarshalInto(raw, &p); err == nil {
			if id, ok := toInt(p.ID); ok {
				_ = store.DeleteAPNByID(ctx, id)
			}
		}
	case EventIMSSubPut:
		var rec models.IMSSubscriber
		if err := unmarshalInto(raw, &rec); err == nil {
			_ = store.UpsertIMSSubscriber(ctx, &rec)
		}
	case EventIMSSubDel:
		var p PayloadOAMDel
		if err := unmarshalInto(raw, &p); err == nil {
			if msisdn, ok := p.ID.(string); ok {
				_ = store.DeleteIMSSubscriberByMSISDN(ctx, msisdn)
			}
		}
	case EventEIRPut:
		var rec models.EIR
		if err := unmarshalInto(raw, &rec); err == nil {
			_ = store.UpsertEIR(ctx, &rec)
		}
	case EventEIRDel:
		var p PayloadOAMDel
		if err := unmarshalInto(raw, &p); err == nil {
			if id, ok := toInt(p.ID); ok {
				_ = store.DeleteEIRByID(ctx, id)
			}
		}
	}
}

func ptrTime(t interface{}) *time.Time {
	// Handle both *time.Time and string types that GORM may return.
	if v, ok := t.(*time.Time); ok {
		return v
	}
	return nil
}

func toInt(v interface{}) (int, bool) {
	switch x := v.(type) {
	case float64:
		return int(x), true
	case int:
		return x, true
	}
	return 0, false
}

func unmarshalInto(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
