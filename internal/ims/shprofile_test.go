package ims

import (
	"strings"
	"testing"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

func TestNormalizeMNC(t *testing.T) {
	if got := NormalizeMNC("99"); got != "099" {
		t.Fatalf("NormalizeMNC(99) = %q, want %q", got, "099")
	}
	if got := NormalizeMNC("435"); got != "435" {
		t.Fatalf("NormalizeMNC(435) = %q, want %q", got, "435")
	}
}

func TestBuildCxUserDataPadsTwoDigitMNCInDomainAndIFCTemplate(t *testing.T) {
	imsi := "999990000000001"
	sub := &models.IMSSubscriber{
		IMSI:   &imsi,
		MSISDN: "15551234567",
	}
	ifc := &models.IFCProfile{
		XMLData: `<PublicIdentity><Identity>sip:{msisdn}@ims.mnc{mnc}.mcc{mcc}.3gppnetwork.org</Identity></PublicIdentity>`,
	}

	got := BuildCxUserData(sub, ifc, "999", "99")

	if !strings.Contains(got, "<PrivateID>999990000000001@ims.mnc099.mcc999.3gppnetwork.org</PrivateID>") {
		t.Fatalf("private ID did not use padded MNC: %s", got)
	}
	if !strings.Contains(got, "sip:15551234567@ims.mnc099.mcc999.3gppnetwork.org") {
		t.Fatalf("IFC template did not use padded MNC: %s", got)
	}
}

func TestBuildShUserDataPadsTwoDigitMNCInDomainAndIFCTemplate(t *testing.T) {
	imsi := "999990000000001"
	sub := &models.IMSSubscriber{
		IMSI:   &imsi,
		MSISDN: "15551234567",
	}
	ifc := &models.IFCProfile{
		XMLData: `<InitialFilterCriteria><ApplicationServer><ServerName>sip:tas.ims.mnc{mnc}.mcc{mcc}.3gppnetwork.org</ServerName></ApplicationServer></InitialFilterCriteria>`,
	}

	got := BuildShUserData(sub, ifc, "999", "99")

	if !strings.Contains(got, "<IMSPrivateUserIdentity>999990000000001@ims.mnc099.mcc999.3gppnetwork.org</IMSPrivateUserIdentity>") {
		t.Fatalf("private identity did not use padded MNC: %s", got)
	}
	if !strings.Contains(got, "sip:15551234567@ims.mnc099.mcc999.3gppnetwork.org") {
		t.Fatalf("public identity did not use padded MNC: %s", got)
	}
	if !strings.Contains(got, "sip:tas.ims.mnc099.mcc999.3gppnetwork.org") {
		t.Fatalf("IFC template did not use padded MNC: %s", got)
	}
}
