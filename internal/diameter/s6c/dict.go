package s6c

import (
	"fmt"
	"strings"
	"sync"

	"github.com/fiorix/go-diameter/v4/diam/dict"
)

// LoadDict registers the S6c application dictionary with the default
// Diameter dictionary parser. Must be called before starting the server.
func LoadDict() error {
	if err := dict.Default.Load(strings.NewReader(dictXML)); err != nil {
		return fmt.Errorf("s6c: load dict: %w", err)
	}
	return nil
}

var (
	loadMSISDNSupplementOnce sync.Once
	loadMSISDNSupplementErr  error
)

// LoadMSISDNSupplement registers MSISDN under application 0 so S6c request
// structs can resolve the shared 3GPP AVP during msg.Unmarshal even when the
// Diameter library does not map S6c to the TGPP parent application.
func LoadMSISDNSupplement() error {
	loadMSISDNSupplementOnce.Do(func() {
		if err := dict.Default.Load(strings.NewReader(msisdnSupplementDict)); err != nil {
			loadMSISDNSupplementErr = fmt.Errorf("s6c: load MSISDN supplement: %w", err)
		}
	})
	return loadMSISDNSupplementErr
}

// dictXML defines the S6c application (3GPP TS 29.338) and only the AVPs
// that are not already registered by previously loaded dicts (e.g. SLh).
// AVPs shared with SLh (MSISDN 701, LMSI 2400, Serving-Node 2401, etc.)
// are intentionally omitted here to avoid duplicate-registration errors.
const dictXML = `<?xml version="1.0" encoding="UTF-8"?>
<diameter>
  <application id="16777312" type="auth" name="S6c">
    <vendor id="10415" name="3GPP"/>

    <!-- Send-Routing-Info-for-SM: SMS-SC → HSS -->
    <command code="8388647" short="SI" name="Send-Routing-Info-for-SM">
      <request>
        <rule avp="Session-Id"         required="true"  max="1"/>
        <rule avp="Auth-Session-State" required="true"  max="1"/>
        <rule avp="Origin-Host"        required="true"  max="1"/>
        <rule avp="Origin-Realm"       required="true"  max="1"/>
        <rule avp="Destination-Realm"  required="true"  max="1"/>
        <rule avp="User-Name"          required="false" max="1"/>
        <rule avp="MSISDN"             required="false" max="1"/>
        <rule avp="SM-RP-MTI"          required="false" max="1"/>
        <rule avp="SC-Address"         required="false" max="1"/>
      </request>
      <answer>
        <rule avp="Session-Id"                       required="true"  max="1"/>
        <rule avp="Auth-Session-State"               required="true"  max="1"/>
        <rule avp="Result-Code"                      required="false" max="1"/>
        <rule avp="Experimental-Result"              required="false" max="1"/>
        <rule avp="Origin-Host"                      required="true"  max="1"/>
        <rule avp="Origin-Realm"                     required="true"  max="1"/>
        <rule avp="User-Name"                        required="false" max="1"/>
        <rule avp="MSISDN"                           required="false" max="1"/>
        <rule avp="Serving-Node"                     required="false" max="1"/>
        <rule avp="Additional-Serving-Node"          required="false" max="1"/>
        <rule avp="LMSI"                             required="false" max="1"/>
        <rule avp="MWD-Status"                       required="false" max="1"/>
        <rule avp="MME-Absent-User-Diagnostic-SM"    required="false" max="1"/>
        <rule avp="SGSN-Absent-User-Diagnostic-SM"   required="false" max="1"/>
        <rule avp="MSC-Absent-User-Diagnostic-SM"    required="false" max="1"/>
      </answer>
    </command>

    <!-- Alert-Service-Centre: HSS → SMS-SC (HSS-initiated) -->
    <command code="8388648" short="AS" name="Alert-Service-Centre">
      <request>
        <rule avp="Session-Id"         required="true"  max="1"/>
        <rule avp="Auth-Session-State" required="true"  max="1"/>
        <rule avp="Origin-Host"        required="true"  max="1"/>
        <rule avp="Origin-Realm"       required="true"  max="1"/>
        <rule avp="Destination-Host"   required="true"  max="1"/>
        <rule avp="Destination-Realm"  required="true"  max="1"/>
        <rule avp="SC-Address"         required="true"  max="1"/>
        <rule avp="User-Identifier"    required="true" max="1"/>
        <rule avp="SMSMI-Correlation-ID" required="false" max="1"/>
        <rule avp="Absent-User-Diagnostic-SM" required="false" max="1"/>
        <rule avp="Serving-Node"       required="false" max="1"/>
        <rule avp="Maximum-UE-Availability-Time" required="false" max="1"/>
        <rule avp="SMS-GMSC-Alert-Event" required="false" max="1"/>
      </request>
      <answer>
        <rule avp="Session-Id"         required="true"  max="1"/>
        <rule avp="Auth-Session-State" required="true"  max="1"/>
        <rule avp="Result-Code"        required="false" max="1"/>
        <rule avp="Experimental-Result" required="false" max="1"/>
        <rule avp="Origin-Host"        required="true"  max="1"/>
        <rule avp="Origin-Realm"       required="true"  max="1"/>
      </answer>
    </command>

    <!-- Report-SM-Delivery-Status: SMS-SC → HSS -->
    <command code="8388649" short="RD" name="Report-SM-Delivery-Status">
      <request>
        <rule avp="Session-Id"         required="true"  max="1"/>
        <rule avp="Auth-Session-State" required="true"  max="1"/>
        <rule avp="Origin-Host"        required="true"  max="1"/>
        <rule avp="Origin-Realm"       required="true"  max="1"/>
        <rule avp="Destination-Realm"  required="true"  max="1"/>
        <rule avp="User-Identifier"    required="true" max="1"/>
        <rule avp="SC-Address"         required="true"  max="1"/>
        <rule avp="SM-RP-MTI"          required="false" max="1"/>
        <rule avp="RDR-Flags"          required="false" max="1"/>
        <rule avp="SMSMI-Correlation-ID" required="false" max="1"/>
        <rule avp="SM-Delivery-Outcome" required="false" max="1"/>
      </request>
      <answer>
        <rule avp="Session-Id"         required="true"  max="1"/>
        <rule avp="Auth-Session-State" required="true"  max="1"/>
        <rule avp="Result-Code"        required="false" max="1"/>
        <rule avp="Experimental-Result" required="false" max="1"/>
        <rule avp="Origin-Host"        required="true"  max="1"/>
        <rule avp="Origin-Realm"       required="true"  max="1"/>
        <rule avp="User-Identifier"    required="false" max="1"/>
        <rule avp="MWD-Status"         required="false" max="1"/>
      </answer>
    </command>

    <!-- S6c-specific AVPs (TS 29.338 §7.3) -->
    <avp name="SC-Address" code="3300" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="SM-RP-MTI" code="3308" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="SM_DELIVER"/>
        <item code="1" name="SMS_SUBMIT_REPORT"/>
      </data>
    </avp>
    <avp name="SM-RP-SMEA" code="3309" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="MWD-Status" code="3312" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="MME-Absent-User-Diagnostic-SM" code="3313" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="SGSN-Absent-User-Diagnostic-SM" code="3314" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="MSC-Absent-User-Diagnostic-SM" code="3315" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="SM-Delivery-Outcome" code="3316" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="MME-Delivery-Outcome"      required="false" max="1"/>
        <rule avp="SGSN-Delivery-Outcome"     required="false" max="1"/>
        <rule avp="MSC-Delivery-Outcome"      required="false" max="1"/>
        <rule avp="IP-SM-GW-Delivery-Outcome" required="false" max="1"/>
      </data>
    </avp>
    <avp name="MME-Delivery-Outcome" code="3317" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="SM-Delivery-Cause" required="false" max="1"/>
        <rule avp="Absent-User-Diagnostic-SM" required="false" max="1"/>
      </data>
    </avp>
    <avp name="SGSN-Delivery-Outcome" code="3318" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="SM-Delivery-Cause" required="false" max="1"/>
        <rule avp="Absent-User-Diagnostic-SM" required="false" max="1"/>
      </data>
    </avp>
    <avp name="MSC-Delivery-Outcome" code="3319" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="SM-Delivery-Cause" required="false" max="1"/>
        <rule avp="Absent-User-Diagnostic-SM" required="false" max="1"/>
      </data>
    </avp>
    <avp name="IP-SM-GW-Delivery-Outcome" code="3320" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="SM-Delivery-Cause" required="false" max="1"/>
        <rule avp="Absent-User-Diagnostic-SM" required="false" max="1"/>
      </data>
    </avp>
    <avp name="SM-Delivery-Cause" code="3321" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Enumerated">
        <item code="0" name="UE_MEMORY_CAPACITY_EXCEEDED"/>
        <item code="1" name="ABSENT_USER"/>
        <item code="2" name="SUCCESSFUL_TRANSFER"/>
      </data>
    </avp>
    <avp name="Absent-User-Diagnostic-SM" code="3322" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="RDR-Flags" code="3323" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
    <avp name="SMSMI-Correlation-ID" code="3324" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="Maximum-UE-Availability-Time" code="3329" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Time"/>
    </avp>
    <avp name="SMS-GMSC-Alert-Event" code="3333" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Unsigned32"/>
    </avp>
  </application>
</diameter>`

