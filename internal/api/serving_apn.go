package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

type ServingAPNListOutput struct{ Body []models.ServingAPN }
type ServingAPNOutput struct{ Body *models.ServingAPN }
type ServingAPNIMSIInput struct{ IMSI string `path:"imsi"` }
type ServingAPNMSISDNInput struct{ MSISDN string `path:"msisdn"` }
type ServingAPNIPInput struct{ IP string `path:"ip"` }

func registerServingAPNRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-serving-apn", Method: http.MethodGet, Path: "/oam/serving_apn", Summary: "List all Serving APNs", Tags: []string{"OAM"}}, s.listServingAPNs)
	huma.Register(api, huma.Operation{OperationID: "get-serving-apn-by-imsi", Method: http.MethodGet, Path: "/oam/serving_apn/imsi/{imsi}", Summary: "Get Serving APN by IMSI", Tags: []string{"OAM"}}, s.getServingAPNByIMSI)
	huma.Register(api, huma.Operation{OperationID: "get-serving-apn-by-msisdn", Method: http.MethodGet, Path: "/oam/serving_apn/msisdn/{msisdn}", Summary: "Get Serving APN by MSISDN", Tags: []string{"OAM"}}, s.getServingAPNByMSISDN)
	huma.Register(api, huma.Operation{OperationID: "get-serving-apn-by-ip", Method: http.MethodGet, Path: "/oam/serving_apn/ip/{ip}", Summary: "Get Serving APN by UE IP", Tags: []string{"OAM"}}, s.getServingAPNByIP)
}

func (s *Server) listServingAPNs(ctx context.Context, _ *struct{}) (*ServingAPNListOutput, error) {
	var items []models.ServingAPN
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &ServingAPNListOutput{Body: items}, nil
}

func (s *Server) getServingAPNByIMSI(ctx context.Context, input *ServingAPNIMSIInput) (*ServingAPNOutput, error) {
	var sub models.Subscriber
	if err := s.db.WithContext(ctx).Where("imsi = ?", input.IMSI).First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("subscriber not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	var rec models.ServingAPN
	if err := s.db.WithContext(ctx).Where("subscriber_id = ?", sub.SubscriberID).First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("no active serving APN", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &ServingAPNOutput{Body: &rec}, nil
}

func (s *Server) getServingAPNByMSISDN(ctx context.Context, input *ServingAPNMSISDNInput) (*ServingAPNOutput, error) {
	var sub models.Subscriber
	if err := s.db.WithContext(ctx).Where("msisdn = ?", input.MSISDN).First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("subscriber not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	var rec models.ServingAPN
	if err := s.db.WithContext(ctx).Where("subscriber_id = ?", sub.SubscriberID).First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("no active serving APN", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &ServingAPNOutput{Body: &rec}, nil
}

func (s *Server) getServingAPNByIP(ctx context.Context, input *ServingAPNIPInput) (*ServingAPNOutput, error) {
	var rec models.ServingAPN
	if err := s.db.WithContext(ctx).Where("ue_ip = ?", input.IP).First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, huma.Error404NotFound("not found", err)
		}
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &ServingAPNOutput{Body: &rec}, nil
}
