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
	normalizedMNC := NormalizeMNC(mnc)
	domain := imsDomain(mcc, mnc)
	privateID := fmt.Sprintf("%s@%s", imsi, domain)

	ifcContent := ""
	if ifc != nil {
		ifcContent = ifc.XMLData
		ifcContent = strings.ReplaceAll(ifcContent, "{msisdn}", sub.MSISDN)
		ifcContent = strings.ReplaceAll(ifcContent, "{mcc}", mcc)
		ifcContent = strings.ReplaceAll(ifcContent, "{mnc}", normalizedMNC)
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

	normalizedMNC := NormalizeMNC(mnc)
	domain := imsDomain(mcc, mnc)
	privateIdentity := fmt.Sprintf("%s@%s", imsi, domain)

	// Public identities for the subscriber's MSISDN: TEL URI + SIP URI.
	tel := fmt.Sprintf("tel:%s", sub.MSISDN)
	sip := fmt.Sprintf("sip:%s@%s", sub.MSISDN, domain)

	pubIDElems := fmt.Sprintf("    <IMSPublicIdentity>%s</IMSPublicIdentity>\n    <IMSPublicIdentity>%s</IMSPublicIdentity>\n",
		escapeXML(tel), escapeXML(sip))

	pubIDBlocks := fmt.Sprintf(`      <PublicIdentity>
        <BarringIndication>0</BarringIndication>
        <Identity>%s</Identity>
      </PublicIdentity>
      <PublicIdentity>
        <BarringIndication>0</BarringIndication>
        <Identity>%s</Identity>
      </PublicIdentity>
`, escapeXML(tel), escapeXML(sip))

	ifcContent := ""
	if ifc != nil {
		ifcContent = ifc.XMLData
		ifcContent = strings.ReplaceAll(ifcContent, "{msisdn}", sub.MSISDN)
		ifcContent = strings.ReplaceAll(ifcContent, "{mcc}", mcc)
		ifcContent = strings.ReplaceAll(ifcContent, "{mnc}", normalizedMNC)
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
		pubIDElems,
		escapeXML(sub.MSISDN),
		userState,
		escapeXML(privateIdentity),
		escapeXML(scscfName),
		pubIDBlocks,
		ifcContent,
	)
}
