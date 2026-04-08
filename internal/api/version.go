package api

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/svinson1121/vectorcore-hss/internal/version"
)

type VersionResponse struct {
	Body struct {
		AppName    string `json:"app_name"    doc:"Application name"`
		AppVersion string `json:"app_version" doc:"Binary release version (set at build time)"`
		APIVersion string `json:"api_version" doc:"REST API contract version"`
	}
}

type HealthResponse struct {
	Body struct {
		Status        string  `json:"status"         doc:"Always 'ok' when the API is reachable"`
		UptimeSeconds float64 `json:"uptime_seconds" doc:"Seconds since the HSS process started"`
		Version       string  `json:"version"        doc:"Running application version"`
	}
}

func registerVersionRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-version",
		Method:      http.MethodGet,
		Path:        "/oam/version",
		Summary:     "Get version information",
		Description: "Returns the application binary version and the REST API contract version.",
		Tags:        []string{"OAM"},
	}, func(ctx context.Context, _ *struct{}) (*VersionResponse, error) {
		resp := &VersionResponse{}
		resp.Body.AppName = "VectorCore HSS"
		resp.Body.AppVersion = version.AppVersion
		resp.Body.APIVersion = version.APIVersion
		return resp, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-health",
		Method:      http.MethodGet,
		Path:        "/oam/health",
		Summary:     "Health check",
		Description: "Lightweight liveness probe. Returns 200 with status, uptime, and version whenever the API is reachable.",
		Tags:        []string{"OAM"},
	}, func(ctx context.Context, _ *struct{}) (*HealthResponse, error) {
		resp := &HealthResponse{}
		resp.Body.Status = "ok"
		resp.Body.UptimeSeconds = time.Since(version.StartTime).Seconds()
		resp.Body.Version = version.AppVersion
		return resp, nil
	})
}
