package peertracker

import (
	"sync"
	"time"
)

// Peer is one currently connected remote peer/client.
type Peer struct {
	Name       string `json:"name"`
	RemoteAddr string `json:"remote_addr"`
	Transport  string `json:"transport"`
}

// Tracker stores a live snapshot of connected peers keyed by remote address.
type Tracker struct {
	mu     sync.RWMutex
	peers  map[string]entry
	maxAge time.Duration
}

type entry struct {
	peer     Peer
	lastSeen time.Time
}

func New() *Tracker {
	return &Tracker{peers: make(map[string]entry)}
}

func NewWithMaxAge(maxAge time.Duration) *Tracker {
	return &Tracker{peers: make(map[string]entry), maxAge: maxAge}
}

func (t *Tracker) Add(p Peer) {
	t.mu.Lock()
	t.peers[p.RemoteAddr] = entry{peer: p, lastSeen: time.Now()}
	t.mu.Unlock()
}

func (t *Tracker) Rename(remoteAddr, name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	p, ok := t.peers[remoteAddr]
	if !ok {
		return
	}
	p.peer.Name = name
	p.lastSeen = time.Now()
	t.peers[remoteAddr] = p
}

func (t *Tracker) Remove(remoteAddr string) {
	t.mu.Lock()
	delete(t.peers, remoteAddr)
	t.mu.Unlock()
}

func (t *Tracker) List() []Peer {
	t.pruneExpired(time.Now())
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]Peer, 0, len(t.peers))
	for _, p := range t.peers {
		out = append(out, p.peer)
	}
	return out
}

func (t *Tracker) pruneExpired(now time.Time) {
	if t == nil || t.maxAge <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for key, p := range t.peers {
		if now.Sub(p.lastSeen) > t.maxAge {
			delete(t.peers, key)
		}
	}
}
