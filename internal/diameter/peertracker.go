package diameter

import (
	"sync"
)

// ConnectedPeer holds information about a directly connected Diameter peer,
// populated from the CER/CEA handshake.
type ConnectedPeer struct {
	OriginHost  string `json:"origin_host"`
	OriginRealm string `json:"origin_realm"`
	RemoteAddr  string `json:"remote_addr"`
	Transport   string `json:"transport"` // "tcp" or "sctp"
}

// PeerTracker tracks physical Diameter connections established via CER/CEA.
// It is keyed by remote address so that a DRA with multiple logical identities
// behind it appears as a single entry.
type PeerTracker struct {
	mu    sync.RWMutex
	peers map[string]ConnectedPeer // key: remoteAddr (host:port)
}

func newPeerTracker() *PeerTracker {
	return &PeerTracker{peers: make(map[string]ConnectedPeer)}
}

func (pt *PeerTracker) add(p ConnectedPeer) {
	pt.mu.Lock()
	pt.peers[p.RemoteAddr] = p
	pt.mu.Unlock()
}

func (pt *PeerTracker) remove(remoteAddr string) {
	pt.mu.Lock()
	delete(pt.peers, remoteAddr)
	pt.mu.Unlock()
}

// List returns a snapshot of all directly connected peers.
func (pt *PeerTracker) List() []ConnectedPeer {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	out := make([]ConnectedPeer, 0, len(pt.peers))
	for _, p := range pt.peers {
		out = append(out, p)
	}
	return out
}
