package swx

import (
	"fmt"
	"strings"

	"github.com/fiorix/go-diameter/v4/diam/dict"
)

func LoadDict() error {
	if err := dict.Default.Load(strings.NewReader(dictXML)); err != nil {
		return fmt.Errorf("swx: load dict: %w", err)
	}
	return nil
}

const dictXML = `<?xml version="1.0" encoding="UTF-8"?>
<diameter>
  <application id="16777265" type="auth" name="SWx">
    <vendor id="10415" name="3GPP"/>
    <command code="301" short="SA" name="Server-Assignment">
      <request>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="Destination-Realm" required="false" max="1"/>
        <rule avp="User-Name" required="true" max="1"/>
        <rule avp="Server-Assignment-Type" required="true" max="1"/>
      </request>
      <answer>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="false" max="1"/>
        <rule avp="Result-Code" required="false" max="1"/>
        <rule avp="Experimental-Result" required="false" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="User-Name" required="false" max="1"/>
        <rule avp="Non-3GPP-User-Data" required="false" max="1"/>
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
        <rule avp="SIP-Number-Auth-Items" required="false" max="1"/>
        <rule avp="SIP-Auth-Data-Item" required="false" max="1"/>
      </request>
      <answer>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Result-Code" required="false" max="1"/>
        <rule avp="Experimental-Result" required="false" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="User-Name" required="false" max="1"/>
        <rule avp="SIP-Number-Auth-Items" required="false" max="1"/>
        <rule avp="SIP-Auth-Data-Item" required="false"/>
      </answer>
    </command>
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
    <avp name="Confidentiality-Key" code="625" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="Integrity-Key" code="626" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="Non-3GPP-User-Data" code="1500" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="Non-3GPP-IP-Access" required="false" max="1"/>
        <rule avp="Non-3GPP-IP-Access-APN" required="false" max="1"/>
        <rule avp="AN-Trusted" required="false" max="1"/>
        <rule avp="APN-Configuration" required="false"/>
        <rule avp="AMBR" required="false" max="1"/>
        <rule avp="Session-Timeout" required="false" max="1"/>
      </data>
    </avp>
    <avp name="Non-3GPP-IP-Access" code="1501" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="NON_3GPP_SUBSCRIPTION_ALLOWED"/>
        <item code="1" name="NON_3GPP_SUBSCRIPTION_BARRED"/>
      </data>
    </avp>
    <avp name="Non-3GPP-IP-Access-APN" code="1502" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="NON_3GPP_APNS_ENABLE"/>
        <item code="1" name="NON_3GPP_APNS_DISABLE"/>
      </data>
    </avp>
    <avp name="AN-Trusted" code="1503" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="TRUSTED"/>
        <item code="1" name="UNTRUSTED"/>
      </data>
    </avp>
  </application>
</diameter>`
