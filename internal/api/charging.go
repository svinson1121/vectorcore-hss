package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

// ── Charging Rule ─────────────────────────────────────────────────────────────

type ChargingRuleListOutput struct{ Body []models.ChargingRule }
type ChargingRuleOutput struct{ Body *models.ChargingRule }
type ChargingRuleIDInput struct{ ID int `path:"id"` }
type ChargingRuleCreateInput struct{ Body *models.ChargingRule }
type ChargingRuleUpdateInput struct {
	ID   int `path:"id"`
	Body *models.ChargingRule
}

func registerChargingRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-charging-rule", Method: http.MethodGet, Path: "/apn/charging_rule", Summary: "List Charging Rules", Tags: []string{"APN"}}, s.listChargingRules)
	huma.Register(api, huma.Operation{OperationID: "create-charging-rule", Method: http.MethodPost, Path: "/apn/charging_rule", Summary: "Create Charging Rule", Tags: []string{"APN"}, DefaultStatus: http.StatusCreated}, s.createChargingRule)
	huma.Register(api, huma.Operation{OperationID: "get-charging-rule", Method: http.MethodGet, Path: "/apn/charging_rule/{id}", Summary: "Get Charging Rule", Tags: []string{"APN"}}, s.getChargingRule)
	huma.Register(api, huma.Operation{OperationID: "update-charging-rule", Method: http.MethodPut, Path: "/apn/charging_rule/{id}", Summary: "Update Charging Rule", Tags: []string{"APN"}}, s.updateChargingRule)
	huma.Register(api, huma.Operation{OperationID: "delete-charging-rule", Method: http.MethodDelete, Path: "/apn/charging_rule/{id}", Summary: "Delete Charging Rule", Tags: []string{"APN"}}, s.deleteChargingRule)
}

func (s *Server) listChargingRules(ctx context.Context, _ *struct{}) (*ChargingRuleListOutput, error) {
	var items []models.ChargingRule
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &ChargingRuleListOutput{Body: items}, nil
}

func (s *Server) createChargingRule(ctx context.Context, input *ChargingRuleCreateInput) (*ChargingRuleOutput, error) {
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	if err := s.db.WithContext(ctx).Create(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &ChargingRuleOutput{Body: input.Body}, nil
}

func (s *Server) getChargingRule(ctx context.Context, input *ChargingRuleIDInput) (*ChargingRuleOutput, error) {
	var item models.ChargingRule
	if err := s.db.WithContext(ctx).First(&item, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &ChargingRuleOutput{Body: &item}, nil
}

func (s *Server) updateChargingRule(ctx context.Context, input *ChargingRuleUpdateInput) (*ChargingRuleOutput, error) {
	if err := s.db.WithContext(ctx).First(&models.ChargingRule{}, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	input.Body.ChargingRuleID = input.ID
	if err := s.db.WithContext(ctx).Save(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return s.getChargingRule(ctx, &ChargingRuleIDInput{ID: input.ID})
}

func (s *Server) deleteChargingRule(ctx context.Context, input *ChargingRuleIDInput) (*struct{}, error) {
	if err := s.db.WithContext(ctx).Delete(&models.ChargingRule{}, input.ID).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return nil, nil
}

// ── TFT ───────────────────────────────────────────────────────────────────────

type TFTListOutput struct{ Body []models.TFT }
type TFTOutput struct{ Body *models.TFT }
type TFTIDInput struct{ ID int `path:"id"` }
type TFTCreateInput struct{ Body *models.TFT }
type TFTUpdateInput struct {
	ID   int `path:"id"`
	Body *models.TFT
}

func registerTFTRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-tft", Method: http.MethodGet, Path: "/apn/charging_rule/tft", Summary: "List TFTs", Tags: []string{"APN"}}, s.listTFTs)
	huma.Register(api, huma.Operation{OperationID: "create-tft", Method: http.MethodPost, Path: "/apn/charging_rule/tft", Summary: "Create TFT", Tags: []string{"APN"}, DefaultStatus: http.StatusCreated}, s.createTFT)
	huma.Register(api, huma.Operation{OperationID: "get-tft", Method: http.MethodGet, Path: "/apn/charging_rule/tft/{id}", Summary: "Get TFT", Tags: []string{"APN"}}, s.getTFT)
	huma.Register(api, huma.Operation{OperationID: "update-tft", Method: http.MethodPut, Path: "/apn/charging_rule/tft/{id}", Summary: "Update TFT", Tags: []string{"APN"}}, s.updateTFT)
	huma.Register(api, huma.Operation{OperationID: "delete-tft", Method: http.MethodDelete, Path: "/apn/charging_rule/tft/{id}", Summary: "Delete TFT", Tags: []string{"APN"}}, s.deleteTFT)
}

func (s *Server) listTFTs(ctx context.Context, _ *struct{}) (*TFTListOutput, error) {
	var items []models.TFT
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &TFTListOutput{Body: items}, nil
}

func (s *Server) createTFT(ctx context.Context, input *TFTCreateInput) (*TFTOutput, error) {
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	if err := s.db.WithContext(ctx).Create(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &TFTOutput{Body: input.Body}, nil
}

func (s *Server) getTFT(ctx context.Context, input *TFTIDInput) (*TFTOutput, error) {
	var item models.TFT
	if err := s.db.WithContext(ctx).First(&item, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &TFTOutput{Body: &item}, nil
}

func (s *Server) updateTFT(ctx context.Context, input *TFTUpdateInput) (*TFTOutput, error) {
	if err := s.db.WithContext(ctx).First(&models.TFT{}, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	input.Body.TFTID = input.ID
	if err := s.db.WithContext(ctx).Save(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return s.getTFT(ctx, &TFTIDInput{ID: input.ID})
}

func (s *Server) deleteTFT(ctx context.Context, input *TFTIDInput) (*struct{}, error) {
	if err := s.db.WithContext(ctx).Delete(&models.TFT{}, input.ID).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return nil, nil
}
