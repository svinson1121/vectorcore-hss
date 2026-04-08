package geored

// manager.go -- Central GeoRed manager: owns all peer clients, implements
// Publisher, and drives the periodic resync timer.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/config"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

// Manager implements Publisher and orchestrates all peer clients.
type Manager struct {
	cfg   config.GeoredConfig
	peers []*peerClient
	store repository.Repository
	log   *zap.Logger
}

// New creates a Manager, starts one peerClient goroutine per configured peer,
// and optionally starts the periodic resync timer.
func New(cfg config.GeoredConfig, store repository.Repository, log *zap.Logger) *Manager {
	batchMax := cfg.BatchMaxEvents
	if batchMax <= 0 {
		batchMax = 500
	}
	batchAgeMs := cfg.BatchMaxAgeMs
	if batchAgeMs <= 0 {
		batchAgeMs = 10
	}
	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = 50000
	}

	m := &Manager{cfg: cfg, store: store, log: log}
	for _, p := range cfg.Peers {
		c := newPeerClient(p, cfg.NodeID, batchMax, batchAgeMs, queueSize, log)
		m.peers = append(m.peers, c)
		go c.run()
	}
	if cfg.PeriodicSyncIntervalS > 0 {
		go m.periodicSync(time.Duration(cfg.PeriodicSyncIntervalS) * time.Second)
	}
	log.Info("geored: manager started",
		zap.String("node_id", cfg.NodeID),
		zap.Int("peers", len(m.peers)),
	)
	return m
}

// Publish fans out a single event to all peer queues (non-blocking).
func (m *Manager) Publish(e Event) {
	for _, p := range m.peers {
		p.enqueue(e)
	}
}

// ── Typed publish helpers called by Diameter / GSUP / API handlers ────────────

func (m *Manager) PublishSQNUpdate(aucID int, sqn int64) {
	m.Publish(newEvent(EventSQNUpdate, PayloadSQNUpdate{AUCID: aucID, SQN: sqn}))
}

func (m *Manager) PublishServingMME(p PayloadServingMME) {
	m.Publish(newEvent(EventServingMME, p))
}

func (m *Manager) PublishServingSGSN(p PayloadServingSGSN) {
	m.Publish(newEvent(EventServingSGSN, p))
}

func (m *Manager) PublishServingVLR(p PayloadServingVLR) {
	m.Publish(newEvent(EventServingVLR, p))
}

func (m *Manager) PublishServingMSC(p PayloadServingMSC) {
	m.Publish(newEvent(EventServingMSC, p))
}

func (m *Manager) PublishIMSSCSCF(p PayloadIMSSCSCF) {
	m.Publish(newEvent(EventIMSSCSCF, p))
}

func (m *Manager) PublishIMSPCSCF(p PayloadIMSPCSCF) {
	m.Publish(newEvent(EventIMSPCSCF, p))
}

func (m *Manager) PublishGxSessionAdd(p PayloadGxSessionAdd) {
	m.Publish(newEvent(EventGxSessionAdd, p))
}

func (m *Manager) PublishGxSessionDel(sessionID string) {
	m.Publish(newEvent(EventGxSessionDel, PayloadGxSessionDel{PCRFSessionID: sessionID}))
}

func (m *Manager) PublishOAMPut(evType EventType, record interface{}) {
	raw, _ := json.Marshal(record)
	m.Publish(newEvent(evType, PayloadOAMPut{Record: raw}))
}

func (m *Manager) PublishOAMDel(evType EventType, id interface{}) {
	m.Publish(newEvent(evType, PayloadOAMDel{ID: id}))
}

// ── Resync ────────────────────────────────────────────────────────────────────

// TriggerResync fetches a fresh snapshot from all peers and applies it.
func (m *Manager) TriggerResync(ctx context.Context) {
	for _, p := range m.peers {
		go func(peer *peerClient) {
			snap, err := peer.fetchSnapshot(ctx)
			if err != nil {
				m.log.Warn("geored: resync fetch failed",
					zap.String("peer", peer.cfg.NodeID), zap.Error(err))
				return
			}
			if err := m.applySnapshot(ctx, snap); err != nil {
				m.log.Warn("geored: resync apply failed",
					zap.String("peer", peer.cfg.NodeID), zap.Error(err))
				return
			}
			m.log.Info("geored: resync complete", zap.String("peer", peer.cfg.NodeID))
		}(p)
	}
}

// TriggerResyncPeer triggers a resync from one specific peer by node_id.
func (m *Manager) TriggerResyncPeer(ctx context.Context, nodeID string) error {
	for _, p := range m.peers {
		if p.cfg.NodeID == nodeID {
			snap, err := p.fetchSnapshot(ctx)
			if err != nil {
				return err
			}
			return m.applySnapshot(ctx, snap)
		}
	}
	return fmt.Errorf("geored: unknown peer %q", nodeID)
}

func (m *Manager) periodicSync(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for range t.C {
		m.log.Info("geored: periodic resync starting")
		m.TriggerResync(context.Background())
	}
}

// ── Status ────────────────────────────────────────────────────────────────────

// PeerStatus describes one peer's current health from our perspective.
type PeerStatus struct {
	NodeID     string    `json:"node_id"`
	Address    string    `json:"address"`
	Healthy    bool      `json:"healthy"`
	QueueDepth int       `json:"queue_depth"`
	LastSentAt time.Time `json:"last_sent_at,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
}

// Status returns the health of all configured peers.
func (m *Manager) Status() []PeerStatus {
	out := make([]PeerStatus, len(m.peers))
	for i, p := range m.peers {
		out[i] = p.status()
	}
	return out
}

// ── Snapshot apply ────────────────────────────────────────────────────────────

// applySnapshot reconciles a remote peer's dynamic state with our local DB.
// Conflict resolution: timestamp-wins for state; max() for SQN.
func (m *Manager) applySnapshot(ctx context.Context, snap *Snapshot) error {
	for _, entry := range snap.SQNs {
		current, err := m.store.GetAUCByID(ctx, entry.AUCID)
		if err != nil {
			continue
		}
		if entry.SQN > current.SQN {
			_ = m.store.ResyncSQN(ctx, entry.AUCID, entry.SQN)
		}
	}
	for _, e := range snap.ServingMMEs {
		applyServingMME(ctx, m.store, e)
	}
	for _, e := range snap.ServingSGSNs {
		applyServingSGSN(ctx, m.store, e)
	}
	for _, e := range snap.ServingVLRs {
		applyServingVLR(ctx, m.store, e)
	}
	for _, e := range snap.ServingMSCs {
		applyServingMSC(ctx, m.store, e)
	}
	for _, e := range snap.IMSSCSCFs {
		applyIMSSCSCF(ctx, m.store, e)
	}
	for _, e := range snap.IMSPCSCFs {
		applyIMSPCSCF(ctx, m.store, e)
	}
	for _, e := range snap.GxSessions {
		applyGxSessionAdd(ctx, m.store, e.IMSI, e.PCRFSessionID, e.APNID, e.APNName, e.PGWIP, e.UEIP)
	}
	return nil
}
