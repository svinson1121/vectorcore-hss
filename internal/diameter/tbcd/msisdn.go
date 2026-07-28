// Package tbcd implements the TBCD form used by 3GPP MSISDN AVPs.
package tbcd

import "fmt"

// EncodeMSISDN encodes decimal MSISDN digits as TBCD.  Odd length values use
// the only permitted filler: the high nibble of the final octet (0xf).
func EncodeMSISDN(number string) ([]byte, error) {
	if number == "" {
		return nil, fmt.Errorf("empty MSISDN")
	}
	out := make([]byte, (len(number)+1)/2)
	for i := 0; i < len(number); i += 2 {
		lo, err := digit(number[i])
		if err != nil {
			return nil, err
		}
		hi := byte(0xf)
		if i+1 < len(number) {
			hi, err = digit(number[i+1])
			if err != nil {
				return nil, err
			}
		}
		out[i/2] = lo | hi<<4
	}
	return out, nil
}

// DecodeMSISDN decodes a bare TBCD MSISDN and rejects non-decimal nibbles,
// filler in any position other than the final high nibble, and empty input.
func DecodeMSISDN(b []byte) (string, error) {
	if len(b) == 0 {
		return "", fmt.Errorf("empty TBCD MSISDN")
	}
	out := make([]byte, 0, len(b)*2)
	for i, octet := range b {
		lo, hi := octet&0x0f, octet>>4
		if lo > 9 {
			return "", fmt.Errorf("invalid TBCD low nibble %#x", lo)
		}
		out = append(out, '0'+lo)
		if hi == 0xf {
			if i != len(b)-1 {
				return "", fmt.Errorf("TBCD filler before final octet")
			}
			continue
		}
		if hi > 9 {
			return "", fmt.Errorf("invalid TBCD high nibble %#x", hi)
		}
		out = append(out, '0'+hi)
	}
	return string(out), nil
}

func digit(c byte) (byte, error) {
	if c < '0' || c > '9' {
		return 0, fmt.Errorf("invalid MSISDN digit %q", c)
	}
	return c - '0', nil
}
