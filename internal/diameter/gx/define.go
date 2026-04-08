package gx

import "github.com/fiorix/go-diameter/v4/diam/datatype"

const (
	AppIDGx    = uint32(16777238)
	Vendor3GPP = uint32(10415)

	// CC-Request-Type values (RFC 4006)
	CCRequestTypeInitial     = 1
	CCRequestTypeUpdate      = 2
	CCRequestTypeTermination = 3

	// Subscription-Id-Type values
	SubscriptionIDTypeIMSI   = 1
	SubscriptionIDTypeMSISDN = 0

	// 3GPP Gx AVP codes (vendor 10415)
	avpCCRequestType            = uint32(416)
	avpCCRequestNumber          = uint32(415)
	avpChargingRuleInstall      = uint32(1001)
	avpChargingRuleRemove       = uint32(1002)
	avpChargingRuleDefinition   = uint32(1003)
	avpChargingRuleName         = uint32(1005)
	avpEventTrigger             = uint32(1006)
	avpQoSInformation           = uint32(1016)
	avpBearerControlMode        = uint32(1023)
	avpNetworkRequestSupport    = uint32(1024)
	avpGuaranteedBitrateDL      = uint32(1025)
	avpGuaranteedBitrateUL      = uint32(1026)
	avpQoSClassIdentifier       = uint32(1028)
	avpAllocationRetentionPri   = uint32(1034)
	avpAPNAggMaxBRDL            = uint32(1040)
	avpAPNAggMaxBRUL            = uint32(1041)
	avpPriorityLevel            = uint32(1046)
	avpPreemptionCapability     = uint32(1047)
	avpPreemptionVulnerability  = uint32(1048)
	avpDefaultEPSBearerQoS      = uint32(1049)
	avpPrecedence               = uint32(1010)
	avpMaxReqBWDL               = uint32(515)
	avpMaxReqBWUL               = uint32(516)
	avpFlowInformation          = uint32(1058)
	avpFlowDirection            = uint32(1080)
	avpFlowDescription          = uint32(507)
	avpFlowStatus               = uint32(511)
)

// CCR is the parsed Credit-Control-Request from a PGW.
type CCR struct {
	SessionID       datatype.UTF8String       `avp:"Session-Id"`
	OriginHost      datatype.DiameterIdentity `avp:"Origin-Host"`
	OriginRealm     datatype.DiameterIdentity `avp:"Origin-Realm"`
	CCRequestType   datatype.Enumerated       `avp:"CC-Request-Type"`
	CCRequestNumber datatype.Unsigned32       `avp:"CC-Request-Number"`
	SubscriptionIDs []SubscriptionID          `avp:"Subscription-Id"`
	CalledStationID datatype.UTF8String       `avp:"Called-Station-Id"`
	FramedIPAddress datatype.OctetString      `avp:"Framed-IP-Address"`
	RATType         datatype.Unsigned32       `avp:"RAT-Type,omitempty"`
}

type SubscriptionID struct {
	Type datatype.Enumerated `avp:"Subscription-Id-Type"`
	Data datatype.UTF8String `avp:"Subscription-Id-Data"`
}
