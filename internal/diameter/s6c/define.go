package s6c

import "github.com/fiorix/go-diameter/v4/diam/datatype"

const (
	AppIDS6c   = uint32(16777312)
	Vendor3GPP = uint32(10415)

	// Command codes (TS 29.338 §6.1)
	cmdSRISM = uint32(8388647)
	cmdALSC  = uint32(8388648)
	cmdRDSM  = uint32(8388649)

	// AVP codes shared with SLh (already loaded from slh/dict.go)
	avpMSISDN             = uint32(701)
	avpLMSI               = uint32(2400)
	avpServingNode        = uint32(2401)
	avpMMEName            = uint32(2402)
	avpMMENumberForMTSMS  = uint32(2403)
	avpAdditionalServing  = uint32(2406)
	avpMMERealm           = uint32(2408)
	avpSGSNName           = uint32(2409)
	avpSGSNRealm          = uint32(2410)

	// S6c-specific AVP codes (TS 29.338 §7.3)
	avpSCAddress                  = uint32(3300)
	avpSMRPMTI                    = uint32(3308)
	avpMWDStatus                  = uint32(3312)
	avpMMEAbsentUserDiagnosticSM  = uint32(3313)
	avpSMDeliveryOutcome          = uint32(3316)
	avpMMEDeliveryOutcome         = uint32(3317)
	avpSGSNDeliveryOutcome        = uint32(3318)
	avpSMDeliveryCause            = uint32(3321)
	avpAbsentUserDiagnosticSM     = uint32(3322)
	avpIPSMGWNumber               = uint32(3327)
	avpIPSMGWName                 = uint32(3328)
	avpIPSMGWRealm                = uint32(3329)

	// MWD-Status bitmask values (TS 29.338 §7.3.12)
	MWDStatusMNRF = uint32(0x02) // Mobile Not Reachable Flag
	MWDStatusMCEF = uint32(0x04) // Memory Capacity Exceeded Flag
	MWDStatusMNRG = uint32(0x08) // Mobile Not Reachable for GPRS

	// SM-Delivery-Cause values (TS 29.338 §7.3.21)
	SMDeliveryCauseMemoryCapacityExceeded = int32(0)
	SMDeliveryCauseAbsentUser             = int32(1)
	SMDeliveryCauseSuccessfulTransfer     = int32(2)
)

// SRISR holds the fields we unmarshal from a Send-Routing-Info-for-SM-Request.
type SRISR struct {
	SessionID        datatype.UTF8String       `avp:"Session-Id"`
	OriginHost       datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	OriginRealm      datatype.DiameterIdentity `avp:"Origin-Realm,omitempty"`
	DestinationRealm datatype.DiameterIdentity `avp:"Destination-Realm,omitempty"`
	UserName         datatype.UTF8String       `avp:"User-Name,omitempty"`
	MSISDN           datatype.OctetString      `avp:"MSISDN,omitempty"`
	SMRPMTI          datatype.Enumerated       `avp:"SM-RP-MTI,omitempty"`
	SCAddress        datatype.OctetString      `avp:"SC-Address,omitempty"`
}

// RDR holds the fields we unmarshal from a Report-SM-Delivery-Status-Request.
// SM-Delivery-Outcome is a vendor-specific grouped AVP; it is parsed manually
// via FindAVP after struct unmarshal (go-diameter does not reliably decode
// vendor-specific grouped AVPs via struct tags).
type RDR struct {
	SessionID        datatype.UTF8String       `avp:"Session-Id"`
	OriginHost       datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	OriginRealm      datatype.DiameterIdentity `avp:"Origin-Realm,omitempty"`
	DestinationRealm datatype.DiameterIdentity `avp:"Destination-Realm,omitempty"`
	UserName         datatype.UTF8String       `avp:"User-Name,omitempty"`
	MSISDN           datatype.OctetString      `avp:"MSISDN,omitempty"`
	SCAddress        datatype.OctetString      `avp:"SC-Address"`
	SMRPMTI          datatype.Enumerated       `avp:"SM-RP-MTI,omitempty"`
}

// SMDeliveryOutcomeResult is the parsed result of an SM-Delivery-Outcome grouped AVP.
type SMDeliveryOutcomeResult struct {
	// Cause is the SM-Delivery-Cause value from whichever node sub-AVP was present.
	// -1 means the AVP was absent from the request (treat as AbsentUser).
	Cause int32
	// AbsentUserDiagnostic is the optional diagnostic code from the sub-AVP.
	AbsentUserDiagnostic uint32
}

// ASA holds fields from an Alert-Service-Centre-Answer (reply to our ALSC request).
type ASA struct {
	OriginHost datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	ResultCode datatype.Unsigned32       `avp:"Result-Code,omitempty"`
}
