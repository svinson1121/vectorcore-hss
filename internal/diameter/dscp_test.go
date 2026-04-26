package diameter

import "testing"

func TestDSCPToTOS(t *testing.T) {
	tests := []struct {
		name string
		dscp int
		want int
	}{
		{name: "best effort", dscp: 0, want: 0},
		{name: "af31", dscp: 26, want: 104},
		{name: "ef", dscp: 46, want: 184},
		{name: "max", dscp: 63, want: 252},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dscpToTOS(tt.dscp); got != tt.want {
				t.Fatalf("dscpToTOS(%d) = %d, want %d", tt.dscp, got, tt.want)
			}
		})
	}
}

func TestLegacySCTPFD(t *testing.T) {
	type sctpConn struct {
		_fd int32
	}
	type diamSCTPConn struct {
		*sctpConn
	}
	type sctpListener struct {
		fd int
	}
	type diamSCTPListener struct {
		*sctpListener
	}

	tests := []struct {
		name string
		sock any
		want int
	}{
		{name: "diam sctp conn", sock: &diamSCTPConn{sctpConn: &sctpConn{_fd: 12}}, want: 12},
		{name: "diam sctp listener", sock: &diamSCTPListener{sctpListener: &sctpListener{fd: 34}}, want: 34},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := legacySCTPFD(tt.sock)
			if !ok {
				t.Fatal("legacySCTPFD() did not find fd")
			}
			if got != tt.want {
				t.Fatalf("legacySCTPFD() = %d, want %d", got, tt.want)
			}
		})
	}
}
