package cx

import (
	"fmt"
	"strings"

	"github.com/fiorix/go-diameter/v4/diam/dict"
)

// LoadDict registers the Cx (IMS) application dictionary with the default
// Diameter dictionary parser. Must be called before starting the server.
func LoadDict() error {
	if err := dict.Default.Load(strings.NewReader(dictXML)); err != nil {
		return fmt.Errorf("cx: load dict: %w", err)
	}
	return nil
}

const dictXML = `<?xml version="1.0" encoding="UTF-8"?>
<diameter>
  <application id="16777216" type="auth" name="Cx">
    <vendor id="10415" name="3GPP"/>
    <command code="300" short="UA" name="User-Authorization">
      <request>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="Destination-Realm" required="true" max="1"/>
        <rule avp="User-Name" required="true" max="1"/>
        <rule avp="Public-Identity" required="true" max="1"/>
        <rule avp="Visited-Network-Identifier" required="true" max="1"/>
        <rule avp="User-Authorization-Type" required="false" max="1"/>
      </request>
      <answer>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Result-Code" required="false" max="1"/>
        <rule avp="Experimental-Result" required="false" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="Server-Name" required="false" max="1"/>
        <rule avp="Server-Capabilities" required="false" max="1"/>
      </answer>
    </command>
    <command code="301" short="SA" name="Server-Assignment">
      <request>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="Destination-Realm" required="true" max="1"/>
        <rule avp="User-Name" required="false" max="1"/>
        <rule avp="Public-Identity" required="false"/>
        <rule avp="Server-Name" required="true" max="1"/>
        <rule avp="Server-Assignment-Type" required="true" max="1"/>
      </request>
      <answer>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Result-Code" required="false" max="1"/>
        <rule avp="Experimental-Result" required="false" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="User-Name" required="false" max="1"/>
        <rule avp="User-Data" required="false" max="1"/>
      </answer>
    </command>
    <command code="302" short="LI" name="Location-Info">
      <request>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="Destination-Realm" required="true" max="1"/>
        <rule avp="Public-Identity" required="true" max="1"/>
      </request>
      <answer>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Result-Code" required="false" max="1"/>
        <rule avp="Experimental-Result" required="false" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="Server-Name" required="false" max="1"/>
        <rule avp="Server-Capabilities" required="false" max="1"/>
      </answer>
    </command>
    <command code="303" short="MA" name="Multimedia-Authentication">
      <request>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="Destination-Realm" required="true" max="1"/>
        <rule avp="User-Name" required="true" max="1"/>
        <rule avp="Public-Identity" required="true" max="1"/>
        <rule avp="SIP-Number-Auth-Items" required="true" max="1"/>
        <rule avp="SIP-Auth-Data-Item" required="true" max="1"/>
      </request>
      <answer>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Result-Code" required="false" max="1"/>
        <rule avp="Experimental-Result" required="false" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="User-Name" required="false" max="1"/>
        <rule avp="Public-Identity" required="false" max="1"/>
        <rule avp="SIP-Number-Auth-Items" required="false" max="1"/>
        <rule avp="SIP-Auth-Data-Item" required="false"/>
      </answer>
    </command>
    <command code="304" short="RT" name="Registration-Termination">
      <request>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="Destination-Host" required="true" max="1"/>
        <rule avp="Destination-Realm" required="true" max="1"/>
        <rule avp="User-Name" required="true" max="1"/>
        <rule avp="Deregistration-Reason" required="true" max="1"/>
      </request>
      <answer>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Result-Code" required="false" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
      </answer>
    </command>
    <command code="305" short="PP" name="Push-Profile">
      <request>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="Destination-Host" required="true" max="1"/>
        <rule avp="Destination-Realm" required="true" max="1"/>
        <rule avp="User-Name" required="true" max="1"/>
        <rule avp="User-Data" required="false" max="1"/>
      </request>
      <answer>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Result-Code" required="false" max="1"/>
        <rule avp="Experimental-Result" required="false" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
      </answer>
    </command>
    <avp name="Visited-Network-Identifier" code="600" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="Public-Identity" code="601" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="UTF8String"/>
    </avp>
    <avp name="Server-Name" code="602" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="UTF8String"/>
    </avp>
    <avp name="Server-Capabilities" code="603" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="Mandatory-Capability" required="false"/>
        <rule avp="Optional-Capability" required="false"/>
      </data>
    </avp>
    <avp name="Mandatory-Capability" code="604" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="Optional-Capability" code="605" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="User-Data" code="606" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="SIP-Number-Auth-Items" code="607" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="SIP-Authentication-Scheme" code="608" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="UTF8String"/>
    </avp>
    <avp name="SIP-Authenticate" code="609" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="SIP-Authorization" code="610" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="SIP-Authentication-Context" code="611" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="SIP-Auth-Data-Item" code="612" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="SIP-Item-Number" required="false" max="1"/>
        <rule avp="SIP-Authentication-Scheme" required="false" max="1"/>
        <rule avp="SIP-Authenticate" required="false" max="1"/>
        <rule avp="SIP-Authorization" required="false" max="1"/>
        <rule avp="SIP-Authentication-Context" required="false" max="1"/>
        <rule avp="Confidentiality-Key" required="false" max="1"/>
        <rule avp="Integrity-Key" required="false" max="1"/>
      </data>
    </avp>
    <avp name="SIP-Item-Number" code="613" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="Server-Assignment-Type" code="614" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="NO_ASSIGNMENT"/>
        <item code="1" name="REGISTRATION"/>
        <item code="2" name="RE_REGISTRATION"/>
        <item code="3" name="UNREGISTERED_USER"/>
        <item code="4" name="TIMEOUT_DEREGISTRATION"/>
        <item code="5" name="USER_DEREGISTRATION"/>
        <item code="6" name="TIMEOUT_DEREGISTRATION_STORE_SERVER_NAME"/>
        <item code="7" name="USER_DEREGISTRATION_STORE_SERVER_NAME"/>
        <item code="8" name="ADMINISTRATIVE_DEREGISTRATION"/>
        <item code="9" name="AUTHENTICATION_FAILURE"/>
        <item code="10" name="AUTHENTICATION_TIMEOUT"/>
        <item code="11" name="DEREGISTRATION_TOO_MUCH_DATA"/>
      </data>
    </avp>
    <avp name="Deregistration-Reason" code="615" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="Reason-Code" required="true" max="1"/>
        <rule avp="Reason-Info" required="false" max="1"/>
      </data>
    </avp>
    <avp name="Reason-Code" code="616" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="PERMANENT_TERMINATION"/>
        <item code="1" name="NEW_SERVER_ASSIGNED"/>
        <item code="2" name="SERVER_CHANGE"/>
        <item code="3" name="REMOVE_S-CSCF"/>
      </data>
    </avp>
    <avp name="Reason-Info" code="617" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="UTF8String"/>
    </avp>
    <avp name="Confidentiality-Key" code="625" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="Integrity-Key" code="626" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="User-Authorization-Type" code="623" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="REGISTRATION"/>
        <item code="1" name="DE_REGISTRATION"/>
        <item code="2" name="REGISTRATION_AND_CAPABILITIES"/>
      </data>
    </avp>
  </application>
</diameter>`
