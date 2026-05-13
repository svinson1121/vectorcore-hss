package swx

import (
	"context"
	"strconv"
	"strings"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

func (h *Handlers) buildNon3GPPUserData(ctx context.Context, sub *models.Subscriber, accessStatus int) *diam.GroupedAVP {
	children := []*diam.AVP{
		diam.NewAVP(avpNon3GPPIPAccess, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.Enumerated(accessStatus)),
	}

	apnConfigs := h.buildSWxAPNConfigurations(ctx, sub)
	if len(apnConfigs) > 0 {
		children = append(children,
			diam.NewAVP(avpNon3GPPIPAccessAPN, avp.Mbit|avp.Vbit, Vendor3GPP,
				datatype.Enumerated(Non3GPPAPNsEnable)),
		)
		children = append(children, apnConfigs...)
	}

	return &diam.GroupedAVP{AVP: children}
}

func (h *Handlers) buildSWxAPNConfigurations(ctx context.Context, sub *models.Subscriber) []*diam.AVP {
	defaultStr := strconv.Itoa(sub.DefaultAPN)
	ordered := []string{defaultStr}
	for _, id := range strings.Split(sub.APNList, ",") {
		id = strings.TrimSpace(id)
		if id != "" && id != defaultStr {
			ordered = append(ordered, id)
		}
	}

	out := make([]*diam.AVP, 0, len(ordered))
	for i, idStr := range ordered {
		apnID, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			h.log.Warn("swx: skipping invalid subscriber APN id",
				zap.String("imsi", sub.IMSI), zap.String("apn_id", idStr), zap.Error(err))
			continue
		}
		a, err := h.store.GetAPNByID(ctx, apnID)
		if err != nil {
			h.log.Warn("swx: skipping missing subscriber APN",
				zap.String("imsi", sub.IMSI), zap.Int("apn_id", apnID), zap.Error(err))
			continue
		}
		out = append(out, buildSWxAPNConfiguration(uint32(i+1), a))
	}
	return out
}

func buildSWxAPNConfiguration(contextID uint32, a *models.APN) *diam.AVP {
	apnAVPs := []*diam.AVP{
		diam.NewAVP(avp.ContextIdentifier, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.Unsigned32(contextID)),
		diam.NewAVP(avp.PDNType, avp.Mbit|avp.Vbit, Vendor3GPP,
			datatype.Enumerated(a.IPVersion)),
		diam.NewAVP(avp.ServiceSelection, avp.Mbit, 0,
			datatype.UTF8String(a.APN)),
		diam.NewAVP(avp.EPSSubscribedQoSProfile, avp.Mbit|avp.Vbit, Vendor3GPP,
			&diam.GroupedAVP{AVP: []*diam.AVP{
				diam.NewAVP(avp.QoSClassIdentifier, avp.Mbit|avp.Vbit, Vendor3GPP,
					datatype.Enumerated(a.QCI)),
				diam.NewAVP(avp.AllocationRetentionPriority, avp.Vbit, Vendor3GPP,
					&diam.GroupedAVP{AVP: []*diam.AVP{
						diam.NewAVP(avp.PriorityLevel, avp.Vbit, Vendor3GPP,
							datatype.Unsigned32(uint32(a.ARPPriority))),
						diam.NewAVP(avp.PreemptionCapability, avp.Vbit, Vendor3GPP,
							datatype.Enumerated(boolToPreemption(a.ARPPreemptionCapability, false))),
						diam.NewAVP(avp.PreemptionVulnerability, avp.Vbit, Vendor3GPP,
							datatype.Enumerated(boolToPreemption(a.ARPPreemptionVulnerability, true))),
					}}),
			}}),
		diam.NewAVP(avp.AMBR, avp.Mbit|avp.Vbit, Vendor3GPP,
			&diam.GroupedAVP{AVP: []*diam.AVP{
				diam.NewAVP(avp.MaxRequestedBandwidthDL, avp.Mbit|avp.Vbit, Vendor3GPP,
					datatype.Unsigned32(uint32(a.APNAMBRDown))),
				diam.NewAVP(avp.MaxRequestedBandwidthUL, avp.Mbit|avp.Vbit, Vendor3GPP,
					datatype.Unsigned32(uint32(a.APNAMBRUp))),
			}}),
	}
	if a.ChargingCharacteristics != "" {
		apnAVPs = append(apnAVPs, diam.NewAVP(avp.TGPPChargingCharacteristics, avp.Vbit, Vendor3GPP,
			datatype.UTF8String(a.ChargingCharacteristics)))
	}
	return diam.NewAVP(avp.APNConfiguration, avp.Mbit|avp.Vbit, Vendor3GPP, &diam.GroupedAVP{AVP: apnAVPs})
}

func boolToPreemption(b *bool, def bool) int32 {
	v := def
	if b != nil {
		v = *b
	}
	if v {
		return 0
	}
	return 1
}
