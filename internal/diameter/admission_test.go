package diameter

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestAdmissionCapsConcurrency proves that at most n callers can hold a
// slot at once — additional callers block until one is released.
func TestAdmissionCapsConcurrency(t *testing.T) {
	const n = 3
	a := newAdmission("test", n)

	var inFlight int32
	var maxObserved int32
	done := make(chan struct{})

	// Launch more concurrent callers than slots available.
	for i := 0; i < n*4; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if !a.acquire(ctx) {
				t.Errorf("acquire failed unexpectedly")
				done <- struct{}{}
				return
			}
			cur := atomic.AddInt32(&inFlight, 1)
			for {
				old := atomic.LoadInt32(&maxObserved)
				if cur <= old || atomic.CompareAndSwapInt32(&maxObserved, old, cur) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
			a.release()
			done <- struct{}{}
		}()
	}

	for i := 0; i < n*4; i++ {
		<-done
	}

	if maxObserved > n {
		t.Errorf("observed %d concurrent holders, want <= %d", maxObserved, n)
	}
}

// TestAdmissionAcquireTimesOut proves a caller that can't get a slot before
// its context deadline gets a clean failure rather than hanging forever.
func TestAdmissionAcquireTimesOut(t *testing.T) {
	a := newAdmission("test", 1)

	ctx := context.Background()
	if !a.acquire(ctx) {
		t.Fatal("expected first acquire to succeed")
	}
	defer a.release()

	acquireCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	ok := a.acquire(acquireCtx)
	elapsed := time.Since(start)

	if ok {
		t.Fatal("expected second acquire to fail while the only slot is held")
	}
	if elapsed > time.Second {
		t.Errorf("acquire took %v to fail, want it to respect the ~50ms deadline", elapsed)
	}
}

// TestAdmissionIndependence proves that exhausting one application's
// semaphore does not block another application's semaphore — this is what
// keeps a Gx CCR-I able to run even while S6a's slots are all in use.
func TestAdmissionIndependence(t *testing.T) {
	s6a := newAdmission("s6a", 1)
	gx := newAdmission("gx", 1)

	ctx := context.Background()
	if !s6a.acquire(ctx) {
		t.Fatal("expected s6a acquire to succeed")
	}
	defer s6a.release()

	gxCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if !gx.acquire(gxCtx) {
		t.Fatal("expected gx acquire to succeed independently of s6a's exhausted semaphore")
	}
	gx.release()
}
