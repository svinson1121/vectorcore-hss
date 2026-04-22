package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"

	"github.com/svinson1121/vectorcore-hss/internal/geored"
	"github.com/svinson1121/vectorcore-hss/internal/models"
)

// ── EIR ──────────────────────────────────────────────────────────────────────

type EIRListOutput struct{ Body []models.EIR }
type EIROutput struct{ Body *models.EIR }
type EIRIDInput struct{ ID int `path:"id"` }
type EIRCreateInput struct{ Body *models.EIR }
type EIRUpdateInput struct {
	ID   int `path:"id"`
	Body *models.EIR
}

func registerEIRRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-eir", Method: http.MethodGet, Path: "/eir", Summary: "List EIR", Tags: []string{"EIR"}}, s.listEIRs)
	huma.Register(api, huma.Operation{OperationID: "create-eir", Method: http.MethodPost, Path: "/eir", Summary: "Create EIR", Tags: []string{"EIR"}, DefaultStatus: http.StatusCreated}, s.createEIR)
	huma.Register(api, huma.Operation{OperationID: "get-eir", Method: http.MethodGet, Path: "/eir/{id}", Summary: "Get EIR", Tags: []string{"EIR"}}, s.getEIR)
	huma.Register(api, huma.Operation{OperationID: "update-eir", Method: http.MethodPut, Path: "/eir/{id}", Summary: "Update EIR", Tags: []string{"EIR"}}, s.updateEIR)
	huma.Register(api, huma.Operation{OperationID: "delete-eir", Method: http.MethodDelete, Path: "/eir/{id}", Summary: "Delete EIR", Tags: []string{"EIR"}}, s.deleteEIR)
}

func (s *Server) listEIRs(ctx context.Context, _ *struct{}) (*EIRListOutput, error) {
	var items []models.EIR
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &EIRListOutput{Body: items}, nil
}

func (s *Server) createEIR(ctx context.Context, input *EIRCreateInput) (*EIROutput, error) {
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	if err := s.db.WithContext(ctx).Create(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if s.geored != nil {
		s.geored.PublishOAMPut(geored.EventEIRPut, input.Body)
	}
	return &EIROutput{Body: input.Body}, nil
}

func (s *Server) getEIR(ctx context.Context, input *EIRIDInput) (*EIROutput, error) {
	var item models.EIR
	if err := s.db.WithContext(ctx).First(&item, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &EIROutput{Body: &item}, nil
}

func (s *Server) updateEIR(ctx context.Context, input *EIRUpdateInput) (*EIROutput, error) {
	if err := s.db.WithContext(ctx).First(&models.EIR{}, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	input.Body.EIRID = input.ID
	if err := s.db.WithContext(ctx).Save(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if s.geored != nil {
		s.geored.PublishOAMPut(geored.EventEIRPut, input.Body)
	}
	return s.getEIR(ctx, &EIRIDInput{ID: input.ID})
}

func (s *Server) deleteEIR(ctx context.Context, input *EIRIDInput) (*struct{}, error) {
	if err := s.db.WithContext(ctx).Delete(&models.EIR{}, input.ID).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if s.geored != nil {
		s.geored.PublishOAMDel(geored.EventEIRDel, input.ID)
	}
	return nil, nil
}

// ── EIR History (read-only) ───────────────────────────────────────────────────

type EIRHistoryListOutput struct{ Body []models.IMSIIMEIHistory }
type EIRHistoryOutput struct{ Body *models.IMSIIMEIHistory }
type EIRHistoryIDInput struct{ ID int `path:"id"` }

func registerEIRHistoryRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-eir-history", Method: http.MethodGet, Path: "/eir/history", Summary: "List EIR History", Tags: []string{"EIR"}}, s.listEIRHistory)
	huma.Register(api, huma.Operation{OperationID: "get-eir-history", Method: http.MethodGet, Path: "/eir/history/{id}", Summary: "Get EIR History", Tags: []string{"EIR"}}, s.getEIRHistory)
}

func (s *Server) listEIRHistory(ctx context.Context, _ *struct{}) (*EIRHistoryListOutput, error) {
	var items []models.IMSIIMEIHistory
	if err := s.db.WithContext(ctx).
		Order("imsi_imei_timestamp DESC NULLS LAST").
		Order("last_modified DESC").
		Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &EIRHistoryListOutput{Body: items}, nil
}

func (s *Server) getEIRHistory(ctx context.Context, input *EIRHistoryIDInput) (*EIRHistoryOutput, error) {
	var item models.IMSIIMEIHistory
	if err := s.db.WithContext(ctx).First(&item, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &EIRHistoryOutput{Body: &item}, nil
}
