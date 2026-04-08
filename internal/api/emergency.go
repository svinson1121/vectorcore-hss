package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

// Emergency subscriber records are runtime state written automatically by the
// Gx CCR handler when an emergency bearer is established for an unprovisioned UE.
// They are deleted automatically on CCR-T. The API is read-only.

type EmergencySubscriberListOutput struct{ Body []models.EmergencySubscriber }
type EmergencySubscriberOutput struct{ Body *models.EmergencySubscriber }
type EmergencySubscriberIDInput struct{ ID int `path:"id"` }

func registerEmergencySubscriberRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-emergency-subscribers",
		Method:      http.MethodGet,
		Path:        "/oam/emergency_subscriber",
		Summary:     "List active emergency sessions",
		Description: "Returns all currently active emergency bearer sessions. Records are created automatically when an unprovisioned UE establishes an emergency bearer via Gx, and deleted when the bearer is torn down.",
		Tags:        []string{"OAM"},
	}, s.listEmergencySubscribers)

	huma.Register(api, huma.Operation{
		OperationID: "get-emergency-subscriber",
		Method:      http.MethodGet,
		Path:        "/oam/emergency_subscriber/{id}",
		Summary:     "Get emergency session by ID",
		Tags:        []string{"OAM"},
	}, s.getEmergencySubscriber)
}

func (s *Server) listEmergencySubscribers(ctx context.Context, _ *struct{}) (*EmergencySubscriberListOutput, error) {
	var items []models.EmergencySubscriber
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &EmergencySubscriberListOutput{Body: items}, nil
}

func (s *Server) getEmergencySubscriber(ctx context.Context, input *EmergencySubscriberIDInput) (*EmergencySubscriberOutput, error) {
	var item models.EmergencySubscriber
	if err := s.db.WithContext(ctx).First(&item, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &EmergencySubscriberOutput{Body: &item}, nil
}
