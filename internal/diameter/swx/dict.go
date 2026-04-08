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
