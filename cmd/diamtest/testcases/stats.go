package testcases

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// recorder accumulates success/failure counts and latencies for one message
// type (AIR, ULR, or CCR-I) during a load/storm run. It's the same
// success/timeout/percentile pattern RunLoad uses, pulled out so the storm
// test can report AIR, ULR, and CCR-I as three independent blocks — seeing
// CCR-I degrade while AIR/ULR keep running under load is the whole point of
// the storm test.
type recorder struct {
	name      string
	nSent     atomic.Int64
	nSuccess  atomic.Int64
	nError    atomic.Int64
	nTimeout  atomic.Int64
	latMu     sync.Mutex
	latencies []int64 // nanoseconds, successful requests only
}

func newRecorder(name string) *recorder { return &recorder{name: name} }

func (r *recorder) ok(lat time.Duration) {
	r.nSent.Add(1)
	r.nSuccess.Add(1)
	r.latMu.Lock()
	r.latencies = append(r.latencies, lat.Nanoseconds())
	r.latMu.Unlock()
}

func (r *recorder) fail(isTimeout bool) {
	r.nSent.Add(1)
	r.nError.Add(1)
	if isTimeout {
		r.nTimeout.Add(1)
	}
}

func (r *recorder) sent() int64 { return r.nSent.Load() }

// report prints one block: sent/success/error/timeout counts and latency
// percentiles for successful requests.
func (r *recorder) report() {
	sent := r.nSent.Load()
	ok := r.nSuccess.Load()
	tos := r.nTimeout.Load()
	errs := r.nError.Load() - tos

	fmt.Printf("\n--- %s ---\n", r.name)
	if sent == 0 {
		fmt.Printf("  (no requests sent)\n")
		return
	}
	fmt.Printf("  Sent:     %d\n", sent)
	fmt.Printf("  Success:  %d  (%.1f%%)\n", ok, pct(ok, sent))
	fmt.Printf("  Errors:   %d  (%.1f%%)\n", errs, pct(errs, sent))
	fmt.Printf("  Timeouts: %d  (%.1f%%)\n", tos, pct(tos, sent))

	r.latMu.Lock()
	defer r.latMu.Unlock()
	if len(r.latencies) == 0 {
		return
	}
	lat := append([]int64(nil), r.latencies...)
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	n := len(lat)
	fmt.Printf("  Latency (successful):\n")
	fmt.Printf("    p50: %s\n", time.Duration(lat[n*50/100]))
	fmt.Printf("    p95: %s\n", time.Duration(lat[n*95/100]))
	fmt.Printf("    p99: %s\n", time.Duration(lat[n*99/100]))
	fmt.Printf("    max: %s\n", time.Duration(lat[n-1]))
}
