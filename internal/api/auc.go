package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"

	"github.com/svinson1121/vectorcore-hss/internal/geored"
	"github.com/svinson1121/vectorcore-hss/internal/models"
)

type AUCListInput struct {
	Search string `query:"search" doc:"Case-insensitive substring search on IMSI, ICCID, vendor, batch" default:""`
	Limit  int    `query:"limit"  doc:"Max rows; 0 = no limit"                                        default:"0"  minimum:"0"`
	Offset int    `query:"offset" doc:"Rows to skip"                                                  default:"0"  minimum:"0"`
}
type AUCListBody struct {
	Total int64        `json:"total"`
	Items []models.AUC `json:"items"`
}
type AUCListOutput struct{ Body AUCListBody }
type AUCOutput struct{ Body *models.AUC }
type AUCIDInput struct {
	ID int `path:"id"`
}
type AUCIMSIInput struct {
	IMSI string `path:"imsi"`
}
type AUCCreateInput struct{ Body *models.AUC }

// AUCUpdateBody is a partial-update shape where Ki/OPc are optional pointers so
// Huma does not mark them as required in the JSON schema.  When nil (absent from
// the request body) the existing database values are preserved.
type AUCUpdateBody struct {
	Ki                 *string `json:"ki,omitempty"            doc:"Authentication key Ki (hex 32). Omit to keep existing value."`
	OPc                *string `json:"opc,omitempty"           doc:"Operator key OPc (hex 32). Omit to keep existing value."`
	AMF                *string `json:"amf,omitempty"           doc:"Authentication Management Field (hex 4)."`
	ICCID              *string `json:"iccid,omitempty"`
	IMSI               *string `json:"imsi,omitempty"`
	AlgorithmProfileID *int64  `json:"algorithm_profile_id,omitempty"`
	BatchName          *string `json:"batch_name,omitempty"`
	SIMVendor          *string `json:"sim_vendor,omitempty"`
	ESim               *bool   `json:"esim,omitempty"`
	LPA                *string `json:"lpa,omitempty"`
	PIN1               *string `json:"pin1,omitempty"`
	PIN2               *string `json:"pin2,omitempty"`
	PUK1               *string `json:"puk1,omitempty"`
	PUK2               *string `json:"puk2,omitempty"`
	KID                *string `json:"kid,omitempty"`
	PSK                *string `json:"psk,omitempty"`
	DES                *string `json:"des,omitempty"`
	ADM1               *string `json:"adm1,omitempty"`
	Misc1              *string `json:"misc1,omitempty"`
	Misc2              *string `json:"misc2,omitempty"`
	Misc3              *string `json:"misc3,omitempty"`
	Misc4              *string `json:"misc4,omitempty"`
}

type AUCUpdateInput struct {
	ID   int `path:"id"`
	Body *AUCUpdateBody
}

func registerAUCRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-auc", Method: http.MethodGet, Path: "/subscriber/auc", Summary: "List AUCs", Tags: []string{"Subscriber"}}, s.listAUCs)
	huma.Register(api, huma.Operation{OperationID: "create-auc", Method: http.MethodPost, Path: "/subscriber/auc", Summary: "Create AUC", Tags: []string{"Subscriber"}, DefaultStatus: http.StatusCreated}, s.createAUC)
	huma.Register(api, huma.Operation{OperationID: "get-auc", Method: http.MethodGet, Path: "/subscriber/auc/{id}", Summary: "Get AUC", Tags: []string{"Subscriber"}}, s.getAUC)
	huma.Register(api, huma.Operation{OperationID: "get-auc-by-imsi", Method: http.MethodGet, Path: "/subscriber/auc/imsi/{imsi}", Summary: "Get AUC by IMSI", Tags: []string{"Subscriber"}}, s.getAUCByIMSI)
	huma.Register(api, huma.Operation{OperationID: "update-auc", Method: http.MethodPut, Path: "/subscriber/auc/{id}", Summary: "Update AUC", Tags: []string{"Subscriber"}}, s.updateAUC)
	huma.Register(api, huma.Operation{OperationID: "delete-auc", Method: http.MethodDelete, Path: "/subscriber/auc/{id}", Summary: "Delete AUC", Tags: []string{"Subscriber"}}, s.deleteAUC)
}

