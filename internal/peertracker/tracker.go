package peertracker

import "sync"

// Peer is one currently connected remote peer/client.
type Peer struct {
	Name       string `json:"name"`
	RemoteAddr string `json:"remote_addr"`
	Transport  string `json:"transport"`
}

// Tracker stores a live snapshot of connected peers keyed by remote address.
type Tracker struct {
	mu    sync.RWMutex
	peers map[string]Peer
}

func New() *Tracker {
	return &Tracker{peers: make(map[string]Peer)}
}

func (t *Tracker) Add(p Peer) {
	t.mu.Lock()
	t.peers[p.RemoteAddr] = p
	t.mu.Unlock()
}

func (t *Tracker) Rename(remoteAddr, name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	p, ok := t.peers[remoteAddr]
	if !ok {
		return
	}
	p.Name = name
	t.peers[remoteAddr] = p
}

func (t *Tracker) Remove(remoteAddr string) {
	t.mu.Lock()
	delete(t.peers, remoteAddr)
	t.mu.Unlock()
}

func (t *Tracker) List() []Peer {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]Peer, 0, len(t.peers))
	for _, p := range t.peers {
		out = append(out, p)
	}
	return out
}
