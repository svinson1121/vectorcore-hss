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

type RoamingRulesListOutput struct{ Body []models.RoamingRules }
type RoamingRulesOutput struct{ Body *models.RoamingRules }
type RoamingRulesIDInput struct{ ID int `path:"id"` }
type RoamingRulesCreateInput struct{ Body *models.RoamingRules }
type RoamingRulesUpdateInput struct {
	ID   int `path:"id"`
	Body *models.RoamingRules
}

func registerRoamingRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-roaming-rules", Method: http.MethodGet, Path: "/roaming_rules", Summary: "List Roaming Rules", Tags: []string{"Roaming"}}, s.listRoamingRules)
	huma.Register(api, huma.Operation{OperationID: "create-roaming-rules", Method: http.MethodPost, Path: "/roaming_rules", Summary: "Create Roaming Rule", Tags: []string{"Roaming"}, DefaultStatus: http.StatusCreated}, s.createRoamingRule)
	huma.Register(api, huma.Operation{OperationID: "get-roaming-rules", Method: http.MethodGet, Path: "/roaming_rules/{id}", Summary: "Get Roaming Rule", Tags: []string{"Roaming"}}, s.getRoamingRule)
	huma.Register(api, huma.Operation{OperationID: "update-roaming-rules", Method: http.MethodPut, Path: "/roaming_rules/{id}", Summary: "Update Roaming Rule", Tags: []string{"Roaming"}}, s.updateRoamingRule)
	huma.Register(api, huma.Operation{OperationID: "delete-roaming-rules", Method: http.MethodDelete, Path: "/roaming_rules/{id}", Summary: "Delete Roaming Rule", Tags: []string{"Roaming"}}, s.deleteRoamingRule)
}

func (s *Server) listRoamingRules(ctx context.Context, _ *struct{}) (*RoamingRulesListOutput, error) {
	var items []models.RoamingRules
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &RoamingRulesListOutput{Body: items}, nil
}

func (s *Server) createRoamingRule(ctx context.Context, input *RoamingRulesCreateInput) (*RoamingRulesOutput, error) {
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	if err := s.db.WithContext(ctx).Create(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &RoamingRulesOutput{Body: input.Body}, nil
}

func (s *Server) getRoamingRule(ctx context.Context, input *RoamingRulesIDInput) (*RoamingRulesOutput, error) {
	var item models.RoamingRules
	if err := s.db.WithContext(ctx).First(&item, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &RoamingRulesOutput{Body: &item}, nil
}

func (s *Server) updateRoamingRule(ctx context.Context, input *RoamingRulesUpdateInput) (*RoamingRulesOutput, error) {
	if err := s.db.WithContext(ctx).First(&models.RoamingRules{}, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	input.Body.RoamingRuleID = input.ID
	if err := s.db.WithContext(ctx).Save(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return s.getRoamingRule(ctx, &RoamingRulesIDInput{ID: input.ID})
}

func (s *Server) deleteRoamingRule(ctx context.Context, input *RoamingRulesIDInput) (*struct{}, error) {
	if err := s.db.WithContext(ctx).Delete(&models.RoamingRules{}, input.ID).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return nil, nil
}
