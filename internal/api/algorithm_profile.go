package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

// ── I/O types ─────────────────────────────────────────────────────────────────

type AlgorithmProfileListOutput struct{ Body []models.AlgorithmProfile }
type AlgorithmProfileOutput struct{ Body *models.AlgorithmProfile }
type AlgorithmProfileIDInput struct{ ID int `path:"id"` }
type AlgorithmProfileCreateInput struct{ Body *models.AlgorithmProfile }
type AlgorithmProfileUpdateInput struct {
	ID   int `path:"id"`
	Body *models.AlgorithmProfile
}

// ── Route registration ────────────────────────────────────────────────────────

func registerAlgorithmProfileRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-algorithm-profiles", Method: http.MethodGet,
		Path: "/subscriber/auc/profile", Summary: "List algorithm profiles", Tags: []string{"Subscriber"},
	}, s.listAlgorithmProfiles)
	huma.Register(api, huma.Operation{
		OperationID: "create-algorithm-profile", Method: http.MethodPost,
		Path: "/subscriber/auc/profile", Summary: "Create algorithm profile", Tags: []string{"Subscriber"},
		DefaultStatus: http.StatusCreated,
	}, s.createAlgorithmProfile)
	huma.Register(api, huma.Operation{
		OperationID: "get-algorithm-profile", Method: http.MethodGet,
		Path: "/subscriber/auc/profile/{id}", Summary: "Get algorithm profile", Tags: []string{"Subscriber"},
	}, s.getAlgorithmProfile)
	huma.Register(api, huma.Operation{
		OperationID: "update-algorithm-profile", Method: http.MethodPut,
		Path: "/subscriber/auc/profile/{id}", Summary: "Update algorithm profile", Tags: []string{"Subscriber"},
	}, s.updateAlgorithmProfile)
	huma.Register(api, huma.Operation{
		OperationID: "delete-algorithm-profile", Method: http.MethodDelete,
		Path: "/subscriber/auc/profile/{id}", Summary: "Delete algorithm profile", Tags: []string{"Subscriber"},
	}, s.deleteAlgorithmProfile)
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (s *Server) listAlgorithmProfiles(ctx context.Context, _ *struct{}) (*AlgorithmProfileListOutput, error) {
	var items []models.AlgorithmProfile
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &AlgorithmProfileListOutput{Body: items}, nil
}

func (s *Server) createAlgorithmProfile(ctx context.Context, input *AlgorithmProfileCreateInput) (*AlgorithmProfileOutput, error) {
	if err := validateProfile(input.Body); err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error(), err)
	}
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	if err := s.db.WithContext(ctx).Create(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &AlgorithmProfileOutput{Body: input.Body}, nil
}

func (s *Server) getAlgorithmProfile(ctx context.Context, input *AlgorithmProfileIDInput) (*AlgorithmProfileOutput, error) {
	var item models.AlgorithmProfile
	if err := s.db.WithContext(ctx).First(&item, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &AlgorithmProfileOutput{Body: &item}, nil
}

func (s *Server) updateAlgorithmProfile(ctx context.Context, input *AlgorithmProfileUpdateInput) (*AlgorithmProfileOutput, error) {
	if err := validateProfile(input.Body); err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error(), err)
	}
	if err := s.db.WithContext(ctx).First(&models.AlgorithmProfile{}, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	input.Body.AlgorithmProfileID = input.ID
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	if err := s.db.WithContext(ctx).Save(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return s.getAlgorithmProfile(ctx, &AlgorithmProfileIDInput{ID: input.ID})
}

func (s *Server) deleteAlgorithmProfile(ctx context.Context, input *AlgorithmProfileIDInput) (*struct{}, error) {
	if err := s.db.WithContext(ctx).Delete(&models.AlgorithmProfile{}, input.ID).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return nil, nil
}

// validateProfile checks that c1-c5 are 32 hex chars and r1-r5 are byte-aligned.
func validateProfile(p *models.AlgorithmProfile) error {
	for _, f := range []struct {
		v, name string
	}{{p.C1, "c1"}, {p.C2, "c2"}, {p.C3, "c3"}, {p.C4, "c4"}, {p.C5, "c5"}} {
		if len(f.v) != 32 {
			return fmt.Errorf("%s must be 32 hex characters (128-bit value), got %d", f.name, len(f.v))
		}
		for _, ch := range f.v {
			if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
				return fmt.Errorf("%s must be a valid hex string", f.name)
			}
		}
	}
	for _, f := range []struct {
		v    int
		name string
	}{{p.R1, "r1"}, {p.R2, "r2"}, {p.R3, "r3"}, {p.R4, "r4"}, {p.R5, "r5"}} {
		if f.v < 0 || f.v > 127 {
			return fmt.Errorf("%s must be between 0 and 127 bits", f.name)
		}
		if f.v%8 != 0 {
			return fmt.Errorf("%s=%d is not byte-aligned (must be a multiple of 8)", f.name, f.v)
		}
	}
	return nil
}
