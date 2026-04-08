package cx

import "github.com/fiorix/go-diameter/v4/diam/datatype"

const (
	AppIDCx    = uint32(16777216)
	Vendor3GPP = uint32(10415)

	// Cx success Experimental-Result-Codes (3GPP TS 29.229 §6.2)
	DiameterFirstRegistration      = uint32(2001)
	DiameterSubsequentRegistration = uint32(2002)

	// Cx error result codes (3GPP TS 29.229)
	DiameterErrorUserUnknown           = uint32(5001)
	DiameterErrorIdentitiesNotMatch    = uint32(5002)
	DiameterErrorRoamingNotAllowed     = uint32(5004)
	DiameterErrorIdentityNotRegistered = uint32(5003)
	DiameterErrorServerSelection       = uint32(5005)
	DiameterErrorAuthSchemeNotSupported = uint32(5013)

	// User-Authorization-Type
	UATRegistration               = 0
	UATDeRegistration             = 1
	UATRegistrationAndCapabilities = 2

	// Server-Assignment-Type
	SATNoAssignment             = 0
	SATRegistration             = 1
	SATReRegistration           = 2
	SATUnregisteredUser         = 3
	SATTimeoutDeregistration    = 4
	SATUserDeregistration       = 5
	SATAdministrativeDeregistration = 8

	// AVP codes
	avpPublicIdentity           = uint32(601)
	avpVisitedNetworkIdentifier = uint32(600)
	avpServerName               = uint32(602)
	avpServerCapabilities       = uint32(603)
	avpMandatoryCapability      = uint32(604)
	avpOptionalCapability       = uint32(605)
	avpUserData                 = uint32(606)
	avpSIPNumberAuthItems       = uint32(607)
	avpSIPAuthenticationScheme  = uint32(608)
	avpSIPAuthenticate          = uint32(609)
	avpSIPAuthorization         = uint32(610)
	avpSIPAuthDataItem          = uint32(612)
	avpSIPItemNumber            = uint32(613)
	avpServerAssignmentType     = uint32(614)
	avpDeregistrationReason     = uint32(615)
	avpReasonCode               = uint32(616)
	avpReasonInfo               = uint32(617)
	avpUserAuthorizationType    = uint32(623)
	avpConfidentialityKey       = uint32(625)
	avpIntegrityKey             = uint32(626)
)

// UAR request struct
type UAR struct {
	SessionID             datatype.UTF8String       `avp:"Session-Id"`
	OriginHost            datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	OriginRealm           datatype.DiameterIdentity `avp:"Origin-Realm,omitempty"`
	UserName              datatype.UTF8String       `avp:"User-Name,omitempty"`
	PublicIdentity        datatype.UTF8String       `avp:"Public-Identity,omitempty"`
	VisitedNetworkID      datatype.OctetString      `avp:"Visited-Network-Identifier,omitempty"`
	UserAuthorizationType datatype.Enumerated       `avp:"User-Authorization-Type,omitempty"`
	AuthSessionState      int32                     `avp:"Auth-Session-State,omitempty"`
}

// SAR request struct
type SAR struct {
	SessionID            datatype.UTF8String       `avp:"Session-Id"`
	OriginHost           datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	OriginRealm          datatype.DiameterIdentity `avp:"Origin-Realm,omitempty"`
	UserName             datatype.UTF8String       `avp:"User-Name,omitempty"`
	PublicIdentity       datatype.UTF8String       `avp:"Public-Identity,omitempty"`
	ServerName           datatype.UTF8String       `avp:"Server-Name,omitempty"`
	ServerAssignmentType datatype.Enumerated       `avp:"Server-Assignment-Type,omitempty"`
	AuthSessionState     int32                     `avp:"Auth-Session-State,omitempty"`
}

// LIR request struct
type LIR struct {
	SessionID        datatype.UTF8String       `avp:"Session-Id"`
	OriginHost       datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	OriginRealm      datatype.DiameterIdentity `avp:"Origin-Realm,omitempty"`
	PublicIdentity   datatype.UTF8String       `avp:"Public-Identity,omitempty"`
	AuthSessionState int32                     `avp:"Auth-Session-State,omitempty"`
}

// MAR request struct
type MARSIPAuthDataItem struct {
	SIPAuthenticationScheme datatype.UTF8String `avp:"SIP-Authentication-Scheme,omitempty"`
}

type MAR struct {
	SessionID          datatype.UTF8String       `avp:"Session-Id"`
	OriginHost         datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	OriginRealm        datatype.DiameterIdentity `avp:"Origin-Realm,omitempty"`
	UserName           datatype.UTF8String       `avp:"User-Name,omitempty"`
	PublicIdentity     datatype.UTF8String       `avp:"Public-Identity,omitempty"`
	SIPNumberAuthItems datatype.Unsigned32       `avp:"SIP-Number-Auth-Items,omitempty"`
	SIPAuthDataItem    *MARSIPAuthDataItem       `avp:"SIP-Auth-Data-Item,omitempty"`
	AuthSessionState   int32                     `avp:"Auth-Session-State,omitempty"`
}