func (s *Server) listAUCs(ctx context.Context, input *AUCListInput) (*AUCListOutput, error) {
	q := s.db.WithContext(ctx).Model(&models.AUC{})
	if input.Search != "" {
		like := "%" + strings.ToLower(input.Search) + "%"
		q = q.Where("LOWER(imsi) LIKE ? OR LOWER(iccid) LIKE ? OR LOWER(sim_vendor) LIKE ? OR LOWER(batch_name) LIKE ?",
			like, like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if input.Limit > 0 {
		q = q.Limit(input.Limit).Offset(input.Offset)
	}
	var items []models.AUC
	if err := q.Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if items == nil {
		items = []models.AUC{}
	}
	return &AUCListOutput{Body: AUCListBody{Total: total, Items: items}}, nil
}

func (s *Server) createAUC(ctx context.Context, input *AUCCreateInput) (*AUCOutput, error) {
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	if err := s.db.WithContext(ctx).Create(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if s.geored != nil {
		s.geored.PublishOAMPut(geored.EventAUCPut, input.Body)
	}
	return &AUCOutput{Body: input.Body}, nil
}

func (s *Server) getAUC(ctx context.Context, input *AUCIDInput) (*AUCOutput, error) {
	var item models.AUC
	if err := s.db.WithContext(ctx).First(&item, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &AUCOutput{Body: &item}, nil
}

func (s *Server) getAUCByIMSI(ctx context.Context, input *AUCIMSIInput) (*AUCOutput, error) {
	var item models.AUC
	if err := s.db.WithContext(ctx).Where("imsi = ?", input.IMSI).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &AUCOutput{Body: &item}, nil
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *Server) updateAUC(ctx context.Context, input *AUCUpdateInput) (*AUCOutput, error) {
	var existing models.AUC
	if err := s.db.WithContext(ctx).First(&existing, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}

	b := input.Body

	// Preserve Ki/OPc when not supplied.
	ki := existing.Ki
	if b.Ki != nil && *b.Ki != "" {
		ki = *b.Ki
	}
	opc := existing.OPc
	if b.OPc != nil && *b.OPc != "" {
		opc = *b.OPc
	}
	amf := existing.AMF
	if b.AMF != nil && *b.AMF != "" {
		amf = *b.AMF
	}

	// Merge pointer fields — nil means "keep existing".
	updated := existing
	updated.Ki = ki
	updated.OPc = opc
	updated.AMF = amf
	if b.ICCID != nil {
		updated.ICCID = b.ICCID
	}
	if b.IMSI != nil {
		updated.IMSI = b.IMSI
	}
	if b.AlgorithmProfileID != nil {
		updated.AlgorithmProfileID = b.AlgorithmProfileID
	}
	if b.BatchName != nil {
		updated.BatchName = b.BatchName
	}
	if b.SIMVendor != nil {
		updated.SIMVendor = b.SIMVendor
	}
	if b.ESim != nil {
		updated.ESim = b.ESim
	}
	if b.LPA != nil {
		updated.LPA = b.LPA
	}
	if b.PIN1 != nil {
		updated.PIN1 = b.PIN1
	}
	if b.PIN2 != nil {
		updated.PIN2 = b.PIN2
	}
	if b.PUK1 != nil {
		updated.PUK1 = b.PUK1
	}
	if b.PUK2 != nil {
		updated.PUK2 = b.PUK2
	}
	if b.KID != nil {
		updated.KID = b.KID
	}
	if b.PSK != nil {
		updated.PSK = b.PSK
	}
	if b.DES != nil {
		updated.DES = b.DES
	}
	if b.ADM1 != nil {
		updated.ADM1 = b.ADM1
	}
	if b.Misc1 != nil {
		updated.Misc1 = b.Misc1
	}
	if b.Misc2 != nil {
		updated.Misc2 = b.Misc2
	}
	if b.Misc3 != nil {
		updated.Misc3 = b.Misc3
	}
	if b.Misc4 != nil {
		updated.Misc4 = b.Misc4
	}
	updated.LastModified = time.Now().UTC().Format(time.RFC3339)

	if err := s.db.WithContext(ctx).Save(&updated).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if s.geored != nil {
		s.geored.PublishOAMPut(geored.EventAUCPut, &updated)
	}
	return s.getAUC(ctx, &AUCIDInput{ID: input.ID})
}

func (s *Server) deleteAUC(ctx context.Context, input *AUCIDInput) (*struct{}, error) {
	if imsi, err := firstString(ctx, s.db, &models.Subscriber{}, "imsi", "auc_id = ?", input.ID); err == nil {
		return nil, conflictInUse("AUC", strconv.Itoa(input.ID), "subscriber", imsi)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if err := s.db.WithContext(ctx).Delete(&models.AUC{}, input.ID).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if s.geored != nil {
		s.geored.PublishOAMDel(geored.EventAUCDel, input.ID)
	}
	return nil, nil
}
