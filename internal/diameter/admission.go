package diameter

import (
	"context"

	"github.com/svinson1121/vectorcore-hss/internal/metrics"
)

// admission is a bounded concurrency gate for one application family (e.g.
// "s6a" or "gx"). It exists because the shared database connection pool
// (internal/config.DatabaseConfig.MaxOpenConns) has no per-application
// reservation: without a gate, a burst of Diameter requests on one
// interface (e.g. an AIR/ULR storm from a mass eNB/MME reconnect) can hold
// enough pool connections that a concurrent request on another interface
// (e.g. a Gx CCR-I) times out waiting for a connection that never frees up
// in time. Each admission instance caps how many handler executions for its
// application family may run concurrently, independent of the other
// family's gate, so one can never fully starve the other.
type admission struct {
	name string
	sem  chan struct{}
}

func newAdmission(name string, n int) *admission {
	if n <= 0 {
		n = 1
	}
	return &admission{name: name, sem: make(chan struct{}, n)}
}

// acquire blocks until a slot is free or ctx is done, whichever comes
// first. It returns false if ctx expired before a slot became available.
func (a *admission) acquire(ctx context.Context) bool {
	select {
	case a.sem <- struct{}{}:
		metrics.AdmissionInFlight.WithLabelValues(a.name).Inc()
		return true
	case <-ctx.Done():
		return false
	}
}

// release frees the slot acquired by a successful acquire call.
func (a *admission) release() {
	metrics.AdmissionInFlight.WithLabelValues(a.name).Dec()
	<-a.sem
}
