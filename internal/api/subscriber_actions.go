package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type CLRInput struct {
	IMSI string `path:"imsi" doc:"Subscriber IMSI"`
}

type CLROutput struct {
	Body struct {
		IMSI    string `json:"imsi"`
		Message string `json:"message"`
	}
}

func registerSubscriberActionRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "send-clr",
		Method:      http.MethodPost,
		Path:        "/subscriber/clr/{imsi}",
		Summary:     "Send Cancel Location Request",
		Description: "Sends a Diameter Cancel-Location-Request (CLR) with Cancellation-Type=SUBSCRIPTION_WITHDRAWAL to the subscriber's serving MME, forcing the UE to detach from the network.",
		Tags:        []string{"Subscriber"},
	}, s.sendCLR)
}

func (s *Server) sendCLR(ctx context.Context, input *CLRInput) (*CLROutput, error) {
	if s.clr == nil {
		return nil, huma.Error503ServiceUnavailable("CLR not available: Diameter layer not wired")
	}
	if err := s.clr.SendCLRByIMSI(ctx, input.IMSI); err != nil {
		// Distinguish subscriber-not-found from other errors based on message prefix.
		// A cleaner approach would be sentinel errors, but this avoids changing the
		// repository interface just for one endpoint.
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}
	out := &CLROutput{}
	out.Body.IMSI = input.IMSI
	out.Body.Message = "CLR sent successfully"
	return out, nil
}
