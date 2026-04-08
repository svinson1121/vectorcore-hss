package rx

import (
	"fmt"
	"strings"

	"github.com/fiorix/go-diameter/v4/diam/dict"
)

const AppIDRx = uint32(16777236)

func LoadDict() error {
	if err := dict.Default.Load(strings.NewReader(dictXML)); err != nil {
		return fmt.Errorf("rx: load dict: %w", err)
	}
	return nil
}

// dictXML declares the 3GPP Rx application (16777236) and its vendor-specific
// AVPs. Command 265 (AA) is declared here as it is not in the base RFC 6733
// dict. AVPs 515 and 516 (Max-Requested-Bandwidth-DL/UL) are omitted because
// they are already registered by the Gx dict loader.
const dictXML = `<?xml version="1.0" encoding="UTF-8"?>
<diameter>
  <application id="16777236" type="auth" name="Rx">
    <vendor id="10415" name="3GPP"/>

    <command code="265" short="AA" name="AA">
      <request>
        <rule avp="Session-Id"                     required="true"  max="1"/>
        <rule avp="Auth-Application-Id"            required="false" max="1"/>
        <rule avp="Vendor-Specific-Application-Id" required="false" max="1"/>
        <rule avp="Origin-Host"                    required="true"  max="1"/>
        <rule avp="Origin-Realm"                   required="true"  max="1"/>
        <rule avp="Destination-Realm"              required="true"  max="1"/>
        <rule avp="Destination-Host"               required="false" max="1"/>
        <rule avp="AF-Application-Identifier"      required="false" max="1"/>
        <rule avp="Media-Component-Description"    required="false"/>
        <rule avp="Service-URN"                    required="false" max="1"/>
        <rule avp="Subscription-Id"                required="false"/>
        <rule avp="Supported-Features"             required="false"/>
        <rule avp="Rx-Request-Type"                required="false" max="1"/>
        <rule avp="Specific-Action"                required="false"/>
        <rule avp="Service-Info-Status"            required="false" max="1"/>
        <rule avp="Origin-State-Id"                required="false" max="1"/>
        <rule avp="Proxy-Info"                     required="false"/>
        <rule avp="Route-Record"                   required="false"/>
      </request>
      <answer>
        <rule avp="Session-Id"                     required="true"  max="1"/>
        <rule avp="Auth-Application-Id"            required="false" max="1"/>
        <rule avp="Result-Code"                    required="false" max="1"/>
        <rule avp="Experimental-Result"            required="false" max="1"/>
        <rule avp="Origin-Host"                    required="true"  max="1"/>
        <rule avp="Origin-Realm"                   required="true"  max="1"/>
        <rule avp="Supported-Features"             required="false"/>
        <rule avp="Origin-State-Id"                required="false" max="1"/>
        <rule avp="Error-Message"                  required="false" max="1"/>
        <rule avp="Failed-AVP"                     required="false"/>
        <rule avp="Proxy-Info"                     required="false"/>
      </answer>
    </command>

    <command code="275" short="ST" name="Session-Termination">
      <request>
        <rule avp="Session-Id"                     required="true"  max="1"/>
        <rule avp="Origin-Host"                    required="true"  max="1"/>
        <rule avp="Origin-Realm"                   required="true"  max="1"/>
        <rule avp="Destination-Realm"              required="true"  max="1"/>
        <rule avp="Auth-Application-Id"            required="true"  max="1"/>
        <rule avp="Termination-Cause"              required="true"  max="1"/>
        <rule avp="Destination-Host"               required="false" max="1"/>
        <rule avp="Subscription-Id"                required="false"/>
        <rule avp="Origin-State-Id"                required="false" max="1"/>
        <rule avp="Proxy-Info"                     required="false"/>
        <rule avp="Route-Record"                   required="false"/>
      </request>
      <answer>
        <rule avp="Session-Id"                     required="true"  max="1"/>
        <rule avp="Result-Code"                    required="true"  max="1"/>
        <rule avp="Origin-Host"                    required="true"  max="1"/>
        <rule avp="Origin-Realm"                   required="true"  max="1"/>
        <rule avp="Error-Message"                  required="false" max="1"/>
        <rule avp="Failed-AVP"                     required="false"/>
        <rule avp="Origin-State-Id"                required="false" max="1"/>
        <rule avp="Proxy-Info"                     required="false"/>
      </answer>
    </command>

    <!-- RFC 4006 Credit Control AVPs used in Rx AAR (not in base RFC 6733 dict,
         and Rx app 16777236 has no parent-app chain in go-diameter, so we
         must declare them here for unmarshal to succeed.) -->
    <avp name="Subscription-Id" code="443" must="M" may="P" must-not="V" may-encrypt="Y">
      <data type="Grouped">
        <rule avp="Subscription-Id-Type" required="true"  max="1"/>
        <rule avp="Subscription-Id-Data" required="true"  max="1"/>
      </data>
    </avp>
    <avp name="Subscription-Id-Data" code="444" must="M" may="P" must-not="V" may-encrypt="Y">
      <data type="UTF8String"/>
    </avp>
    <avp name="Subscription-Id-Type" code="450" must="M" may="P" must-not="V" may-encrypt="Y">
      <data type="Enumerated">
        <item code="0" name="END_USER_E164"/>
        <item code="1" name="END_USER_IMSI"/>
        <item code="2" name="END_USER_SIP_URI"/>
        <item code="3" name="END_USER_NAI"/>
        <item code="4" name="END_USER_PRIVATE"/>
      </data>
    </avp>

    <!-- Rx 3GPP AVPs (3GPP TS 29.214) -->
    <avp name="Abort-Cause" code="500" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="BEARER_RELEASED"/>
        <item code="1" name="INSUFFICIENT_SERVER_RESOURCES"/>
        <item code="2" name="INSUFFICIENT_BEARER_RESOURCES"/>
        <item code="3" name="PS_TO_CS_HANDOVER"/>
        <item code="4" name="SPONSORED_DATA_CONNECTIVITY_DISALLOWED"/>
      </data>
    </avp>

    <avp name="AF-Application-Identifier" code="504" must="V" may="M" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="OctetString"/>
    </avp>

    <avp name="AF-Charging-Identifier" code="505" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="OctetString"/>
    </avp>

    <avp name="Flow-Description" code="507" must="V" may="M" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="IPFilterRule"/>
    </avp>

    <avp name="Flow-Number" code="509" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>

    <avp name="Flow-Status" code="511" must="V" may="M" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="ENABLED-UPLINK"/>
        <item code="1" name="ENABLED-DOWNLINK"/>
        <item code="2" name="ENABLED"/>
        <item code="3" name="DISABLED"/>
        <item code="4" name="REMOVED"/>
      </data>
    </avp>

    <avp name="Flow-Usage" code="512" must="V" may="M" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="NO_INFORMATION"/>
        <item code="1" name="RTCP"/>
        <item code="2" name="AF_SIGNALLING"/>
      </data>
    </avp>

    <avp name="Specific-Action" code="513" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Enumerated">
        <item code="1"  name="CHARGING_CORRELATION_EXCHANGE"/>
        <item code="2"  name="INDICATION_OF_LOSS_OF_BEARER"/>
        <item code="3"  name="INDICATION_OF_RECOVERY_OF_BEARER"/>
        <item code="4"  name="INDICATION_OF_RELEASE_OF_BEARER"/>
        <item code="5"  name="IP_CAN_CHANGE"/>
        <item code="6"  name="INDICATION_OF_OUT_OF_CREDIT"/>
        <item code="7"  name="INDICATION_OF_SUCCESSFUL_RESOURCES_ALLOCATION"/>
        <item code="8"  name="INDICATION_OF_FAILED_RESOURCES_ALLOCATION"/>
        <item code="9"  name="INDICATION_OF_LIMITED_PCC_DEPLOYMENT"/>
        <item code="10" name="USAGE_REPORT"/>
        <item code="11" name="ACCESS_NETWORK_INFO_REPORT"/>
      </data>
    </avp>

    <avp name="Media-Component-Description" code="517" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Grouped"/>
    </avp>

    <avp name="Media-Component-Number" code="518" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>

    <avp name="Media-Sub-Component" code="519" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Grouped"/>
    </avp>

    <avp name="Media-Type" code="520" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Enumerated">
        <item code="0"          name="AUDIO"/>
        <item code="1"          name="VIDEO"/>
        <item code="2"          name="DATA"/>
        <item code="3"          name="APPLICATION"/>
        <item code="4"          name="CONTROL"/>
        <item code="5"          name="TEXT"/>
        <item code="6"          name="MESSAGE"/>
      </data>
    </avp>

    <avp name="RR-Bandwidth" code="521" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>

    <avp name="RS-Bandwidth" code="522" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>

    <avp name="SIP-Forking-Indication" code="523" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="SINGLE_DIALOGUE"/>
        <item code="1" name="SEVERAL_DIALOGUES"/>
      </data>
    </avp>

    <avp name="Codec-Data" code="524" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="OctetString"/>
    </avp>

    <avp name="Service-URN" code="525" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="OctetString"/>
    </avp>

    <avp name="Service-Info-Status" code="527" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="FINAL_SERVICE_INFORMATION"/>
        <item code="1" name="PRELIMINARY_SERVICE_INFORMATION"/>
      </data>
    </avp>

    <avp name="Rx-Request-Type" code="533" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="INITIAL_REQUEST"/>
        <item code="1" name="UPDATE_REQUEST"/>
        <item code="2" name="PCSCF_RESTORATION"/>
      </data>
    </avp>

    <avp name="Min-Requested-Bandwidth-DL" code="534" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>

    <avp name="Min-Requested-Bandwidth-UL" code="535" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>

    <avp name="Required-Access-Info" code="536" must="M,V" may="-" must-not="-" may-encrypt="Y" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="USER_LOCATION"/>
        <item code="1" name="MS_TIME_ZONE"/>
      </data>
    </avp>

  </application>
</diameter>`
