package slh

import (
	"fmt"
	"strings"

	"github.com/fiorix/go-diameter/v4/diam/dict"
)

// LoadDict registers the SLh application dictionary with the default
// Diameter dictionary parser. Must be called before starting the server.
func LoadDict() error {
	if err := dict.Default.Load(strings.NewReader(dictXML)); err != nil {
		return fmt.Errorf("slh: load dict: %w", err)
	}
	return nil
}

const dictXML = `<?xml version="1.0" encoding="UTF-8"?>
<diameter>
  <application id="16777291" type="auth" name="SLh">
    <vendor id="10415" name="3GPP"/>
    <command code="8388622" short="LR" name="LCS-Routing-Info">
      <request>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="Destination-Realm" required="true" max="1"/>
        <rule avp="User-Name" required="false" max="1"/>
        <rule avp="MSISDN" required="false" max="1"/>
      </request>
      <answer>
        <rule avp="Session-Id" required="true" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/>
        <rule avp="Result-Code" required="false" max="1"/>
        <rule avp="Experimental-Result" required="false" max="1"/>
        <rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/>
        <rule avp="User-Name" required="false" max="1"/>
        <rule avp="MSISDN" required="false" max="1"/>
        <rule avp="Serving-Node" required="false" max="1"/>
      </answer>
    </command>
    <avp name="LMSI" code="2400" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="Serving-Node" code="2401" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="MME-Name" required="false" max="1"/>
        <rule avp="MME-Number-for-MT-SMS" required="false" max="1"/>
        <rule avp="MME-Realm" required="false" max="1"/>
        <rule avp="SGSN-Name" required="false" max="1"/>
        <rule avp="SGSN-Realm" required="false" max="1"/>
        <rule avp="LCS-Capabilities-Sets" required="false" max="1"/>
      </data>
    </avp>
    <avp name="MME-Name" code="2402" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="DiameterIdentity"/>
    </avp>
    <avp name="MME-Number-for-MT-SMS" code="2403" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="LCS-Capabilities-Sets" code="2404" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="Additional-Serving-Node" code="2406" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="MME-Name" required="false" max="1"/>
        <rule avp="MME-Realm" required="false" max="1"/>
        <rule avp="SGSN-Name" required="false" max="1"/>
        <rule avp="SGSN-Realm" required="false" max="1"/>
      </data>
    </avp>
    <avp name="MME-Realm" code="2408" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="DiameterIdentity"/>
    </avp>
    <avp name="SGSN-Name" code="2409" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="DiameterIdentity"/>
    </avp>
    <avp name="SGSN-Realm" code="2410" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="DiameterIdentity"/>
    </avp>
  </application>
</diameter>`
