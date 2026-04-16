package api

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

type IFCProfileListOutput struct{ Body []models.IFCProfile }
type IFCProfileOutput struct{ Body *models.IFCProfile }
type IFCProfileIDInput struct {
	ID int `path:"id"`
}
type IFCProfileCreateInput struct{ Body *models.IFCProfile }
type IFCProfileUpdateInput struct {
	ID   int `path:"id"`
	Body *models.IFCProfile
}

func registerIFCProfileRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-ifc-profile", Method: http.MethodGet, Path: "/ims_subscriber/ifc_profile", Summary: "List IFC Profiles", Tags: []string{"IMS Subscriber"}}, s.listIFCProfiles)
	huma.Register(api, huma.Operation{OperationID: "create-ifc-profile", Method: http.MethodPost, Path: "/ims_subscriber/ifc_profile", Summary: "Create IFC Profile", Tags: []string{"IMS Subscriber"}, DefaultStatus: http.StatusCreated}, s.createIFCProfile)
	huma.Register(api, huma.Operation{OperationID: "get-ifc-profile", Method: http.MethodGet, Path: "/ims_subscriber/ifc_profile/{id}", Summary: "Get IFC Profile", Tags: []string{"IMS Subscriber"}}, s.getIFCProfile)
	huma.Register(api, huma.Operation{OperationID: "update-ifc-profile", Method: http.MethodPut, Path: "/ims_subscriber/ifc_profile/{id}", Summary: "Update IFC Profile", Tags: []string{"IMS Subscriber"}}, s.updateIFCProfile)
	huma.Register(api, huma.Operation{OperationID: "delete-ifc-profile", Method: http.MethodDelete, Path: "/ims_subscriber/ifc_profile/{id}", Summary: "Delete IFC Profile", Tags: []string{"IMS Subscriber"}}, s.deleteIFCProfile)
}

func (s *Server) listIFCProfiles(ctx context.Context, _ *struct{}) (*IFCProfileListOutput, error) {
	var items []models.IFCProfile
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &IFCProfileListOutput{Body: items}, nil
}

func (s *Server) createIFCProfile(ctx context.Context, input *IFCProfileCreateInput) (*IFCProfileOutput, error) {
	if err := validateIFCProfileXML(input.Body.XMLData); err != nil {
		return nil, err
	}
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	if err := s.db.WithContext(ctx).Create(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &IFCProfileOutput{Body: input.Body}, nil
}

func (s *Server) getIFCProfile(ctx context.Context, input *IFCProfileIDInput) (*IFCProfileOutput, error) {
	var item models.IFCProfile
	if err := s.db.WithContext(ctx).First(&item, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &IFCProfileOutput{Body: &item}, nil
}

func (s *Server) updateIFCProfile(ctx context.Context, input *IFCProfileUpdateInput) (*IFCProfileOutput, error) {
	if err := s.db.WithContext(ctx).First(&models.IFCProfile{}, input.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if err := validateIFCProfileXML(input.Body.XMLData); err != nil {
		return nil, err
	}
	input.Body.LastModified = time.Now().UTC().Format(time.RFC3339)
	input.Body.IFCProfileID = input.ID
	if err := s.db.WithContext(ctx).Save(input.Body).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return s.getIFCProfile(ctx, &IFCProfileIDInput{ID: input.ID})
}

func (s *Server) deleteIFCProfile(ctx context.Context, input *IFCProfileIDInput) (*struct{}, error) {
	if msisdn, err := firstString(ctx, s.db, &models.IMSSubscriber{}, "msisdn", "ifc_profile_id = ?", input.ID); err == nil {
		return nil, conflictInUse("IFC profile", strconv.Itoa(input.ID), "IMS subscriber", msisdn)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	if err := s.db.WithContext(ctx).Delete(&models.IFCProfile{}, input.ID).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return nil, nil
}

func validateIFCProfileXML(xmlData string) error {
	if err := validateIFCProfileXMLFragment(xmlData); err != nil {
		return huma.Error400BadRequest("invalid IFC XML", err)
	}
	return nil
}

func validateIFCProfileXMLFragment(xmlData string) error {
	trimmed := strings.TrimSpace(xmlData)
	if trimmed == "" {
		return fmt.Errorf("xml_data must not be empty")
	}
	if strings.HasPrefix(trimmed, "<?xml") {
		return fmt.Errorf("xml_data must not include an XML declaration")
	}

	decoder := xml.NewDecoder(strings.NewReader("<root>" + trimmed + "</root>"))
	for {
		tok, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("xml_data must be well-formed XML fragments: %w", err)
		}

		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		switch start.Name.Local {
		case "IMSSubscription", "PrivateID", "ServiceProfile":
			return fmt.Errorf("xml_data must only contain inner ServiceProfile fragments, not <%s>", start.Name.Local)
		}
	}
}
