package sh

import (
	"fmt"
	"strings"

	"github.com/fiorix/go-diameter/v4/diam/dict"
)

// LoadDict registers the Sh (IMS) application dictionary with the default
// Diameter dictionary parser. Must be called before starting the server.
func LoadDict() error {
	if err := dict.Default.Load(strings.NewReader(dictXML)); err != nil {
		return fmt.Errorf("sh: load dict: %w", err)
	}
	return nil
}

const dictXML = `<?xml version="1.0" encoding="UTF-8"?>
<diameter>
  <application id="16777217" type="auth" name="Sh">
    <vendor id="10415" name="3GPP"/>
    <command code="306" short="UD" name="User-Data">
      <request>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="Destination-Realm" required="true" max="1"/>
        <rule avp="User-Identity" required="true" max="1"/>
        <rule avp="Data-Reference" required="true"/>
      </request>
      <answer>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Result-Code" required="false" max="1"/>
        <rule avp="Experimental-Result" required="false" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="User-Data" required="false" max="1"/>
      </answer>
    </command>
    <command code="309" short="PN" name="Push-Notification">
      <request>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="Destination-Host" required="true" max="1"/>
        <rule avp="Destination-Realm" required="true" max="1"/>
        <rule avp="User-Identity" required="true" max="1"/>
        <rule avp="User-Data" required="true" max="1"/>
      </request>
      <answer>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Result-Code" required="false" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
      </answer>
    </command>
    <!-- Sub-AVPs of User-Identity must be defined here so that DecodeGrouped
         resolves them in the Sh application context (appid 16777217).
         Without these definitions, go-diameter falls back to UnknownType
         and the data arrives as raw OctetString instead of UTF8String. -->
    <avp name="Public-Identity" code="601" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="UTF8String"/>
    </avp>
    <avp name="MSISDN" code="701" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <!-- AVP 702: Sh-User-Data (TS 29.329). Distinct from AVP 606 Cx-User-Data (TS 29.229). -->
    <avp name="User-Data" code="702" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="User-Identity" code="700" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="Public-Identity" required="false" max="1"/>
        <rule avp="MSISDN" required="false" max="1"/>
      </data>
    </avp>
    <avp name="Data-Reference" code="703" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="RepositoryData"/>
        <item code="10" name="IMSPublicIdentity"/>
        <item code="11" name="IMSUserState"/>
        <item code="12" name="S-CSCFName"/>
        <item code="13" name="InitialFilterCriteria"/>
        <item code="14" name="LocationInformation"/>
        <item code="15" name="UserState"/>
        <item code="16" name="ChargingInformation"/>
        <item code="17" name="MSISDN"/>
        <item code="18" name="PSIActivation"/>
        <item code="19" name="DSAI"/>
        <item code="21" name="ServiceLevelTraceInfo"/>
        <item code="23" name="IPAddressSecureBindingInformation"/>
        <item code="24" name="ServicePriorityLevel"/>
        <item code="25" name="SMSRegistrationInfo"/>
        <item code="26" name="UEReachabilityForIP"/>
        <item code="27" name="TADSinformation"/>
        <item code="29" name="STN-SR"/>
        <item code="30" name="UE-SRVCC-Capability"/>
        <item code="31" name="ExtendedPriority"/>
        <item code="32" name="CSRN"/>
        <item code="33" name="ReferenceLocationInformation"/>
      </data>
    </avp>
  </application>
</diameter>`
