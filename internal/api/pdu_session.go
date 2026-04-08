package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

type PDUSessionListOutput struct{ Body []models.ServingPDUSession }
type PDUSessionIMSIInput struct{ IMSI string `path:"imsi"` }

func registerPDUSessionRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-pdu-sessions", Method: http.MethodGet, Path: "/oam/pdu_session", Summary: "List all active 5G PDU sessions", Tags: []string{"OAM"}}, s.listPDUSessions)
	huma.Register(api, huma.Operation{OperationID: "list-pdu-sessions-by-imsi", Method: http.MethodGet, Path: "/oam/pdu_session/imsi/{imsi}", Summary: "List 5G PDU sessions by IMSI", Tags: []string{"OAM"}}, s.listPDUSessionsByIMSI)
}

func (s *Server) listPDUSessions(ctx context.Context, _ *struct{}) (*PDUSessionListOutput, error) {
	var items []models.ServingPDUSession
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &PDUSessionListOutput{Body: items}, nil
}

func (s *Server) listPDUSessionsByIMSI(ctx context.Context, input *PDUSessionIMSIInput) (*PDUSessionListOutput, error) {
	var items []models.ServingPDUSession
	if err := s.db.WithContext(ctx).Where("imsi = ?", input.IMSI).Find(&items).Error; err != nil {
		return nil, huma.Error500InternalServerError("db error", err)
	}
	return &PDUSessionListOutput{Body: items}, nil
}
