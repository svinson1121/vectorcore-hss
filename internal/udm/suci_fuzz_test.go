package udm

import "testing"

func FuzzParseSUPI(f *testing.F) {
	f.Add("imsi-311435000070570")
	f.Add("imsi-001010000000001")
	f.Add("suci-0-311-435-0000-0-0-000070570")
	f.Add("suci-0-001-01-0-0-0-0000000001")
	f.Add("imsi-")

	f.Fuzz(func(t *testing.T, input string) {
		imsi, err := ParseSUPI(input)
		if err != nil {
			return
		}
		if len(imsi) < 5 || len(imsi) > 15 {
			t.Fatalf("successful parse returned IMSI with invalid length %d: input %q, IMSI %q", len(imsi), input, imsi)
		}
		for _, c := range imsi {
			if c < '0' || c > '9' {
				t.Fatalf("successful parse returned non-decimal IMSI: input %q, IMSI %q", input, imsi)
			}
		}
	})
}
