package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type ServicePeersResponse struct {
	Body struct {
		Peers []ServicePeer `json:"peers"`
	}
}

func registerGSUPPeersRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-gsup-peers",
		Method:      http.MethodGet,
		Path:        "/oam/gsup/peers",
		Summary:     "List GSUP peers",
		Tags:        []string{"OAM"},
	}, func(ctx context.Context, _ *struct{}) (*ServicePeersResponse, error) {
		resp := &ServicePeersResponse{}
		resp.Body.Peers = s.gsupPeers.List()
		return resp, nil
	})
}

func registerSBIPeersRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-sbi-peers",
		Method:      http.MethodGet,
		Path:        "/oam/sbi/peers",
		Summary:     "List SBI peers",
		Tags:        []string{"OAM"},
	}, func(ctx context.Context, _ *struct{}) (*ServicePeersResponse, error) {
		resp := &ServicePeersResponse{}
		resp.Body.Peers = s.sbiPeers.List()
		return resp, nil
	})
}
