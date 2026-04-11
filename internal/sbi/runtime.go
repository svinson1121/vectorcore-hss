package sbi

import (
	"crypto/rand"
	"fmt"
	"net"
)

// NewNFInstanceID returns a random RFC 4122 version 4 UUID string.
func NewNFInstanceID() string {
	var b [16]byte
	rand.Read(b[:]) //nolint:errcheck // crypto/rand.Read is expected to succeed on supported platforms
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ResolveOurIP picks the local source IP used to reach the wider network.
func ResolveOurIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

// SchemeFromTLS returns the SBI scheme implied by the TLS settings.
func SchemeFromTLS(certFile string) string {
	if certFile != "" {
		return "https"
	}
	return "http"
}
