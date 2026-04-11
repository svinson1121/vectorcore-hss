package sbi

import (
	"net"
	"net/url"
	"strings"
)

// PeerDisplayName returns a friendly peer label for known SBI peers such as
// the configured SCP. Unknown peers fall back to their remote address.
func PeerDisplayName(remoteAddr, scpAddress string) string {
	if matchesHost(remoteAddr, scpAddress) {
		return "SCP"
	}
	return remoteAddr
}

func matchesHost(remoteAddr, rawURL string) bool {
	if strings.TrimSpace(remoteAddr) == "" || strings.TrimSpace(rawURL) == "" {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}
	remoteHost := hostOnly(remoteAddr)
	targetHost := hostOnly(u.Host)
	return remoteHost != "" && targetHost != "" && strings.EqualFold(remoteHost, targetHost)
}

func hostOnly(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(addr, "[]")
}
