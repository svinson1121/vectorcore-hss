package peertracker

import (
	"testing"
	"time"
)

func TestTrackerPrunesExpiredEntriesOnList(t *testing.T) {
	tr := NewWithMaxAge(10 * time.Millisecond)
	tr.Add(Peer{Name: "AMF/amf-1 via SCP", RemoteAddr: "192.0.2.10", Transport: "h2c via scp"})
	time.Sleep(25 * time.Millisecond)
	if got := tr.List(); len(got) != 0 {
		t.Fatalf("expected expired peer to be pruned, got %#v", got)
	}
}
