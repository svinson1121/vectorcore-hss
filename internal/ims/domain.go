package ims

import "fmt"

// NormalizeMNC formats an MNC for use in 3GPP domain names.
func NormalizeMNC(mnc string) string {
	if len(mnc) == 2 {
		return "0" + mnc
	}
	return mnc
}

func imsDomain(mcc, mnc string) string {
	return fmt.Sprintf("ims.mnc%s.mcc%s.3gppnetwork.org", NormalizeMNC(mnc), mcc)
}
