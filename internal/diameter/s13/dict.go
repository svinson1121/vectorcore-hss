package s13

import (
	"fmt"
	"strings"

	"github.com/fiorix/go-diameter/v4/diam/dict"
)

// LoadDict registers the S13 (EIR) application dictionary with the default
// Diameter dictionary parser. Must be called before starting the server.
func LoadDict() error {
	if err := dict.Default.Load(strings.NewReader(dictXML)); err != nil {
		return fmt.Errorf("s13: load dict: %w", err)
	}
	return nil
}

// dictXML is a minimal 3GPP S13 (EIR) dictionary fragment.
// It defines application 16777252, command 324 (ECR/ECA), and the key AVPs.
const dictXML = `<?xml version="1.0" encoding="UTF-8"?>
<diameter>
  <application id="16777252" type="auth" name="S13">
    <vendor id="10415" name="3GPP"/>
    <command code="324" short="EC" name="ME-Identity-Check">
      <request>
        <rule avp="Session-Id"           required="true"  max="1"/>
        <rule avp="Auth-Session-State"   required="true"  max="1"/>
        <rule avp="Origin-Host"          required="true"  max="1"/>
        <rule avp="Origin-Realm"         required="true"  max="1"/>
        <rule avp="Destination-Realm"    required="true"  max="1"/>
        <rule avp="Terminal-Information" required="false" max="1"/>
        <rule avp="User-Name"            required="false" max="1"/>
      </request>
      <answer>
        <rule avp="Session-Id"           required="true"  max="1"/>
        <rule avp="Auth-Session-State"   required="true"  max="1"/>
        <rule avp="Result-Code"          required="false" max="1"/>
        <rule avp="Experimental-Result"  required="false" max="1"/>
        <rule avp="Origin-Host"          required="true"  max="1"/>
        <rule avp="Origin-Realm"         required="true"  max="1"/>
        <rule avp="Equipment-Status"     required="false" max="1"/>
      </answer>
    </command>
    <avp name="Terminal-Information" code="1401" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="IMEI"             required="false" max="1"/>
        <rule avp="Software-Version" required="false" max="1"/>
      </data>
    </avp>
    <avp name="IMEI" code="1402" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="UTF8String"/>
    </avp>
    <avp name="Software-Version" code="1403" must="V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="UTF8String"/>
    </avp>
    <avp name="Equipment-Status" code="1445" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="WHITELISTED"/>
        <item code="1" name="BLACKLISTED"/>
        <item code="2" name="GREYLIST"/>
      </data>
    </avp>
  </application>
</diameter>`
