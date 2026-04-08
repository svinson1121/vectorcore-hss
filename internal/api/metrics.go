package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/client_golang/prometheus"
)

// ── Payload types ─────────────────────────────────────────────────────────────

type MetricsResponse struct {
	Body *MetricsPayload
}

type MetricsPayload struct {
	Timestamp time.Time       `json:"timestamp"  doc:"UTC time the snapshot was taken"`
	Diameter  DiameterMetrics `json:"diameter"`
	Database  DatabaseMetrics `json:"database"`
	Cache     CacheMetrics    `json:"cache"`
	API       APIMetrics      `json:"api"`
	Crypto    CryptoMetrics   `json:"crypto"`
	TAC       TACMetrics      `json:"tac"`
}

// TACMetrics summarises the Type Allocation Code device database.
type TACMetrics struct {
	CacheSize    float64  `json:"cache_size"     doc:"Number of TAC entries currently in memory"`
	ImportedTotal float64 `json:"imported_total" doc:"Cumulative TAC records written via CSV import"`
	Lookups      []TACLookupStat `json:"lookups" doc:"Lookup result breakdown"`
}

type TACLookupStat struct {
	Result string  `json:"result" doc:"hit or miss"`
	Count  float64 `json:"count"`
}

// DiameterMetrics summarises traffic per Diameter command.
type DiameterMetrics struct {
	Commands []DiameterCommand `json:"commands" doc:"Per-command request summary"`
}

type DiameterCommand struct {
	Command      string  `json:"command"`
	Total        float64 `json:"total"          doc:"Total requests (success + error)"`
	Errors       float64 `json:"errors"         doc:"Error count"`
	ErrorRatePct float64 `json:"error_rate_pct" doc:"Error percentage (0-100)"`
	MeanMs       float64 `json:"mean_ms"        doc:"Mean processing time in milliseconds"`
}

// DatabaseMetrics covers query latency and connection pool health.
type DatabaseMetrics struct {
	Queries []DBQueryStat `json:"queries" doc:"Per-operation query latency summary"`
	Pool    DBPool        `json:"pool"    doc:"Connection pool state"`
}

type DBQueryStat struct {
	Operation string  `json:"operation" doc:"GORM operation: query / create / update / delete / raw"`
	Table     string  `json:"table"     doc:"Database table"`
	Count     uint64  `json:"count"     doc:"Number of queries observed"`
	MeanMs    float64 `json:"mean_ms"   doc:"Mean query duration in milliseconds"`
	SumMs     float64 `json:"sum_ms"    doc:"Total cumulative query time in milliseconds"`
}

type DBPool struct {
	OpenConnections int     `json:"open_connections" doc:"Total open connections (in-use + idle)"`
	InUse           int     `json:"in_use"           doc:"Connections currently executing a query"`
	Idle            int     `json:"idle"             doc:"Connections waiting in the pool"`
	WaitCount       int64   `json:"wait_count"       doc:"Times the pool was exhausted and a caller had to wait"`
	WaitDurationMs  float64 `json:"wait_duration_ms" doc:"Total time spent waiting for a pool connection (ms)"`
}

// CacheMetrics summarises the in-memory subscriber/AUC cache.
type CacheMetrics struct {
	Entities []CacheEntity `json:"entities" doc:"Per-entity cache hit/miss summary"`
}

type CacheEntity struct {
	Entity     string  `json:"entity"`
	Hits       float64 `json:"hits"`
	Misses     float64 `json:"misses"`
	HitRatePct float64 `json:"hit_rate_pct" doc:"Hit percentage (0-100). 0 if no lookups yet."`
}

// APIMetrics summarises OAM REST API traffic.
// Internal paths (docs, openapi, schemas, /metrics) are excluded.
type APIMetrics struct {
	Requests []APIRequestStat `json:"requests" doc:"Per-path request summary (internal/meta paths excluded)"`
}

type APIRequestStat struct {
	Method  string  `json:"method"`
	Path    string  `json:"path"`
	Total   float64 `json:"total"   doc:"Total requests"`
	MeanMs  float64 `json:"mean_ms" doc:"Mean response time in milliseconds"`
}

// CryptoMetrics summarises Milenage vector generation performance.
type CryptoMetrics struct {
	Vectors []VectorStat `json:"vectors" doc:"Per-type vector generation latency"`
}

type VectorStat struct {
	Type   string  `json:"type"     doc:"Vector type: eutran (S6a AIR) or eap_aka (SWx/Cx/Zh MAR)"`
	Count  uint64  `json:"count"    doc:"Total vectors generated"`
	MeanMs float64 `json:"mean_ms"  doc:"Mean generation time in milliseconds"`
	SumMs  float64 `json:"sum_ms"   doc:"Total cumulative generation time in milliseconds"`
}

