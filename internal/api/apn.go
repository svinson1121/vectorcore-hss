package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"

	"github.com/svinson1121/vectorcore-hss/internal/geored"
	"github.com/svinson1121/vectorcore-hss/internal/models"
)

type APNListOutput struct{ Body []models.APN }
type APNOutput struct{ Body *models.APN }
type APNIDInput struct {
	ID int `path:"id"`
}
type APNCreateInput struct{ Body *models.APN }
type APNUpdateInput struct {
	ID   int `path:"id"`
	Body *models.APN
}

func registerAPNRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-apn", Method: http.MethodGet, Path: "/apn", Summary: "List APNs", Tags: []string{"APN"}}, s.listAPNs)
	huma.Register(api, huma.Operation{OperationID: "create-apn", Method: http.MethodPost, Path: "/apn", Summary: "Create APN", Tags: []string{"APN"}, DefaultStatus: http.StatusCreated}, s.createAPN)
	huma.Register(api, huma.Operation{OperationID: "get-apn", Method: http.MethodGet, Path: "/apn/{id}", Summary: "Get APN", Tags: []string{"APN"}}, s.getAPN)
	huma.Register(api, huma.Operation{OperationID: "update-apn", Method: http.MethodPut, Path: "/apn/{id}", Summary: "Update APN", Tags: []string{"APN"}}, s.updateAPN)
	huma.Register(api, huma.Operation{OperationID: "delete-apn", Method: http.MethodDelete, Path: "/apn/{id}", Summary: "Delete APN", Tags: []string{"APN"}}, s.deleteAPN)
}

func (s *Server) listAPNs(ctx context.Context, _ *struct{}) (*APNListOutput, error) {
	var items []models.APN
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &APNListOutput{Body: items}, nil
}

// validateAPNCiot enforces the TS 29.272 §7.3.204-209/§7.3.222 dependency
// rules for APN-level CIoT/NIDD configuration: NIDD fields are only
// meaningful when Non-IP PDN is enabled, SCEF-ID/Realm/RDS only apply to the
// SCEF-based delivery mechanism, and Preferred-Data-Mode must set at least
// one of its two defined bits when present (spec NOTE 2 under §7.3.209).
// Rejecting nonsensical combinations here mirrors validateAccessRestrictionData
// in subscriber.go: don't silently reinterpret or drop an admin's input.
func validateAPNCiot(a *models.APN) error {
	nonIPPDN := a.NonIPPDN != nil && *a.NonIPPDN
	niddSet := (a.NIDDMechanism != nil) || (a.NIDDScefID != nil && *a.NIDDScefID != "") ||
		(a.NIDDScefRealm != nil && *a.NIDDScefRealm != "") || (a.NIDDRDS != nil)
	if !nonIPPDN && niddSet {
		return fmt.Errorf("nidd_mechanism/nidd_scef_id/nidd_scef_realm/nidd_rds require non_ip_pdn to be enabled (TS 29.272 §7.3.205/§7.3.207/§7.3.222)")
	}
	if a.NIDDMechanism != nil {
		switch *a.NIDDMechanism {
		case models.NIDDMechanismSGiBased, models.NIDDMechanismSCEFBased:
		default:
			return fmt.Errorf("nidd_mechanism must be 0 (SGi-based) or 1 (SCEF-based), got %d", *a.NIDDMechanism)
		}
		scefBased := *a.NIDDMechanism == models.NIDDMechanismSCEFBased
		hasScefID := a.NIDDScefID != nil && *a.NIDDScefID != ""
		hasScefRealm := a.NIDDScefRealm != nil && *a.NIDDScefRealm != ""
		if !scefBased && (hasScefID || hasScefRealm) {
			return fmt.Errorf("nidd_scef_id/nidd_scef_realm require nidd_mechanism to be 1 (SCEF-based)")
		}
	}
	if a.NIDDRDS != nil {
		switch *a.NIDDRDS {
		case models.RDSDisabled, models.RDSEnabled:
		default:
			return fmt.Errorf("nidd_rds must be 0 (disabled) or 1 (enabled), got %d", *a.NIDDRDS)
		}
	}
	if a.NIDDPreferredDataMode != nil {
		mode := *a.NIDDPreferredDataMode
		if mode == 0 || mode&^models.PreferredDataModeValidMask != 0 {
			return fmt.Errorf("nidd_preferred_data_mode must set at least one of bit 0 (User Plane) or bit 1 (Control Plane) and no other bits, got %d", mode)
		}
	}
	return nil
}

func (s *Server) createAPN(ctx context.Context, input *APNCreateInput) (*APNOutput, error) {
	if err := validateAPNCiot(input.Body); err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error(), err)
	}
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	if err := s.db.WithContext(ctx).Create(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if s.geored != nil {
		s.geored.PublishOAMPut(geored.EventAPNPut, input.Body)
	}
	return &APNOutput{Body: input.Body}, nil
}

func (s *Server) getAPN(ctx context.Context, input *APNIDInput) (*APNOutput, error) {
	var item models.APN
	if err := s.db.WithContext(ctx).First(&item, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &APNOutput{Body: &item}, nil
}

func (s *Server) updateAPN(ctx context.Context, input *APNUpdateInput) (*APNOutput, error) {
	if err := s.db.WithContext(ctx).First(&models.APN{}, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if err := validateAPNCiot(input.Body); err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error(), err)
	}
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	input.Body.APNID = input.ID
	if err := s.db.WithContext(ctx).Save(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if s.geored != nil {
		s.geored.PublishOAMPut(geored.EventAPNPut, input.Body)
	}
	return s.getAPN(ctx, &APNIDInput{ID: input.ID})
}

func (s *Server) deleteAPN(ctx context.Context, input *APNIDInput) (*struct{}, error) {
	if imsi, err := firstString(ctx, s.db, &models.Subscriber{}, "imsi", "default_apn = ?", input.ID); err == nil {
		return nil, conflictInUse("APN", strconv.Itoa(input.ID), "subscriber default APN", imsi)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	where, token := csvContainsID("apn_list", input.ID)
	if imsi, err := firstString(ctx, s.db, &models.Subscriber{}, "imsi", where, token); err == nil {
		return nil, conflictInUse("APN", strconv.Itoa(input.ID), "subscriber APN list", imsi)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	var routing models.SubscriberRouting
	if err := s.db.WithContext(ctx).Where("apn_id = ?", input.ID).Take(&routing).Error; err == nil {
		return nil, conflictInUse("APN", strconv.Itoa(input.ID), "subscriber routing", strconv.Itoa(routing.SubscriberRoutingID))
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if err := s.db.WithContext(ctx).Delete(&models.APN{}, input.ID).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if s.geored != nil {
		s.geored.PublishOAMDel(geored.EventAPNDel, input.ID)
	}
	return nil, nil
}
