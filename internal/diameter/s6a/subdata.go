package s6a

import (
	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
)

// avpIMSVoiceOverPSSessions is AVP 1291 (3GPP TS 29.272 §7.3.132).
// V-bit set, M-bit clear per spec.
const avpIMSVoiceOverPSSessions = uint32(1291)

// appendSubscriptionDataAVPs adds a Subscription-Data grouped AVP to msg,
// populated from sd. Used by both buildULA (ULR response) and SendIDR.
func appendSubscriptionDataAVPs(msg *diam.Message, sd *SubscriptionData) {
	subAVPs := []*diam.AVP{
		diam.NewAVP(avp.SubscriberStatus, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Enumerated(sd.SubscriberStatus)),
		diam.NewAVP(avp.NetworkAccessMode, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Enumerated(sd.NetworkAccessMode)),
		diam.NewAVP(avp.AccessRestrictionData, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(sd.AccessRestrictionData)),
		diam.NewAVP(avp.SubscribedPeriodicRAUTAUTimer, avp.Vbit, Vendor3GPP, datatype.Unsigned32(sd.SubscribedPeriodicRAUTAUTimer)),
		diam.NewAVP(avp.AMBR, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: []*diam.AVP{
			diam.NewAVP(avp.MaxRequestedBandwidthDL, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(sd.AMBR.MaxRequestedBandwidthDL)),
			diam.NewAVP(avp.MaxRequestedBandwidthUL, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(sd.AMBR.MaxRequestedBandwidthUL)),
		}}),
	}
	if len(sd.MSISDN) > 0 {
		subAVPs = append(subAVPs, diam.NewAVP(avp.MSISDN, avp.Mbit|avp.Vbit, Vendor3GPP, sd.MSISDN))
	}
	if sd.IMSVoiceOverPSSessions >= 0 {
		subAVPs = append(subAVPs, diam.NewAVP(avpIMSVoiceOverPSSessions, avp.Vbit, Vendor3GPP,
			datatype.Enumerated(sd.IMSVoiceOverPSSessions)))
	}

	profAVPs := []*diam.AVP{
		diam.NewAVP(avp.ContextIdentifier, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(sd.APNConfigurationProfile.ContextIdentifier)),
		diam.NewAVP(avp.AllAPNConfigurationsIncludedIndicator, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Enumerated(sd.APNConfigurationProfile.AllAPNConfigurationsIncludedIndicator)),
	}
	for _, a := range sd.APNConfigurationProfile.APNConfiguration {
		apnAVPs := []*diam.AVP{
			diam.NewAVP(avp.ContextIdentifier, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(a.ContextIdentifier)),
			diam.NewAVP(avp.PDNType, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Enumerated(a.PDNType)),
			diam.NewAVP(avp.ServiceSelection, avp.Mbit, 0, datatype.UTF8String(a.ServiceSelection)),
			diam.NewAVP(avp.EPSSubscribedQoSProfile, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: []*diam.AVP{
				diam.NewAVP(avp.QoSClassIdentifier, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Enumerated(a.EPSSubscribedQoSProfile.QoSClassIdentifier)),
				diam.NewAVP(avp.AllocationRetentionPriority, avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: []*diam.AVP{
					diam.NewAVP(avp.PriorityLevel, avp.Vbit, Vendor3GPP, datatype.Unsigned32(a.EPSSubscribedQoSProfile.AllocationRetentionPriority.PriorityLevel)),
					diam.NewAVP(avp.PreemptionCapability, avp.Vbit, Vendor3GPP, datatype.Enumerated(a.EPSSubscribedQoSProfile.AllocationRetentionPriority.PreemptionCapability)),
					diam.NewAVP(avp.PreemptionVulnerability, avp.Vbit, Vendor3GPP, datatype.Enumerated(a.EPSSubscribedQoSProfile.AllocationRetentionPriority.PreemptionVulnerability)),
				}}),
			}}),
			diam.NewAVP(avp.AMBR, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: []*diam.AVP{
				diam.NewAVP(avp.MaxRequestedBandwidthDL, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(a.AMBR.MaxRequestedBandwidthDL)),
				diam.NewAVP(avp.MaxRequestedBandwidthUL, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Unsigned32(a.AMBR.MaxRequestedBandwidthUL)),
			}}),
		}
		if a.TGPPChargingCharacteristics != "" {
			apnAVPs = append(apnAVPs, diam.NewAVP(avp.TGPPChargingCharacteristics, avp.Vbit, Vendor3GPP, datatype.UTF8String(a.TGPPChargingCharacteristics)))
		}
		profAVPs = append(profAVPs, diam.NewAVP(avp.APNConfiguration, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: apnAVPs}))
	}
	subAVPs = append(subAVPs, diam.NewAVP(avp.APNConfigurationProfile, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: profAVPs}))
	msg.NewAVP(avp.SubscriptionData, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: subAVPs})
}
