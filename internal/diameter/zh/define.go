package zh

import "github.com/fiorix/go-diameter/v4/diam/datatype"

const (
	Vendor3GPP = uint32(10415)

	// SIP AVP codes — same codes as Cx/SWx (already registered by SWx dict).
	avpSIPNumberAuthItems      = uint32(607)
	avpSIPAuthenticationScheme = uint32(608)
	avpSIPAuthenticate         = uint32(609)
	avpSIPAuthorization        = uint32(610)
	avpSIPAuthDataItem         = uint32(612)
	avpSIPItemNumber           = uint32(613)
	avpConfidentialityKey      = uint32(625)
	avpIntegrityKey            = uint32(626)

	// GBA authentication scheme (3GPP TS 33.220).
	schemeDigestAKAv1 = "Digest-AKAv1-MD5"
)

// MAR is the parsed Multimedia-Auth-Request from the BSF.
type MAR struct {
	SessionID   datatype.UTF8String       `avp:"Session-Id"`
	OriginHost  datatype.DiameterIdentity `avp:"Origin-Host"`
	OriginRealm datatype.DiameterIdentity `avp:"Origin-Realm"`
	// User-Name carries the IMPI, typically in NAI format: IMSI@realm.
	UserName datatype.UTF8String `avp:"User-Name"`
}
