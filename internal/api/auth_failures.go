package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// AuthFailuresResponse is the response body for GET /oam/diameter/auth_failures.
type AuthFailuresResponse struct {
	Body struct {
		Failures []AuthFailure `json:"failures"`
	}
}

func registerAuthFailureRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-auth-failures",
		Method:      http.MethodGet,
		Path:        "/oam/diameter/auth_failures",
		Summary:     "List recent S6a authentication failures",
		Tags:        []string{"OAM"},
	}, func(ctx context.Context, _ *struct{}) (*AuthFailuresResponse, error) {
		resp := &AuthFailuresResponse{}
		resp.Body.Failures = s.authFailures.RecentAuthFailures()
		return resp, nil
	})
}
