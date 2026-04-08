package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// DiameterPeersResponse is the response body for GET /diameter/peers.
type DiameterPeersResponse struct {
	Body struct {
		Peers []ConnectedPeer `json:"peers"`
	}
}

func registerDiameterPeersRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-diameter-peers",
		Method:      http.MethodGet,
		Path:        "/oam/diameter/peers",
		Summary:     "List Diameter peers",
		Tags:        []string{"OAM"},
	}, func(ctx context.Context, _ *struct{}) (*DiameterPeersResponse, error) {
		resp := &DiameterPeersResponse{}
		resp.Body.Peers = s.peers.List()
		return resp, nil
	})
}
