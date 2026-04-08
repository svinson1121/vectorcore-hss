package s13

import "github.com/fiorix/go-diameter/v4/diam/datatype"

const (
	AppIDS13   = uint32(16777252)
	Vendor3GPP = uint32(10415)

	// AVP codes not in the fiorix avp package
	avpEquipmentStatus = uint32(1445)

	// Equipment-Status enumeration values (3GPP TS 29.272 §7.3.51)
	EquipmentWhitelisted = 0
	EquipmentBlacklisted = 1
	EquipmentGreylisted  = 2
)

// ECR is the ME-Identity-Check-Request (S13 interface, command code 324).
type ECR struct {
	SessionID           datatype.UTF8String       `avp:"Session-Id"`
	OriginHost          datatype.DiameterIdentity `avp:"Origin-Host,omitempty"`
	OriginRealm         datatype.DiameterIdentity `avp:"Origin-Realm,omitempty"`
	UserName            datatype.UTF8String       `avp:"User-Name,omitempty"`
	TerminalInformation *TerminalInformation      `avp:"Terminal-Information,omitempty"`
	AuthSessionState    int32                     `avp:"Auth-Session-State,omitempty"`
}

type TerminalInformation struct {
	IMEI            datatype.UTF8String `avp:"IMEI,omitempty"`
	SoftwareVersion datatype.UTF8String `avp:"Software-Version,omitempty"`
}
