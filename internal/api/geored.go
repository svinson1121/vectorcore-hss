package api

// geored.go -- OAM endpoints for GeoRed management.
//
// Routes (under /api/v1, protected by the existing OAM auth):
//   GET  /geored/status           — peer health, queue depth, last sync
//   POST /geored/sync             — trigger full resync with all peers
//   POST /geored/sync/{nodeId}    — trigger resync with a specific peer

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/svinson1121/vectorcore-hss/internal/geored"
)

// GeoredManager is the subset of *geored.Manager used by the OAM API.
type GeoredManager interface {
	Status() []geored.PeerStatus
	TriggerResync(ctx context.Context)
	TriggerResyncPeer(ctx context.Context, nodeID string) error
	PublishOAMPut(evType geored.EventType, record interface{})
	PublishOAMDel(evType geored.EventType, id interface{})
}

// WithGeored attaches a GeoRed manager so geored routes are registered.
func (s *Server) WithGeored(m GeoredManager) *Server {
	s.geored = m
	return s
}

type georedStatusOutput struct {
	Body []geored.PeerStatus
}

type georedSyncPeerInput struct {
	NodeID string `path:"nodeId"`
}

func registerGeoredRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "geored-status",
		Method:      http.MethodGet,
		Path:        "/geored/status",
		Summary:     "GeoRed peer status",
		Tags:        []string{"GeoRed"},
	}, s.georedStatus)

	huma.Register(api, huma.Operation{
		OperationID:   "geored-sync-all",
		Method:        http.MethodPost,
		Path:          "/geored/sync",
		Summary:       "Trigger full resync with all peers",
		Tags:          []string{"GeoRed"},
		DefaultStatus: http.StatusAccepted,
	}, s.georedSyncAll)

	huma.Register(api, huma.Operation{
		OperationID:   "geored-sync-peer",
		Method:        http.MethodPost,
		Path:          "/geored/sync/{nodeId}",
		Summary:       "Trigger resync with a specific peer",
		Tags:          []string{"GeoRed"},
		DefaultStatus: http.StatusNoContent,
	}, s.georedSyncPeer)
}

func (s *Server) georedStatus(ctx context.Context, _ *struct{}) (*georedStatusOutput, error) {
	if s.geored == nil {
		return nil, huma.Error503ServiceUnavailable("geored not enabled", nil)
	}
	return &georedStatusOutput{Body: s.geored.Status()}, nil
}

func (s *Server) georedSyncAll(ctx context.Context, _ *struct{}) (*struct{}, error) {
	if s.geored == nil {
		return nil, huma.Error503ServiceUnavailable("geored not enabled", nil)
	}
	s.geored.TriggerResync(ctx)
	return nil, nil
}

func (s *Server) georedSyncPeer(ctx context.Context, input *georedSyncPeerInput) (*struct{}, error) {
	if s.geored == nil {
		return nil, huma.Error503ServiceUnavailable("geored not enabled", nil)
	}
	if err := s.geored.TriggerResyncPeer(ctx, input.NodeID); err != nil {
		return nil, huma.Error404NotFound("peer not found or sync failed", err)
	}
	return nil, nil
}
