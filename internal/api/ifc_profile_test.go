package api

import (
	"strings"
	"testing"
)

func TestValidateIFCProfileXMLAcceptsInnerFragments(t *testing.T) {
	xmlData := `
<PublicIdentity>
  <Identity>sip:{msisdn}@ims.mnc{mnc}.mcc{mcc}.3gppnetwork.org</Identity>
</PublicIdentity>
<InitialFilterCriteria>
  <Priority>11</Priority>
</InitialFilterCriteria>`

	if err := validateIFCProfileXML(xmlData); err != nil {
		t.Fatalf("expected valid IFC fragment, got %v", err)
	}
}

func TestValidateIFCProfileXMLRejectsWrappedDocument(t *testing.T) {
	xmlData := `
<IMSSubscription>
  <PrivateID>{imsi}@ims.mnc{mnc}.mcc{mcc}.3gppnetwork.org</PrivateID>
  <ServiceProfile>
    <PublicIdentity>
      <Identity>sip:{msisdn}@ims.mnc{mnc}.mcc{mcc}.3gppnetwork.org</Identity>
    </PublicIdentity>
  </ServiceProfile>
</IMSSubscription>`

	err := validateIFCProfileXMLFragment(xmlData)
	if err == nil {
		t.Fatal("expected wrapped IFC XML to be rejected")
	}
	if !strings.Contains(err.Error(), "IMSSubscription") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateIFCProfileXMLRejectsXMLDeclaration(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<PublicIdentity>
  <Identity>sip:{msisdn}@ims.mnc{mnc}.mcc{mcc}.3gppnetwork.org</Identity>
</PublicIdentity>`

	err := validateIFCProfileXMLFragment(xmlData)
	if err == nil {
		t.Fatal("expected XML declaration to be rejected")
	}
	if !strings.Contains(err.Error(), "XML declaration") {
		t.Fatalf("unexpected error: %v", err)
	}
}
