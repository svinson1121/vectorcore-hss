package s6a

import "github.com/fiorix/go-diameter/v4/diam/datatype"

const (
	AppIDS6a   = uint32(16777251)
	Vendor3GPP = uint32(10415)

	ULAFlagSeparationIndication = uint32(1 << 0)
	ULAFlagMMERegisteredForSMS  = uint32(1 << 1)

	ULRFlagS6aIndicator      = uint32(1 << 1)
	ULRFlagSMSOnlyIndication = uint32(1 << 7)

	FeatureListIDSMSInMME = uint32(2)
	FeatureBitSMSInMME    = uint32(1 << 0)

	avpMMENumberForMTSMS  = uint32(1645)
	avpSMSRegisterRequest = uint32(1648)

	SMSRegistrationRequired     = int32(0)
	SMSRegistrationNotPreferred = int32(1)
	SMSRegistrationNoPreference = int32(2)
)

type RequestedEUTRANAuthInfo struct {
	NumVectors        datatype.Unsigned32  `avp:"Number-Of-Requested-Vectors,omitempty"`
	ImmediateResponse datatype.Unsigned32  `avp:"Immediate-Response-Preferred,omitempty"`
	ResyncInfo        datatype.OctetString `avp:"Re-synchronization-Info,omitempty"`
}

type AIR struct {
	SessionID               datatype.UTF8String       `avp:"Session-Id"`
	OriginHost              datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	OriginRealm             datatype.DiameterIdentity `avp:"Origin-Realm,omitempty"`
	UserName                string                    `avp:"User-Name"`
	VisitedPLMNID           datatype.OctetString      `avp:"Visited-PLMN-Id,omitempty"`
	RequestedEUTRANAuthInfo *RequestedEUTRANAuthInfo  `avp:"Requested-EUTRAN-Authentication-Info,omitempty"`
	AuthSessionState        int32                     `avp:"Auth-Session-State,omitempty"`
}

type ULR struct {
	SessionID        datatype.UTF8String       `avp:"Session-Id"`
	OriginHost       datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	OriginRealm      datatype.DiameterIdentity `avp:"Origin-Realm,omitempty"`
	UserName         datatype.UTF8String       `avp:"User-Name"`
	VisitedPLMNID    datatype.OctetString      `avp:"Visited-PLMN-Id,omitempty"`
	RATType          datatype.Unsigned32       `avp:"RAT-Type,omitempty"`
	ULRFlags         datatype.Unsigned32       `avp:"ULR-Flags,omitempty"`
	AuthSessionState int32                     `avp:"Auth-Session-State,omitempty"`
	UserLocationInfo datatype.OctetString      `avp:"User-Location-Info,omitempty"`
}

type PUR struct {
	SessionID        datatype.UTF8String       `avp:"Session-Id"`
	OriginHost       datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	OriginRealm      datatype.DiameterIdentity `avp:"Origin-Realm,omitempty"`
	UserName         datatype.UTF8String       `avp:"User-Name"`
	AuthSessionState int32                     `avp:"Auth-Session-State,omitempty"`
}

type AMBR struct {
	MaxRequestedBandwidthDL uint32
	MaxRequestedBandwidthUL uint32
}

type AllocationRetentionPriority struct {
	PriorityLevel           uint32
	PreemptionCapability    int32
	PreemptionVulnerability int32
}

type EPSSubscribedQoSProfile struct {
	QoSClassIdentifier          int32
	AllocationRetentionPriority AllocationRetentionPriority
}

type APNConfiguration struct {
	ContextIdentifier           uint32
	PDNType                     int32
	ServiceSelection            string
	EPSSubscribedQoSProfile     EPSSubscribedQoSProfile
	AMBR                        AMBR
	TGPPChargingCharacteristics string
}

type APNConfigurationProfile struct {
	ContextIdentifier                     uint32
	AllAPNConfigurationsIncludedIndicator int32
	APNConfiguration                      []APNConfiguration
}

type SubscriptionData struct {
	MSISDN                        datatype.OctetString
	AccessRestrictionData         uint32
	SubscriberStatus              int32
	NetworkAccessMode             int32
	AMBR                          AMBR
	APNConfigurationProfile       APNConfigurationProfile
	SubscribedPeriodicRAUTAUTimer uint32
	IMSVoiceOverPSSessions        int32 // 0=NOT_SUPPORTED, 1=SUPPORTED; -1=omit
}
