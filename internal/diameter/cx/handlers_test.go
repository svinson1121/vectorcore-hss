package cx

import "testing"

func TestIMSIMSDomainPadsTwoDigitMNC(t *testing.T) {
	got := imsIMSDomain("999", "99")
	want := "ims.mnc099.mcc999.3gppnetwork.org"
	if got != want {
		t.Fatalf("imsIMSDomain() = %q, want %q", got, want)
	}
}
