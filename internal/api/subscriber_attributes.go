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

type SubscriberAttributeListOutput struct{ Body []models.SubscriberAttribute }
type SubscriberAttributeOutput struct{ Body *models.SubscriberAttribute }
type SubscriberAttributeIDInput struct{ ID int `path:"id"` }
type SubscriberAttributeCreateInput struct{ Body *models.SubscriberAttribute }
type SubscriberAttributeUpdateInput struct {
	ID   int `path:"id"`
	Body *models.SubscriberAttribute
}

func registerSubscriberAttributeRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-subscriber-attribute", Method: http.MethodGet, Path: "/subscriber/attributes", Summary: "List Subscriber Attributes", Tags: []string{"Subscriber"}}, s.listSubscriberAttributes)
	huma.Register(api, huma.Operation{OperationID: "create-subscriber-attribute", Method: http.MethodPost, Path: "/subscriber/attributes", Summary: "Create Subscriber Attribute", Tags: []string{"Subscriber"}, DefaultStatus: http.StatusCreated}, s.createSubscriberAttribute)
	huma.Register(api, huma.Operation{OperationID: "get-subscriber-attribute", Method: http.MethodGet, Path: "/subscriber/attributes/{id}", Summary: "Get Subscriber Attribute", Tags: []string{"Subscriber"}}, s.getSubscriberAttribute)
	huma.Register(api, huma.Operation{OperationID: "update-subscriber-attribute", Method: http.MethodPut, Path: "/subscriber/attributes/{id}", Summary: "Update Subscriber Attribute", Tags: []string{"Subscriber"}}, s.updateSubscriberAttribute)
	huma.Register(api, huma.Operation{OperationID: "delete-subscriber-attribute", Method: http.MethodDelete, Path: "/subscriber/attributes/{id}", Summary: "Delete Subscriber Attribute", Tags: []string{"Subscriber"}}, s.deleteSubscriberAttribute)
}

func (s *Server) listSubscriberAttributes(ctx context.Context, _ *struct{}) (*SubscriberAttributeListOutput, error) {
	var items []models.SubscriberAttribute
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &SubscriberAttributeListOutput{Body: items}, nil
}

func (s *Server) createSubscriberAttribute(ctx context.Context, input *SubscriberAttributeCreateInput) (*SubscriberAttributeOutput, error) {
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	if err := s.db.WithContext(ctx).Create(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &SubscriberAttributeOutput{Body: input.Body}, nil
}

func (s *Server) getSubscriberAttribute(ctx context.Context, input *SubscriberAttributeIDInput) (*SubscriberAttributeOutput, error) {
	var item models.SubscriberAttribute
	if err := s.db.WithContext(ctx).First(&item, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &SubscriberAttributeOutput{Body: &item}, nil
}

func (s *Server) updateSubscriberAttribute(ctx context.Context, input *SubscriberAttributeUpdateInput) (*SubscriberAttributeOutput, error) {
	if err := s.db.WithContext(ctx).First(&models.SubscriberAttribute{}, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	input.Body.SubscriberAttributeID = input.ID
	if err := s.db.WithContext(ctx).Save(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return s.getSubscriberAttribute(ctx, &SubscriberAttributeIDInput{ID: input.ID})
}

func (s *Server) deleteSubscriberAttribute(ctx context.Context, input *SubscriberAttributeIDInput) (*struct{}, error) {
	if err := s.db.WithContext(ctx).Delete(&models.SubscriberAttribute{}, input.ID).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return nil, nil
}
