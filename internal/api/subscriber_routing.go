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

type SubscriberRoutingListOutput struct{ Body []models.SubscriberRouting }
type SubscriberRoutingOutput struct{ Body *models.SubscriberRouting }
type SubscriberRoutingIDInput struct{ ID int `path:"id"` }
type SubscriberRoutingCreateInput struct{ Body *models.SubscriberRouting }
type SubscriberRoutingUpdateInput struct {
	ID   int `path:"id"`
	Body *models.SubscriberRouting
}

func registerSubscriberRoutingRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-subscriber-routing", Method: http.MethodGet, Path: "/subscriber/routing", Summary: "List Subscriber Routings", Tags: []string{"Subscriber"}}, s.listSubscriberRoutings)
	huma.Register(api, huma.Operation{OperationID: "create-subscriber-routing", Method: http.MethodPost, Path: "/subscriber/routing", Summary: "Create Subscriber Routing", Tags: []string{"Subscriber"}, DefaultStatus: http.StatusCreated}, s.createSubscriberRouting)
	huma.Register(api, huma.Operation{OperationID: "get-subscriber-routing", Method: http.MethodGet, Path: "/subscriber/routing/{id}", Summary: "Get Subscriber Routing", Tags: []string{"Subscriber"}}, s.getSubscriberRouting)
	huma.Register(api, huma.Operation{OperationID: "update-subscriber-routing", Method: http.MethodPut, Path: "/subscriber/routing/{id}", Summary: "Update Subscriber Routing", Tags: []string{"Subscriber"}}, s.updateSubscriberRouting)
	huma.Register(api, huma.Operation{OperationID: "delete-subscriber-routing", Method: http.MethodDelete, Path: "/subscriber/routing/{id}", Summary: "Delete Subscriber Routing", Tags: []string{"Subscriber"}}, s.deleteSubscriberRouting)
}

func (s *Server) listSubscriberRoutings(ctx context.Context, _ *struct{}) (*SubscriberRoutingListOutput, error) {
	var items []models.SubscriberRouting
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &SubscriberRoutingListOutput{Body: items}, nil
}

func (s *Server) createSubscriberRouting(ctx context.Context, input *SubscriberRoutingCreateInput) (*SubscriberRoutingOutput, error) {
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	if err := s.db.WithContext(ctx).Create(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &SubscriberRoutingOutput{Body: input.Body}, nil
}

func (s *Server) getSubscriberRouting(ctx context.Context, input *SubscriberRoutingIDInput) (*SubscriberRoutingOutput, error) {
	var item models.SubscriberRouting
	if err := s.db.WithContext(ctx).First(&item, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &SubscriberRoutingOutput{Body: &item}, nil
}

func (s *Server) updateSubscriberRouting(ctx context.Context, input *SubscriberRoutingUpdateInput) (*SubscriberRoutingOutput, error) {
	if err := s.db.WithContext(ctx).First(&models.SubscriberRouting{}, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	input.Body.SubscriberRoutingID = input.ID
	if err := s.db.WithContext(ctx).Save(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return s.getSubscriberRouting(ctx, &SubscriberRoutingIDInput{ID: input.ID})
}

func (s *Server) deleteSubscriberRouting(ctx context.Context, input *SubscriberRoutingIDInput) (*struct{}, error) {
	if err := s.db.WithContext(ctx).Delete(&models.SubscriberRouting{}, input.ID).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return nil, nil
}
