package gx

import (
	"fmt"
	"strings"

	"github.com/fiorix/go-diameter/v4/diam/dict"
)

func LoadDict() error {
	if err := dict.Default.Load(strings.NewReader(dictXML)); err != nil {
		return fmt.Errorf("gx: load dict: %w", err)
	}
	return nil
}

// dictXML registers the 3GPP Gx application (16777238), its Credit-Control
// command (272), and its vendor-specific AVPs.
//
// The CCR/CCA command MUST be declared here because dict.FindCommand only
// falls back to app 0 (base), not the parent app 4.  Without this declaration
// readHeader returns "Could not find preloaded Command with code 272" and the
// connection is closed before the handler is ever dispatched.
const dictXML = `<?xml version="1.0" encoding="UTF-8"?>
<diameter>
  <application id="16777238" type="auth" name="Gx">
    <vendor id="10415" name="3GPP"/>

    <!-- Credit-Control command (3GPP TS 29.212 §5.6) -->
    <command code="272" short="CC" name="Credit-Control">
      <request>
        <rule avp="Session-Id"                         required="true"  max="1"/>
        <rule avp="Origin-Host"                        required="true"  max="1"/>
        <rule avp="Origin-Realm"                       required="true"  max="1"/>
        <rule avp="Destination-Realm"                  required="true"  max="1"/>
        <rule avp="Auth-Application-Id"                required="true"  max="1"/>
        <rule avp="CC-Request-Type"                    required="true"  max="1"/>
        <rule avp="CC-Request-Number"                  required="true"  max="1"/>
        <rule avp="Destination-Host"                   required="false" max="1"/>
        <rule avp="Origin-State-Id"                    required="false" max="1"/>
        <rule avp="Subscription-Id"                    required="false"/>
        <rule avp="Supported-Features"                 required="false"/>
        <rule avp="Network-Request-Support"            required="false" max="1"/>
        <rule avp="Framed-IP-Address"                  required="false" max="1"/>
        <rule avp="Framed-IPv6-Prefix"                 required="false" max="1"/>
        <rule avp="IP-CAN-Type"                        required="false" max="1"/>
        <rule avp="Called-Station-Id"                  required="false" max="1"/>
        <rule avp="RAT-Type"                           required="false" max="1"/>
        <rule avp="QoS-Information"                    required="false" max="1"/>
        <rule avp="Default-EPS-Bearer-QoS"             required="false" max="1"/>
        <rule avp="AN-GW-Address"                      required="false"/>
        <rule avp="Bearer-Usage"                       required="false" max="1"/>
        <rule avp="Online"                             required="false" max="1"/>
        <rule avp="Offline"                            required="false" max="1"/>
        <rule avp="Access-Network-Charging-Address"    required="false" max="1"/>
        <rule avp="Access-Network-Charging-Identifier-Gx" required="false"/>
        <rule avp="User-Equipment-Info"                required="false" max="1"/>
        <rule avp="Route-Record"                       required="false"/>
        <rule avp="Proxy-Info"                         required="false"/>
        <rule avp="Termination-Cause"                  required="false" max="1"/>
      </request>
      <answer>
        <rule avp="Session-Id"             required="true"  max="1"/>
        <rule avp="Result-Code"            required="false" max="1"/>
        <rule avp="Experimental-Result"    required="false" max="1"/>
        <rule avp="Origin-Host"            required="true"  max="1"/>
        <rule avp="Origin-Realm"           required="true"  max="1"/>
        <rule avp="CC-Request-Type"        required="true"  max="1"/>
        <rule avp="CC-Request-Number"      required="true"  max="1"/>
        <rule avp="Origin-State-Id"        required="false" max="1"/>
        <rule avp="Charging-Rule-Install"  required="false"/>
        <rule avp="Charging-Rule-Remove"   required="false"/>
        <rule avp="Event-Trigger"          required="false"/>
        <rule avp="Framed-IP-Address"      required="false" max="1"/>
        <rule avp="Framed-IPv6-Prefix"     required="false" max="1"/>
        <rule avp="Failed-AVP"             required="false" max="1"/>
        <rule avp="Route-Record"           required="false"/>
        <rule avp="Proxy-Info"             required="false"/>
      </answer>
    </command>

    <!-- Charging rule AVPs -->
    <avp name="Charging-Rule-Install" code="1001" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="Charging-Rule-Definition" required="false"/>
        <rule avp="Charging-Rule-Name"       required="false"/>
      </data>
    </avp>
    <avp name="Charging-Rule-Remove" code="1002" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="Charging-Rule-Name" required="false"/>
      </data>
    </avp>
    <avp name="Charging-Rule-Definition" code="1003" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="Charging-Rule-Name"  required="true"  max="1"/>
        <rule avp="Rating-Group"        required="false" max="1"/>
        <rule avp="Precedence"          required="false" max="1"/>
        <rule avp="QoS-Information"     required="false" max="1"/>
      </data>
    </avp>
    <avp name="Charging-Rule-Name" code="1005" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="Event-Trigger" code="1006" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="0"  name="SGSN_CHANGE"/>
        <item code="1"  name="QOS_CHANGE"/>
        <item code="2"  name="RAT_CHANGE"/>
        <item code="13" name="UE_IP_ADDRESS_ALLOCATE"/>
        <item code="14" name="UE_IP_ADDRESS_RELEASE"/>
        <item code="26" name="DEFAULT_EPS_BEARER_QOS_CHANGE"/>
        <item code="33" name="DEFAULT_EPS_BEARER_QOS_MODIFICATION_FAILURE"/>
      </data>
    </avp>

    <!-- QoS AVPs -->
    <avp name="QoS-Information" code="1016" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="QoS-Class-Identifier"         required="false" max="1"/>
        <rule avp="Max-Requested-Bandwidth-DL"   required="false" max="1"/>
        <rule avp="Max-Requested-Bandwidth-UL"   required="false" max="1"/>
        <rule avp="Guaranteed-Bitrate-DL"        required="false" max="1"/>
        <rule avp="Guaranteed-Bitrate-UL"        required="false" max="1"/>
        <rule avp="APN-Aggregate-Max-Bitrate-DL" required="false" max="1"/>
        <rule avp="APN-Aggregate-Max-Bitrate-UL" required="false" max="1"/>
        <rule avp="Allocation-Retention-Priority" required="false" max="1"/>
      </data>
    </avp>
    <avp name="Max-Requested-Bandwidth-DL" code="515" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="Max-Requested-Bandwidth-UL" code="516" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="Guaranteed-Bitrate-DL" code="1025" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="Guaranteed-Bitrate-UL" code="1026" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="QoS-Class-Identifier" code="1028" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="1"  name="QCI_1"/>
        <item code="2"  name="QCI_2"/>
        <item code="3"  name="QCI_3"/>
        <item code="4"  name="QCI_4"/>
        <item code="5"  name="QCI_5"/>
        <item code="6"  name="QCI_6"/>
        <item code="7"  name="QCI_7"/>
        <item code="8"  name="QCI_8"/>
        <item code="9"  name="QCI_9"/>
      </data>
    </avp>
    <avp name="Allocation-Retention-Priority" code="1034" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="Priority-Level"           required="true"  max="1"/>
        <rule avp="Pre-emption-Capability"   required="false" max="1"/>
        <rule avp="Pre-emption-Vulnerability" required="false" max="1"/>
      </data>
    </avp>
    <avp name="APN-Aggregate-Max-Bitrate-DL" code="1040" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="APN-Aggregate-Max-Bitrate-UL" code="1041" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="Priority-Level" code="1046" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="Pre-emption-Capability" code="1047" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="PRE-EMPTION_CAPABILITY_ENABLED"/>
        <item code="1" name="PRE-EMPTION_CAPABILITY_DISABLED"/>
      </data>
    </avp>
    <avp name="Pre-emption-Vulnerability" code="1048" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="PRE-EMPTION_VULNERABILITY_ENABLED"/>
        <item code="1" name="PRE-EMPTION_VULNERABILITY_DISABLED"/>
      </data>
    </avp>
    <avp name="Default-EPS-Bearer-QoS" code="1049" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="QoS-Class-Identifier"         required="false" max="1"/>
        <rule avp="Allocation-Retention-Priority" required="false" max="1"/>
      </data>
    </avp>
    <avp name="Bearer-Control-Mode" code="1023" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="UE_ONLY"/>
        <item code="1" name="RESERVED"/>
        <item code="2" name="UE_NW"/>
      </data>
    </avp>
    <avp name="Network-Request-Support" code="1024" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="NETWORK_REQUEST_NOT_SUPPORTED"/>
        <item code="1" name="NETWORK_REQUEST_SUPPORTED"/>
      </data>
    </avp>
    <avp name="Precedence" code="1010" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>

    <!-- IP-CAN-Type and RAT-Type — sent by P-GW in CCR-I -->
    <avp name="IP-CAN-Type" code="1027" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <!-- 3GPP TS 29.212 §5.3.27 -->
      <data type="Enumerated">
        <item code="0"  name="3GPP-GPRS"/>
        <item code="1"  name="DOCSIS"/>
        <item code="2"  name="xDSL"/>
        <item code="3"  name="WiMAX"/>
        <item code="4"  name="3GPP2"/>
        <item code="5"  name="3GPP-EPS"/>
        <item code="6"  name="Non-3GPP-EPS"/>
      </data>
    </avp>
    <avp name="RAT-Type" code="1032" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <!-- 3GPP TS 29.212 §5.3.31 -->
      <data type="Enumerated">
        <item code="0"    name="WLAN"/>
        <item code="1"    name="VIRTUAL"/>
        <item code="1000" name="UTRAN"/>
        <item code="1001" name="GERAN"/>
        <item code="1002" name="GAN"/>
        <item code="1003" name="HSPA_EVOLUTION"/>
        <item code="1004" name="EUTRAN"/>
        <item code="2000" name="CDMA2000_1X"/>
        <item code="2001" name="HRPD"/>
        <item code="2002" name="UMB"/>
        <item code="2003" name="EHRPD"/>
      </data>
    </avp>

    <!-- Supported-Features and sub-AVPs — used by freeDiameter peers -->
    <avp name="Supported-Features" code="628" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <!-- 3GPP TS 29.229 §6.3.29 -->
      <data type="Grouped">
        <rule avp="Vendor-Id"      required="true"  max="1"/>
        <rule avp="Feature-List-ID" required="true"  max="1"/>
        <rule avp="Feature-List"   required="true"  max="1"/>
      </data>
    </avp>
    <avp name="Feature-List-ID" code="629" must="V" must-not="M" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="Feature-List" code="630" must="V" must-not="M" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>

    <!-- Access-Network-Charging AVPs -->
    <avp name="Access-Network-Charging-Address" code="501" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <!-- 3GPP TS 29.214 §5.3.2 -->
      <data type="Address"/>
    </avp>
    <avp name="Access-Network-Charging-Identifier-Gx" code="1022" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <!-- 3GPP TS 29.212 §5.3.22 -->
      <data type="Grouped">
        <rule avp="Access-Network-Charging-Identifier-Value" required="false" max="1"/>
      </data>
    </avp>
    <avp name="Access-Network-Charging-Identifier-Value" code="503" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <!-- 3GPP TS 29.214 §5.3.4 -->
      <data type="OctetString"/>
    </avp>

    <!-- AN-GW-Address — P-GW address reported in CCR -->
    <avp name="AN-GW-Address" code="1050" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <!-- 3GPP TS 29.212 §5.3.49 -->
      <data type="Address"/>
    </avp>

    <!-- Online / Offline charging indicators -->
    <avp name="Online" code="1009" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <!-- 3GPP TS 29.212 §5.3.9 -->
      <data type="Enumerated">
        <item code="0" name="DISABLE_ONLINE"/>
        <item code="1" name="ENABLE_ONLINE"/>
      </data>
    </avp>
    <avp name="Offline" code="1008" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <!-- 3GPP TS 29.212 §5.3.8 -->
      <data type="Enumerated">
        <item code="0" name="DISABLE_OFFLINE"/>
        <item code="1" name="ENABLE_OFFLINE"/>
      </data>
    </avp>

    <!-- Bearer-Usage -->
    <avp name="Bearer-Usage" code="1000" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <!-- 3GPP TS 29.212 §5.3.1 -->
      <data type="Enumerated">
        <item code="0" name="GENERAL"/>
        <item code="1" name="IMS_SIGNALLING"/>
      </data>
    </avp>

  </application>
</diameter>`