const msisdnSupplementDict = `<?xml version="1.0" encoding="UTF-8"?>
<diameter>
  <application id="0" name="Base">
    <vendor id="10415" name="3GPP"/>
    <avp name="User-Identifier" code="3102" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="User-Name" required="false" max="1"/>
        <rule avp="MSISDN" required="false" max="1"/>
        <rule avp="LMSI" required="false" max="1"/>
      </data>
    </avp>
    <avp name="Serving-Node" code="2401" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="Grouped">
        <rule avp="SGSN-Name" required="false" max="1"/>
        <rule avp="SGSN-Realm" required="false" max="1"/>
        <rule avp="MME-Name" required="false" max="1"/>
        <rule avp="MME-Realm" required="false" max="1"/>
        <rule avp="MME-Number-for-MT-SMS" required="false" max="1"/>
      </data>
    </avp>
    <avp name="MME-Name" code="2402" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="DiameterIdentity"/>
    </avp>
    <avp name="SGSN-Name" code="2409" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="DiameterIdentity"/>
    </avp>
    <avp name="MME-Realm" code="2408" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="DiameterIdentity"/>
    </avp>
    <avp name="SGSN-Realm" code="2410" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="DiameterIdentity"/>
    </avp>
    <avp name="MME-Number-for-MT-SMS" code="1645" must="V" may="M" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
    <avp name="MSISDN" code="701" must="M,V" must-not="-" may-encrypt="N" vendor-id="10415">
      <data type="OctetString"/>
    </avp>
  </application>
</diameter>`
