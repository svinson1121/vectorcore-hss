package slh

import "github.com/fiorix/go-diameter/v4/diam/datatype"

const (
	AppIDSLh     = uint32(16777291)
	Vendor3GPP   = uint32(10415)

	avpServingNode = uint32(2401)
	avpMMEName     = uint32(2402)
	avpMMERealm    = uint32(2408)
	avpSGSNName    = uint32(2409)
	avpSGSNRealm   = uint32(2410)
	avpMSISDN      = uint32(701)
)

// LRR holds the fields we unmarshal from a LCS-Routing-Info-Request.
type LRR struct {
	SessionID        datatype.UTF8String       `avp:"Session-Id"`
	OriginHost       datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	OriginRealm      datatype.DiameterIdentity `avp:"Origin-Realm,omitempty"`
	DestinationRealm datatype.DiameterIdentity `avp:"Destination-Realm,omitempty"`
	UserName         datatype.UTF8String       `avp:"User-Name,omitempty"`
	MSISDN           datatype.OctetString      `avp:"MSISDN,omitempty"`
}