// ── Route registration ─────────────────────────────────────────────────────────

func registerMetricsRoutes(s *Server, api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-metrics",
		Method:      http.MethodGet,
		Path:        "/oam/metrics",
		Summary:     "Get HSS metrics snapshot",
		Description: "Returns a structured JSON snapshot of Diameter, database, cache, and API metrics. Suitable for dashboard consumption without a Prometheus scraper.",
		Tags:        []string{"OAM"},
	}, func(ctx context.Context, _ *struct{}) (*MetricsResponse, error) {
		payload, err := gatherMetrics()
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to gather metrics", err)
		}
		return &MetricsResponse{Body: payload}, nil
	})
}

// ── Gathering ─────────────────────────────────────────────────────────────────

// isNoisePath returns true for internal meta-paths that pollute the API stats.
func isNoisePath(path string) bool {
	for _, prefix := range []string{
		"/api/v1/docs",
		"/api/v1/openapi",
		"/api/v1/schemas",
		"/metrics",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func gatherMetrics() (*MetricsPayload, error) {
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return nil, err
	}

	p := &MetricsPayload{Timestamp: time.Now().UTC()}

	// Intermediate maps for aggregation.
	type diamKey struct{ cmd, result string }
	diamCounts := map[string]map[string]float64{}  // cmd → result → count
	diamSum    := map[string]float64{}              // cmd → duration sum (seconds)
	diamCount  := map[string]uint64{}               // cmd → sample count

	type dbKey struct{ op, table string }
	dbSum   := map[dbKey]float64{}
	dbCount := map[dbKey]uint64{}

	type cacheKey struct{ entity, result string }
	cacheVals := map[cacheKey]float64{}

	type apiKey struct{ method, path string }
	apiTotal   := map[apiKey]float64{}
	apiSum     := map[apiKey]float64{}
	apiSamples := map[apiKey]uint64{}

	cryptoSum   := map[string]float64{}
	cryptoCount := map[string]uint64{}

	tacLookups := map[string]float64{} // result → count

	for _, mf := range families {
		name := mf.GetName()
		switch name {

		case "hss_diameter_requests_total":
			for _, m := range mf.GetMetric() {
				cmd := labelValue(m, "command")
				res := labelValue(m, "result")
				if diamCounts[cmd] == nil {
					diamCounts[cmd] = map[string]float64{}
				}
				diamCounts[cmd][res] += m.GetCounter().GetValue()
			}

		case "hss_diameter_request_duration_seconds":
			for _, m := range mf.GetMetric() {
				cmd := labelValue(m, "command")
				h := m.GetHistogram()
				diamSum[cmd] += h.GetSampleSum()
				diamCount[cmd] += h.GetSampleCount()
			}

		case "hss_db_query_duration_seconds":
			for _, m := range mf.GetMetric() {
				k := dbKey{labelValue(m, "operation"), labelValue(m, "table")}
				h := m.GetHistogram()
				dbSum[k] += h.GetSampleSum()
				dbCount[k] += h.GetSampleCount()
			}

		case "hss_cache_hits_total":
			for _, m := range mf.GetMetric() {
				k := cacheKey{labelValue(m, "entity"), labelValue(m, "result")}
				cacheVals[k] += m.GetCounter().GetValue()
			}

		case "hss_api_requests_total":
			for _, m := range mf.GetMetric() {
				path := labelValue(m, "path")
				if isNoisePath(path) {
					continue
				}
				k := apiKey{labelValue(m, "method"), path}
				apiTotal[k] += m.GetCounter().GetValue()
			}

		case "hss_api_request_duration_seconds":
			for _, m := range mf.GetMetric() {
				path := labelValue(m, "path")
				if isNoisePath(path) {
					continue
				}
				k := apiKey{labelValue(m, "method"), path}
				h := m.GetHistogram()
				apiSum[k] += h.GetSampleSum()
				apiSamples[k] += h.GetSampleCount()
			}

		case "hss_crypto_vector_generation_seconds":
			for _, m := range mf.GetMetric() {
				t := labelValue(m, "type")
				h := m.GetHistogram()
				cryptoSum[t] += h.GetSampleSum()
				cryptoCount[t] += h.GetSampleCount()
			}

		case "hss_tac_lookups_total":
			for _, m := range mf.GetMetric() {
				tacLookups[labelValue(m, "result")] += m.GetCounter().GetValue()
			}

		case "hss_tac_cache_size":
			if len(mf.GetMetric()) > 0 {
				p.TAC.CacheSize = mf.GetMetric()[0].GetGauge().GetValue()
			}

		case "hss_tac_imported_total":
			if len(mf.GetMetric()) > 0 {
				p.TAC.ImportedTotal = mf.GetMetric()[0].GetCounter().GetValue()
			}

		// DB pool gauges/counters registered by DBPoolCollector.
		case "hss_db_pool_open_connections":
			if len(mf.GetMetric()) > 0 {
				p.Database.Pool.OpenConnections = int(mf.GetMetric()[0].GetGauge().GetValue())
			}
		case "hss_db_pool_in_use_connections":
			if len(mf.GetMetric()) > 0 {
				p.Database.Pool.InUse = int(mf.GetMetric()[0].GetGauge().GetValue())
			}
		case "hss_db_pool_idle_connections":
			if len(mf.GetMetric()) > 0 {
				p.Database.Pool.Idle = int(mf.GetMetric()[0].GetGauge().GetValue())
			}
		case "hss_db_pool_wait_count_total":
			if len(mf.GetMetric()) > 0 {
				p.Database.Pool.WaitCount = int64(mf.GetMetric()[0].GetCounter().GetValue())
			}
		case "hss_db_pool_wait_duration_seconds_total":
			if len(mf.GetMetric()) > 0 {
				p.Database.Pool.WaitDurationMs = mf.GetMetric()[0].GetCounter().GetValue() * 1000
			}
		}
	}

	// Build Diameter commands summary.
	for cmd, results := range diamCounts {
		var total, errors float64
		for res, count := range results {
			total += count
			if res == "error" {
				errors += count
			}
		}
		var meanMs float64
		if c := diamCount[cmd]; c > 0 {
			meanMs = (diamSum[cmd] / float64(c)) * 1000
		}
		errPct := 0.0
		if total > 0 {
			errPct = (errors / total) * 100
		}
		p.Diameter.Commands = append(p.Diameter.Commands, DiameterCommand{
			Command:      cmd,
			Total:        total,
			Errors:       errors,
			ErrorRatePct: round2(errPct),
			MeanMs:       round2(meanMs),
		})
	}

	// Build DB query summary.
	for k, count := range dbCount {
		meanMs := 0.0
		if count > 0 {
			meanMs = (dbSum[k] / float64(count)) * 1000
		}
		p.Database.Queries = append(p.Database.Queries, DBQueryStat{
			Operation: k.op,
			Table:     k.table,
			Count:     count,
			MeanMs:    round2(meanMs),
			SumMs:     round2(dbSum[k] * 1000),
		})
	}

	// Build cache summary.
	entities := map[string]*CacheEntity{}
	for k, v := range cacheVals {
		e := entities[k.entity]
		if e == nil {
			e = &CacheEntity{Entity: k.entity}
			entities[k.entity] = e
		}
		if k.result == "hit" {
			e.Hits += v
		} else {
			e.Misses += v
		}
	}
	for _, e := range entities {
		total := e.Hits + e.Misses
		if total > 0 {
			e.HitRatePct = round2((e.Hits / total) * 100)
		}
		p.Cache.Entities = append(p.Cache.Entities, *e)
	}

	// Build API request summary.
	for k, total := range apiTotal {
		meanMs := 0.0
		if c := apiSamples[k]; c > 0 {
			meanMs = (apiSum[k] / float64(c)) * 1000
		}
		p.API.Requests = append(p.API.Requests, APIRequestStat{
			Method: k.method,
			Path:   k.path,
			Total:  total,
			MeanMs: round2(meanMs),
		})
	}

	// Build TAC lookup summary.
	for result, count := range tacLookups {
		p.TAC.Lookups = append(p.TAC.Lookups, TACLookupStat{Result: result, Count: count})
	}

	// Build crypto vector generation summary.
	for vecType, count := range cryptoCount {
		meanMs := 0.0
		if count > 0 {
			meanMs = (cryptoSum[vecType] / float64(count)) * 1000
		}
		p.Crypto.Vectors = append(p.Crypto.Vectors, VectorStat{
			Type:   vecType,
			Count:  count,
			MeanMs: round2(meanMs),
			SumMs:  round2(cryptoSum[vecType] * 1000),
		})
	}

	return p, nil
}

// labelValue returns the value of a Prometheus label by name.
func labelValue(m *dto.Metric, name string) string {
	for _, lp := range m.GetLabel() {
		if lp.GetName() == name {
			return lp.GetValue()
		}
	}
	return ""
}

// round2 rounds a float to 2 decimal places.
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
