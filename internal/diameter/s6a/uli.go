package s6a

import "fmt"

// ULIFields holds the decoded values from a 3GPP User-Location-Info AVP.
// Fields are empty strings when the corresponding location type was not present.
type ULIFields struct {
	MCC      string // e.g. "311"
	MNC      string // e.g. "435"
	TAC      string // Tracking Area Code, decimal
	ENodeBID string // eNodeB ID, decimal
	CellID   string // Cell Identity (CI), decimal
	ECI      string // Full E-UTRAN Cell Identity (28-bit), decimal
}

// parseULI decodes the binary User-Location-Info AVP (3GPP TS 29.274 §8.22).
//
// Byte layout:
//
//	[0]       flags  — bitmask of which location types follow
//	[1..5]    TAI    — present when bit 3 (0x08) is set: 3-byte PLMN + 2-byte TAC
//	[next..7] ECGI   — present when bit 4 (0x10) is set: 3-byte PLMN + 4-byte ECI
//
// CGI/SAI/RAI/LAI/Macro-eNodeB types are parsed and skipped so the offsets
// remain correct, but their values are not returned.
func parseULI(b []byte) (ULIFields, bool) {
	if len(b) < 1 {
		return ULIFields{}, false
	}

	flags := b[0]
	pos := 1
	var f ULIFields

	consume := func(n int) ([]byte, bool) {
		if pos+n > len(b) {
			return nil, false
		}
		chunk := b[pos : pos+n]
		pos += n
		return chunk, true
	}

	// CGI (7 bytes): skip
	if flags&0x01 != 0 {
		if _, ok := consume(7); !ok {
			return f, false
		}
	}
	// SAI (7 bytes): skip
	if flags&0x02 != 0 {
		if _, ok := consume(7); !ok {
			return f, false
		}
	}
	// RAI (7 bytes): skip
	if flags&0x04 != 0 {
		if _, ok := consume(7); !ok {
			return f, false
		}
	}
	// TAI (5 bytes): PLMN(3) + TAC(2)
	if flags&0x08 != 0 {
		tai, ok := consume(5)
		if !ok {
			return f, false
		}
		mcc, mnc := decodePLMN(tai[0:3])
		tac := uint16(tai[3])<<8 | uint16(tai[4])
		f.MCC = mcc
		f.MNC = mnc
		f.TAC = fmt.Sprintf("%d", tac)
	}
	// ECGI (7 bytes): PLMN(3) + ECI(4)
	if flags&0x10 != 0 {
		ecgi, ok := consume(7)
		if !ok {
			return f, false
		}
		mcc, mnc := decodePLMN(ecgi[0:3])
		// ECI is 28 bits — top 4 bits of first byte are spare.
		eci := (uint32(ecgi[3]&0x0F) << 24) |
			(uint32(ecgi[4]) << 16) |
			(uint32(ecgi[5]) << 8) |
			uint32(ecgi[6])
		eNodeBID := eci >> 8       // top 20 bits
		cellID := eci & 0xFF       // bottom 8 bits
		// Prefer MCC/MNC from ECGI when TAI was not present.
		if f.MCC == "" {
			f.MCC = mcc
			f.MNC = mnc
		}
		f.ECI = fmt.Sprintf("%d", eci)
		f.ENodeBID = fmt.Sprintf("%d", eNodeBID)
		f.CellID = fmt.Sprintf("%d", cellID)
	}

	if f.MCC == "" && f.TAC == "" && f.ENodeBID == "" {
		return f, false
	}
	return f, true
}

// decodePLMN decodes a 3-byte 3GPP PLMN-ID into MCC and MNC strings.
//
//	Octet 1: MCC digit 2 (high) | MCC digit 1 (low)
//	Octet 2: MNC digit 3 (high) | MCC digit 3 (low)
//	Octet 3: MNC digit 2 (high) | MNC digit 1 (low)
func decodePLMN(p []byte) (mcc, mnc string) {
	mcc1 := p[0] & 0x0F
	mcc2 := (p[0] >> 4) & 0x0F
	mcc3 := p[1] & 0x0F
	mnc3 := (p[1] >> 4) & 0x0F
	mnc1 := p[2] & 0x0F
	mnc2 := (p[2] >> 4) & 0x0F

	mcc = fmt.Sprintf("%d%d%d", mcc1, mcc2, mcc3)
	if mnc3 == 0xF {
		mnc = fmt.Sprintf("%d%d", mnc1, mnc2)
	} else {
		mnc = fmt.Sprintf("%d%d%d", mnc1, mnc2, mnc3)
	}
	return
}
