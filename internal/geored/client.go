package geored

// client.go -- Outbound peer client: batches events and pushes them over HTTP/2.
// Each peer has one peerClient with its own bounded channel and worker goroutine.
//
// HTTP/2 transport:
//   https:// peers → standard TLS + ALPN h2 negotiation (automatic)
//   http://  peers → h2c (cleartext HTTP/2) via custom DialTLSContext

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/http2"

	"github.com/svinson1121/vectorcore-hss/internal/config"
)

type peerClient struct {
	cfg        config.GeoredPeer
	nodeID     string
	batchMax   int
	batchAgeMs int
	ch         chan Event
	log        *zap.Logger
	http       *http.Client

	mu       sync.Mutex
	healthy  bool
	lastSent time.Time
	lastErr  string

	queueDepth int32 // accessed atomically
}

func newPeerClient(cfg config.GeoredPeer, nodeID string, batchMax, batchAgeMs, queueSize int, log *zap.Logger) *peerClient {
	var transport http.RoundTripper

	isH2C := len(cfg.Address) >= 7 && cfg.Address[:7] == "http://"
	if isH2C {
		// h2c: HTTP/2 over cleartext TCP. Use a plain net.Dial in DialTLSContext
		// so http2.Transport skips TLS while still speaking HTTP/2 framing.
		transport = &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		}
	} else {
		// TLS: standard http2.Transport; TLS + ALPN negotiation is automatic.
		transport = &http2.Transport{}
	}

	return &peerClient{
		cfg:        cfg,
		nodeID:     nodeID,
		batchMax:   batchMax,
		batchAgeMs: batchAgeMs,
		ch:         make(chan Event, queueSize),
		log:        log,
		http:       &http.Client{Transport: transport, Timeout: 10 * time.Second},
		healthy:    true,
	}
}

// enqueue adds an event to the peer's outbound queue.
// If the queue is full it drops the oldest event and tries once more.
func (c *peerClient) enqueue(e Event) {
	select {
	case c.ch <- e:
		atomic.AddInt32(&c.queueDepth, 1)
	default:
		// Queue full: drain one stale event, then try again.
		select {
		case <-c.ch:
			atomic.AddInt32(&c.queueDepth, -1)
		default:
		}
		select {
		case c.ch <- e:
			atomic.AddInt32(&c.queueDepth, 1)
		default:
		}
		c.log.Warn("geored: peer queue full, event dropped",
			zap.String("peer", c.cfg.NodeID),
			zap.String("event", string(e.Type)),
		)
	}
}

// run is the long-lived worker goroutine for this peer. It accumulates events
// into batches and flushes based on size or age.
func (c *peerClient) run() {
	flushAge := time.Duration(c.batchAgeMs) * time.Millisecond
	if flushAge <= 0 {
		flushAge = 10 * time.Millisecond
	}
	ticker := time.NewTicker(flushAge)
	defer ticker.Stop()

	var batch []Event

	flush := func() {
		if len(batch) == 0 {
			return
		}
		toSend := batch
		batch = nil
		if err := c.push(toSend); err != nil {
			c.setUnhealthy(err.Error())
		} else {
			c.setHealthy()
		}
	}

	for {
		select {
		case e := <-c.ch:
			atomic.AddInt32(&c.queueDepth, -1)
			batch = append(batch, e)
			if len(batch) >= c.batchMax {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// push serialises a batch and POSTs it to the peer.
func (c *peerClient) push(events []Event) error {
	b := Batch{Source: c.nodeID, Events: events}
	body, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.Address+"/geored/v1/events", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.BearerToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("peer returned HTTP %d", resp.StatusCode)
	}

	c.mu.Lock()
	c.lastSent = time.Now()
	c.mu.Unlock()
	return nil
}

// fetchSnapshot requests a full state snapshot from the peer for resync.
func (c *peerClient) fetchSnapshot(ctx context.Context) (*Snapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.cfg.Address+"/geored/v1/snapshot", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.BearerToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("snapshot: peer returned HTTP %d", resp.StatusCode)
	}
	var snap Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		return nil, fmt.Errorf("snapshot decode: %w", err)
	}
	return &snap, nil
}

func (c *peerClient) setHealthy() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.healthy = true
	c.lastErr = ""
}

func (c *peerClient) setUnhealthy(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.healthy = false
	c.lastErr = msg
	c.log.Warn("geored: peer push failed",
		zap.String("peer", c.cfg.NodeID),
		zap.String("error", msg),
	)
}

func (c *peerClient) status() PeerStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return PeerStatus{
		NodeID:     c.cfg.NodeID,
		Address:    c.cfg.Address,
		Healthy:    c.healthy,
		QueueDepth: int(atomic.LoadInt32(&c.queueDepth)),
		LastSentAt: c.lastSent,
		LastError:  c.lastErr,
	}
}
