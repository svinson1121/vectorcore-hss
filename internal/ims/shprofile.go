package ims

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

func escapeXML(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// BuildCxUserData generates the 3GPP Cx User-Data XML document (IMSSubscription,
// 3GPP TS 29.228 Annex D) for use in SAA Server-Assignment-Answer responses.
// The IFC profile xml_data (PublicIdentity blocks + InitialFilterCriteria) is
// embedded inside the ServiceProfile element.
func BuildCxUserData(sub *models.IMSSubscriber, ifc *models.IFCProfile, mcc, mnc string) string {
	imsi := ""
	if sub.IMSI != nil {
		imsi = *sub.IMSI
	}
	mncPadded := fmt.Sprintf("%03s", mnc)
	imsDomain := fmt.Sprintf("ims.mnc%s.mcc%s.3gppnetwork.org", mncPadded, mcc)
	privateID := fmt.Sprintf("%s@%s", imsi, imsDomain)

	ifcContent := ""
	if ifc != nil {
		ifcContent = ifc.XMLData
		ifcContent = strings.ReplaceAll(ifcContent, "{msisdn}", sub.MSISDN)
		ifcContent = strings.ReplaceAll(ifcContent, "{mcc}", mcc)
		ifcContent = strings.ReplaceAll(ifcContent, "{mnc}", mnc)
		if sub.IMSI != nil {
			ifcContent = strings.ReplaceAll(ifcContent, "{imsi}", *sub.IMSI)
		}
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<IMSSubscription>
  <PrivateID>%s</PrivateID>
  <ServiceProfile>
    %s
  </ServiceProfile>
</IMSSubscription>`,
		escapeXML(privateID),
		ifcContent,
	)
}

// BuildShUserData generates the 3GPP Sh User-Data XML document (3GPP TS 29.328)
// for an IMS subscriber. If ifc is non-nil its XMLData is embedded inside the
// ServiceProfile element as the Initial Filter Criteria content.
func BuildShUserData(sub *models.IMSSubscriber, ifc *models.IFCProfile, mcc, mnc string) string {
	userState := 1 // UNREGISTERED
	if sub.SCSCF != nil && *sub.SCSCF != "" {
		userState = 0 // REGISTERED
	}

	imsi := ""
	if sub.IMSI != nil {
		imsi = *sub.IMSI
	}

	mncPadded := fmt.Sprintf("%03s", mnc)
	imsDomain := fmt.Sprintf("ims.mnc%s.mcc%s.3gppnetwork.org", mncPadded, mcc)
	privateIdentity := fmt.Sprintf("%s@%s", imsi, imsDomain)

	// Build the list of public identities: TEL URI + SIP URI for primary MSISDN,
	// plus any additional MSISDNs from MSISDNList.
	type pubID struct{ tel, sip string }
	pubIDs := []pubID{{
		tel: fmt.Sprintf("tel:%s", sub.MSISDN),
		sip: fmt.Sprintf("sip:%s@%s", sub.MSISDN, imsDomain),
	}}
	if sub.MSISDNList != nil && *sub.MSISDNList != "" {
		for _, extra := range strings.Split(*sub.MSISDNList, ",") {
			extra = strings.TrimSpace(extra)
			if extra != "" && extra != sub.MSISDN {
				pubIDs = append(pubIDs, pubID{
					tel: fmt.Sprintf("tel:%s", extra),
					sip: fmt.Sprintf("sip:%s@%s", extra, imsDomain),
				})
			}
		}
	}

	// PublicIdentifiers block: one IMSPublicIdentity per URI.
	var pubIDElems strings.Builder
	for _, p := range pubIDs {
		fmt.Fprintf(&pubIDElems, "    <IMSPublicIdentity>%s</IMSPublicIdentity>\n", escapeXML(p.tel))
		fmt.Fprintf(&pubIDElems, "    <IMSPublicIdentity>%s</IMSPublicIdentity>\n", escapeXML(p.sip))
	}

	// ServiceProfile PublicIdentity blocks: one per URI.
	var pubIDBlocks strings.Builder
	for _, p := range pubIDs {
		fmt.Fprintf(&pubIDBlocks, `      <PublicIdentity>
        <BarringIndication>0</BarringIndication>
        <Identity>%s</Identity>
      </PublicIdentity>
      <PublicIdentity>
        <BarringIndication>0</BarringIndication>
        <Identity>%s</Identity>
      </PublicIdentity>
`, escapeXML(p.tel), escapeXML(p.sip))
	}

	ifcContent := ""
	if ifc != nil {
		ifcContent = ifc.XMLData
		ifcContent = strings.ReplaceAll(ifcContent, "{msisdn}", sub.MSISDN)
		ifcContent = strings.ReplaceAll(ifcContent, "{mcc}", mcc)
		ifcContent = strings.ReplaceAll(ifcContent, "{mnc}", mnc)
		if sub.IMSI != nil {
			ifcContent = strings.ReplaceAll(ifcContent, "{imsi}", *sub.IMSI)
		}
	}

	scscfName := ""
	if sub.SCSCF != nil {
		scscfName = *sub.SCSCF
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Sh-Data xmlns="urn:3gpp:ns:pss:shDataType:7.0">
  <PublicIdentifiers>
%s    <MSISDN>%s</MSISDN>
  </PublicIdentifiers>
  <IMSUserState>%d</IMSUserState>
  <ShIMSData>
    <IMSPrivateUserIdentity>%s</IMSPrivateUserIdentity>
    <SCSCFName>%s</SCSCFName>
    <ServiceProfile>
%s      %s
    </ServiceProfile>
  </ShIMSData>
</Sh-Data>`,
		pubIDElems.String(),
		escapeXML(sub.MSISDN),
		userState,
		escapeXML(privateIdentity),
		escapeXML(scscfName),
		pubIDBlocks.String(),
		ifcContent,
	)
}
