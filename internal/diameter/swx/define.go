package swx

import "github.com/fiorix/go-diameter/v4/diam/datatype"

const (
	AppIDSWx   = uint32(16777265)
	Vendor3GPP = uint32(10415)

	// Non-3GPP-IP-Access values
	Non3GPPAccessAllowed = 0
	Non3GPPAccessBarred  = 1

	// AN-Trusted values
	ANTrusted   = 0
	ANUntrusted = 1

	// AVP codes
	avpConfidentialityKey = uint32(625)
	avpIntegrityKey       = uint32(626)
	avpNon3GPPUserData    = uint32(1500)
	avpNon3GPPIPAccess    = uint32(1501)
	avpANTrusted          = uint32(1503)

	// Shared AVP codes (defined in Cx dict, used here by code)
	avpSIPNumberAuthItems      = uint32(607)
	avpSIPAuthenticationScheme = uint32(608)
	avpSIPAuthenticate         = uint32(609)
	avpSIPAuthorization        = uint32(610)
	avpSIPItemNumber           = uint32(613)
	avpSIPAuthDataItem         = uint32(612)
	avpServerAssignmentType    = uint32(614)

	// Result codes
	DiameterErrorUserUnknown = uint32(5001)
)

type MARSIPAuthDataItem struct {
	SIPAuthenticationScheme datatype.UTF8String `avp:"SIP-Authentication-Scheme,omitempty"`
}

type MAR struct {
	SessionID          datatype.UTF8String       `avp:"Session-Id"`
	OriginHost         datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	OriginRealm        datatype.DiameterIdentity `avp:"Origin-Realm,omitempty"`
	UserName           datatype.UTF8String       `avp:"User-Name"`
	SIPNumberAuthItems datatype.Unsigned32       `avp:"SIP-Number-Auth-Items,omitempty"`
	SIPAuthDataItem    *MARSIPAuthDataItem       `avp:"SIP-Auth-Data-Item,omitempty"`
	AuthSessionState   int32                     `avp:"Auth-Session-State,omitempty"`
}

type SAR struct {
	SessionID            datatype.UTF8String       `avp:"Session-Id"`
	OriginHost           datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	OriginRealm          datatype.DiameterIdentity `avp:"Origin-Realm,omitempty"`
	UserName             datatype.UTF8String       `avp:"User-Name"`
	ServerAssignmentType datatype.Enumerated       `avp:"Server-Assignment-Type,omitempty"`
	AuthSessionState     int32                     `avp:"Auth-Session-State,omitempty"`
}
