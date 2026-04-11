package udm

// suci.go — SUPI/SUCI decoder.
//
// SUPI format:  imsi-<15 digits>           e.g. "imsi-311435000070570"
// SUCI format (null-scheme, scheme 0):
//   suci-0-<MCC>-<MNC>-<RoutingIndicator>-0-0-<MSIN>
//   e.g. "suci-0-311-435-0000-0-0-000070570"
//
// SUCI format (encrypted, scheme 1/2):
//   suci-0-<MCC>-<MNC>-<RoutingIndicator>-<SchemeID>-<KeyID>-<HexSchemeOutput>
//   e.g. "suci-0-311-435-0000-1-1-<64+N+16 hex chars>"
//
// ParseSUPI handles SUPI and null-scheme SUCI without a key store.
// ParseSUPIWithKeys handles all schemes; pass nil keys to restrict to null-scheme.

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// ParseSUPI extracts the IMSI string from a SUPI or null-scheme SUCI.
// Returns an error for encrypted SUCI — use ParseSUPIWithKeys for those.
func ParseSUPI(supi string) (string, error) {
	return ParseSUPIWithKeys(supi, nil)
}

// ParseSUPIWithKeys extracts the IMSI from a SUPI or SUCI of any scheme.
// keys may be nil, in which case only null-scheme (0) is accepted.
func ParseSUPIWithKeys(supi string, keys *HNetKeyStore) (string, error) {
	if strings.HasPrefix(supi, "imsi-") {
		return strings.TrimPrefix(supi, "imsi-"), nil
	}
	if strings.HasPrefix(supi, "suci-") {
		return parseSUCI(supi, keys)
	}
	return "", fmt.Errorf("udm: unrecognised SUPI/SUCI format: %q", supi)
}

// parseSUCI decodes a SUCI into an IMSI string.
// Format: suci-<addrType>-<MCC>-<MNC>-<RoutingIndicator>-<SchemeID>-<KeyID>-<SchemeOutput>
func parseSUCI(suci string, keys *HNetKeyStore) (string, error) {
	parts := strings.SplitN(suci, "-", 8)
	if len(parts) != 8 {
		return "", fmt.Errorf("udm: malformed SUCI %q", suci)
	}

	mcc := parts[2]
	mnc := parts[3]
	schemeID, err := strconv.Atoi(parts[5])
	if err != nil {
		return "", fmt.Errorf("udm: SUCI scheme ID not numeric in %q", suci)
	}
	keyID, err := strconv.Atoi(parts[6])
	if err != nil {
		return "", fmt.Errorf("udm: SUCI key ID not numeric in %q", suci)
	}
	schemeOutputRaw := parts[7]

	if schemeID == 0 {
		// Null scheme — scheme output is the plaintext MSIN.
		return mcc + mnc + schemeOutputRaw, nil
	}

	// Encrypted scheme — need key store.
	if keys == nil {
		return "", fmt.Errorf("udm: encrypted SUCI (scheme %d) received but no HNet keys configured", schemeID)
	}
	schemeOutput, err := hex.DecodeString(schemeOutputRaw)
	if err != nil {
		return "", fmt.Errorf("udm: SUCI scheme output is not valid hex: %w", err)
	}
	msin, err := keys.Decrypt(schemeID, keyID, schemeOutput)
	if err != nil {
		return "", fmt.Errorf("udm: SUCI decryption failed: %w", err)
	}
	return mcc + mnc + msin, nil
}
