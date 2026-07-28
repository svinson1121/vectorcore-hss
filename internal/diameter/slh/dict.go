package slh

import (
	"fmt"
	"strings"
	"sync"

	"github.com/fiorix/go-diameter/v4/diam/dict"
)

// LoadDict registers the TS 29.173 Release 16 SLh grammar.  go-diameter uses
// LRR as its historical dispatch alias; on the wire this is RIR/RIA.
var loadDictOnce sync.Once
var loadDictErr error

func LoadDict() error {
	loadDictOnce.Do(func() {
		if err := dict.Default.Load(strings.NewReader(dictXML)); err != nil {
			loadDictErr = fmt.Errorf("slh: load dict: %w", err)
		}
	})
	return loadDictErr
}

const dictXML = `<?xml version="1.0" encoding="UTF-8"?>
<diameter>
  <application id="16777291" type="auth" name="SLh">
    <vendor id="10415" name="3GPP"/>
    <command code="8388622" short="LR" name="LCS-Routing-Info">
      <request>
        <rule avp="Session-Id" required="true" max="1"/><rule avp="Vendor-Specific-Application-Id" required="false" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/><rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/><rule avp="Destination-Host" required="false" max="1"/>
        <rule avp="Destination-Realm" required="true" max="1"/><rule avp="User-Name" required="false" max="1"/>
        <rule avp="MSISDN" required="false" max="1"/><rule avp="GMLC-Number" required="false" max="1"/>
        <rule avp="Supported-Features" required="false" max="1"/><rule avp="Proxy-Info" required="false"/>
        <rule avp="Route-Record" required="false"/><rule avp="AVP" required="false"/>
      </request>
      <answer>
        <rule avp="Session-Id" required="true" max="1"/><rule avp="Vendor-Specific-Application-Id" required="false" max="1"/>
        <rule avp="Result-Code" required="false" max="1"/><rule avp="Experimental-Result" required="false" max="1"/>
        <rule avp="Auth-Session-State" required="true" max="1"/><rule avp="Origin-Host" required="true" max="1"/>
        <rule avp="Origin-Realm" required="true" max="1"/><rule avp="Supported-Features" required="false" max="1"/>
        <rule avp="User-Name" required="false" max="1"/><rule avp="MSISDN" required="false" max="1"/>
        <rule avp="LMSI" required="false" max="1"/><rule avp="Serving-Node" required="false" max="1"/>
        <rule avp="Additional-Serving-Node" required="false"/><rule avp="GMLC-Address" required="false" max="1"/>
        <rule avp="PPR-Address" required="false" max="1"/><rule avp="RIA-Flags" required="false" max="1"/>
        <rule avp="Failed-AVP" required="false" max="1"/><rule avp="Proxy-Info" required="false"/>
        <rule avp="Route-Record" required="false"/><rule avp="AVP" required="false"/>
      </answer>
    </command>
    <avp name="LMSI" code="2400" must="M,V" may-encrypt="N" vendor-id="10415"><data type="OctetString"/></avp>
    <avp name="MSISDN" code="701" must="M,V" may-encrypt="N" vendor-id="10415"><data type="OctetString"/></avp>
    <avp name="Serving-Node" code="2401" must="M,V" may-encrypt="N" vendor-id="10415"><data type="Grouped">
      <rule avp="SGSN-Number" required="false" max="1"/><rule avp="SGSN-Name" required="false" max="1"/><rule avp="SGSN-Realm" required="false" max="1"/>
      <rule avp="MME-Name" required="false" max="1"/><rule avp="MME-Realm" required="false" max="1"/><rule avp="MSC-Number" required="false" max="1"/>
	      <rule avp="3GPP-AAA-Server-Name" required="false" max="1"/><rule avp="LCS-Capabilities-Sets" required="false" max="1"/>
      <rule avp="GMLC-Address" required="false" max="1"/><rule avp="AVP" required="false"/>
    </data></avp>
    <avp name="MME-Name" code="2402" must="M,V" may-encrypt="N" vendor-id="10415"><data type="DiameterIdentity"/></avp>
    <avp name="MSC-Number" code="2403" must="M,V" may-encrypt="N" vendor-id="10415"><data type="OctetString"/></avp>
    <avp name="LCS-Capabilities-Sets" code="2404" must="M,V" may-encrypt="N" vendor-id="10415"><data type="Unsigned32"/></avp>
    <avp name="GMLC-Address" code="2405" must="M,V" may-encrypt="N" vendor-id="10415"><data type="Address"/></avp>
    <avp name="Additional-Serving-Node" code="2406" must="M,V" may-encrypt="N" vendor-id="10415"><data type="Grouped">
      <rule avp="SGSN-Number" required="false" max="1"/><rule avp="SGSN-Name" required="false" max="1"/><rule avp="SGSN-Realm" required="false" max="1"/>
      <rule avp="MME-Name" required="false" max="1"/><rule avp="MME-Realm" required="false" max="1"/><rule avp="MSC-Number" required="false" max="1"/>
	      <rule avp="3GPP-AAA-Server-Name" required="false" max="1"/><rule avp="LCS-Capabilities-Sets" required="false" max="1"/>
      <rule avp="GMLC-Address" required="false" max="1"/><rule avp="AVP" required="false"/>
    </data></avp>
    <avp name="PPR-Address" code="2407" must="M,V" may-encrypt="N" vendor-id="10415"><data type="Address"/></avp>
    <avp name="MME-Realm" code="2408" must="V" may-encrypt="N" vendor-id="10415"><data type="DiameterIdentity"/></avp>
    <avp name="SGSN-Name" code="2409" must="V" may-encrypt="N" vendor-id="10415"><data type="DiameterIdentity"/></avp>
    <avp name="SGSN-Realm" code="2410" must="V" may-encrypt="N" vendor-id="10415"><data type="DiameterIdentity"/></avp>
    <avp name="RIA-Flags" code="2411" must="V" may-encrypt="N" vendor-id="10415"><data type="Unsigned32"/></avp>
	    <avp name="3GPP-AAA-Server-Name" code="318" must="M,V" may-encrypt="N" vendor-id="10415"><data type="DiameterIdentity"/></avp>
  </application>
</diameter>`
