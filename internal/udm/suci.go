package udm

// suci.go — SUPI/SUCI decoder.
//
// SUPI format:  imsi-<15 digits>           e.g. "imsi-311435000070570"
// SUCI format (null-scheme):
//   suci-0-<MCC>-<MNC>-<RoutingIndicator>-<SchemeID>-<KeyID>-<MSIN>
//   e.g. "suci-0-311-435-0-0-0-000070570"
//
// Phase 1: null-scheme (0) only.  ECIES (profile-A/B) is Phase 2.

import (
	"fmt"
	"strings"
)

// ParseSUPI extracts the IMSI string from a SUPI or null-scheme SUCI.
// Returns an error for encrypted SUCI (non-null scheme).
func ParseSUPI(supi string) (string, error) {
	if strings.HasPrefix(supi, "imsi-") {
		return strings.TrimPrefix(supi, "imsi-"), nil
	}
	if strings.HasPrefix(supi, "suci-") {
		return parseSUCI(supi)
	}
	return "", fmt.Errorf("udm: unrecognised SUPI/SUCI format: %q", supi)
}

// parseSUCI decodes a null-scheme SUCI into an IMSI string.
func parseSUCI(suci string) (string, error) {
	// suci-<addrIndicator>-<MCC>-<MNC>-<RI>-<SchemeID>-<KeyID>-<SchemeOutput>
	parts := strings.SplitN(suci, "-", 8)
	if len(parts) != 8 {
		return "", fmt.Errorf("udm: malformed SUCI %q", suci)
	}
	schemeID := parts[5]
	if schemeID != "0" {
		return "", fmt.Errorf("udm: encrypted SUCI (scheme %s) not yet supported", schemeID)
	}
	mcc := parts[2]
	mnc := parts[3]
	msin := parts[7]
	return mcc + mnc + msin, nil
}
