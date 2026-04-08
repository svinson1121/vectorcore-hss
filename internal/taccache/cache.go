// Package taccache provides a shared in-memory TAC (Type Allocation Code)
// lookup table. It is populated from the tac DB table at startup and updated
// in-place on every API write, giving the S13 EIR handler O(1) device
// make/model resolution with no database round-trip.
package taccache

import (
	"sync"

	"github.com/svinson1121/vectorcore-hss/internal/metrics"
)

// Entry holds the device make and model for one TAC.
type Entry struct {
	Make  string
	Model string
}

// Cache is a thread-safe in-memory map from TAC string to device info.
// A nil *Cache is safe to use — all operations become no-ops or return "not found".
type Cache struct {
	mu   sync.RWMutex
	data map[string]Entry
}

// New returns an empty, ready-to-use Cache.
func New() *Cache {
	return &Cache{data: make(map[string]Entry)}
}

// Load replaces the entire cache contents atomically.
// Call this at startup and again after a bulk CSV import.
func (c *Cache) Load(entries map[string]Entry) {
	c.mu.Lock()
	c.data = entries
	n := len(c.data)
	c.mu.Unlock()
	metrics.TACCacheSize.Set(float64(n))
}

// Set adds or overwrites a single TAC entry (called after API create/update).
func (c *Cache) Set(tac, make, model string) {
	c.mu.Lock()
	c.data[tac] = Entry{Make: make, Model: model}
	n := len(c.data)
	c.mu.Unlock()
	metrics.TACCacheSize.Set(float64(n))
}

// Delete removes a single TAC entry (called after API delete).
func (c *Cache) Delete(tac string) {
	c.mu.Lock()
	delete(c.data, tac)
	n := len(c.data)
	c.mu.Unlock()
	metrics.TACCacheSize.Set(float64(n))
}

// Lookup resolves an IMEI to a device make/model.
// It tries the first 8 digits (modern GSMA TAC), then falls back to 6 digits
// (pre-2004 format). Returns empty strings and false if not found.
// Records hit/miss in Prometheus.
func (c *Cache) Lookup(imei string) (make, model string, found bool) {
	if c == nil || len(imei) < 6 {
		metrics.TACLookupsTotal.WithLabelValues("miss").Inc()
		return "", "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(imei) >= 8 {
		if e, ok := c.data[imei[:8]]; ok {
			metrics.TACLookupsTotal.WithLabelValues("hit").Inc()
			return e.Make, e.Model, true
		}
	}
	if e, ok := c.data[imei[:6]]; ok {
		metrics.TACLookupsTotal.WithLabelValues("hit").Inc()
		return e.Make, e.Model, true
	}
	metrics.TACLookupsTotal.WithLabelValues("miss").Inc()
	return "", "", false
}

// Len returns the number of entries currently in the cache.
func (c *Cache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}
