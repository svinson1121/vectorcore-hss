package sbi

import "testing"

func TestPeerDisplayNameMatchesSCPByHost(t *testing.T) {
	if got := PeerDisplayName("192.168.105.14:36652", "http://192.168.105.14:7777"); got != "SCP" {
		t.Fatalf("expected SCP label, got %q", got)
	}
}

func TestPeerDisplayNameFallsBackForUnknownPeer(t *testing.T) {
	const remote = "192.168.105.15:36652"
	if got := PeerDisplayName(remote, "http://192.168.105.14:7777"); got != remote {
		t.Fatalf("expected remote fallback, got %q", got)
	}
}

func TestPeerDisplayNameMatchesIPv6SCP(t *testing.T) {
	if got := PeerDisplayName("[2001:db8::1]:36652", "https://[2001:db8::1]:7777"); got != "SCP" {
		t.Fatalf("expected SCP label for IPv6, got %q", got)
	}
}
