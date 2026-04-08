package diameter

import (
	"sync"

	"github.com/fiorix/go-diameter/v4/diam"
)

// ConnTracker maps peer OriginHost → active Diameter connection.
// The tracker is populated by the handler wrapper in server.go on every
// incoming request, so any MME that has sent at least one message is reachable.
type ConnTracker struct {
	mu    sync.RWMutex
	conns map[string]diam.Conn
}

func newConnTracker() *ConnTracker {
	return &ConnTracker{conns: make(map[string]diam.Conn)}
}

func (ct *ConnTracker) Set(originHost string, conn diam.Conn) {
	ct.mu.Lock()
	ct.conns[originHost] = conn
	ct.mu.Unlock()
}

func (ct *ConnTracker) Get(originHost string) (diam.Conn, bool) {
	ct.mu.RLock()
	conn, ok := ct.conns[originHost]
	ct.mu.RUnlock()
	return conn, ok
}

func (ct *ConnTracker) Delete(originHost string) {
	ct.mu.Lock()
	delete(ct.conns, originHost)
	ct.mu.Unlock()
}

// GetConn satisfies s6a.PeerLookup.
func (ct *ConnTracker) GetConn(originHost string) (diam.Conn, bool) {
	return ct.Get(originHost)
}

// List returns a snapshot of all currently tracked peers as a map of
// OriginHost → remote address string.
func (ct *ConnTracker) List() map[string]string {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	out := make(map[string]string, len(ct.conns))
	for host, conn := range ct.conns {
		out[host] = conn.RemoteAddr().String()
	}
	return out
}
