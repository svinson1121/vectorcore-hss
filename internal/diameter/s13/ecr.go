package s13

import (
	"context"
	"regexp"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/diameter/avputil"
	"github.com/svinson1121/vectorcore-hss/internal/models"
)

// ECR handles ME-Identity-Check-Request (S13, command code 324).
func (h *Handlers) ECR(conn diam.Conn, msg *diam.Message) (*diam.Message, error) {
	var ecr ECR
	if err := msg.Unmarshal(&ecr); err != nil {
		h.log.Error("s13: ECR unmarshal failed", zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, "", h.originHost, h.originRealm, diam.UnableToComply), err
	}

	imei := ""
	if ecr.TerminalInformation != nil {
		imei = string(ecr.TerminalInformation.IMEI)
	}
	imsi := string(ecr.UserName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := h.checkEIR(ctx, imei, imsi)
	if err != nil {
		h.log.Error("s13: EIR check failed", zap.String("imei", imei), zap.Error(err))
		return avputil.ConstructFailureAnswer(msg, ecr.SessionID, h.originHost, h.originRealm, diam.UnableToComply), err
	}

	statusNames := []string{"WHITELISTED", "BLACKLISTED", "GREYLIST"}
	statusName := "UNKNOWN"
	if status >= 0 && status < len(statusNames) {
		statusName = statusNames[status]
	}
	h.log.Info("s13: ECR", zap.String("imei", imei), zap.String("imsi", imsi), zap.String("status", statusName))

	if h.eirIMSIIMEILog && imsi != "" && imei != "" {
		devMake, devModel, found := h.tac.Lookup(imei)
		if !found {
			devMake, devModel = "Unknown", "Unknown"
		}
		if err := h.store.UpsertIMSIIMEIHistory(ctx, imsi, imei, devMake, devModel, status); err != nil {
			h.log.Warn("s13: EIR history write failed", zap.Error(err))
		}
	}

	ans := avputil.ConstructSuccessAnswer(msg, ecr.SessionID, h.originHost, h.originRealm, AppIDS13)
	ans.NewAVP(avpEquipmentStatus, avp.Mbit|avp.Vbit, Vendor3GPP, datatype.Enumerated(status))
	return ans, nil
}

// checkEIR returns the Equipment-Status for the given IMEI/IMSI pair.
// Lookup order: exact IMSI+IMEI match → IMSI-only match → IMEI-only match → no-match default.
func (h *Handlers) checkEIR(ctx context.Context, imei, imsi string) (int, error) {
	var entries []models.EIR
	if err := h.store.ListEIR(ctx, &entries); err != nil {
		return h.eirNoMatchResp, err
	}

	for _, e := range entries {
		eIMEI := ""
		eIMSI := ""
		if e.IMEI != nil {
			eIMEI = *e.IMEI
		}
		if e.IMSI != nil {
			eIMSI = *e.IMSI
		}

		matched := false
		if e.RegexMode == 1 {
			// Regex match on IMEI
			if eIMEI != "" && imei != "" {
				if ok, _ := regexp.MatchString(eIMEI, imei); ok {
					matched = true
				}
			}
		} else {
			// Exact match
			imeiMatch := eIMEI == "" || eIMEI == imei
			imsiMatch := eIMSI == "" || eIMSI == imsi
			matched = imeiMatch && imsiMatch && (eIMEI != "" || eIMSI != "")
		}

		if matched {
			return e.MatchResponseCode, nil
		}
	}

	return h.eirNoMatchResp, nil
}
