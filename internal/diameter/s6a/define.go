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

	avpAlertReason               = uint32(1434)
	avpMMENumberForMTSMS         = uint32(1645)
	avpSMSRegisterRequest        = uint32(1648)
	avpMaximumUEAvailabilityTime = uint32(3329)

	// CIoT/NIDD APN-Configuration AVPs (3GPP TS 29.272 §7.3.204-209, §7.3.222;
	// SCEF-ID per TS 29.336). Not present in go-diameter's avp package, so
	// defined locally like the other 3GPP AVPs above. All use the V-bit only
	// (M-bit must not be set) per their AVP flag rules.
	avpNonIPPDNTypeIndicator      = uint32(1681)
	avpNonIPDataDeliveryMechanism = uint32(1682)
	avpSCEFRealm                  = uint32(1684)
	avpPreferredDataMode          = uint32(1686)
	avpRDSIndicator               = uint32(1697)
	avpSCEFID                     = uint32(3125)

	SMSRegistrationRequired     = int32(0)
	SMSRegistrationNotPreferred = int32(1)
	SMSRegistrationNoPreference = int32(2)

	AlertReasonUEPresent         = int32(0)
	AlertReasonUEMemoryAvailable = int32(1)
)

type AlertTrigger string

const (
	AlertTriggerAttach          AlertTrigger = "attach"
	AlertTriggerUserAvailable   AlertTrigger = "user_available"
	AlertTriggerMemoryAvailable AlertTrigger = "memory_available"
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
	// ServedPartyIPAddress holds an optional static UE IPv4/IPv6 address for
	// this APN (3GPP TS 29.272 §7.3.5, AVP 848). Empty means dynamic
	// allocation; the AVP is then omitted.
	ServedPartyIPAddress string

	// CIoT/NIDD fields (TS 29.272 §7.3.204-209, §7.3.222). All are only
	// populated when the APN has CIoT features enabled; see
	// buildSubscriptionData in ulr.go and appendSubscriptionDataAVPs in
	// subdata.go for the emission/presence rules.
	NonIPPDN          bool  // Non-IP-PDN-Type-Indicator; only set when true (omitted otherwise, default FALSE)
	NIDDMechanism     int32 // Non-IP-Data-Delivery-Mechanism; only emitted when NonIPPDN is true
	SCEFID            string
	SCEFRealm         string
	RDSSet            bool // whether RDS-Indicator should be emitted at all
	RDSIndicator      int32
	PreferredDataMode uint32 // Preferred-Data-Mode bitmask; 0 = omit
}

type APNConfigurationProfile struct {
	ContextIdentifier                     uint32
	AllAPNConfigurationsIncludedIndicator int32
	APNConfiguration                      []APNConfiguration
}

type SubscriptionData struct {
	MSISDN                datatype.OctetString
	AccessRestrictionData uint32
	SubscriberStatus      int32
	// OperatorDeterminedBarring is the Operator-Determined-Barring AVP bitmask
	// (TS 29.272 §7.3.30). Only emitted when SubscriberStatus indicates
	// OPERATOR_DETERMINED_BARRING; see appendSubscriptionDataAVPs.
	OperatorDeterminedBarring     uint32
	NetworkAccessMode             int32
	AMBR                          AMBR
	APNConfigurationProfile       APNConfigurationProfile
	SubscribedPeriodicRAUTAUTimer uint32
	IMSVoiceOverPSSessions        int32 // 0=NOT_SUPPORTED, 1=SUPPORTED; -1=omit
}
