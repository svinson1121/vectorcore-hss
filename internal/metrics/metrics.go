// Package metrics provides Prometheus instrumentation for VectorCore HSS.
package metrics

import (
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// DiameterRequestsTotal counts Diameter requests by command and result.
	DiameterRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "hss",
		Subsystem: "diameter",
		Name:      "requests_total",
		Help:      "Total number of Diameter requests processed, split by command and result (success/error).",
	}, []string{"command", "result"})

	// DiameterRequestDuration measures end-to-end handler latency per command.
	DiameterRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "hss",
		Subsystem: "diameter",
		Name:      "request_duration_seconds",
		Help:      "Diameter request processing duration in seconds (from recv to send).",
		Buckets:   []float64{.001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	}, []string{"command"})

	// DBQueryDuration measures database query latency by operation and table.
	DBQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "hss",
		Subsystem: "db",
		Name:      "query_duration_seconds",
		Help:      "Database query duration in seconds, split by operation (query/create/update/delete) and table.",
		Buckets:   []float64{.0005, .001, .002, .005, .01, .025, .05, .1, .25, .5, 1},
	}, []string{"operation", "table"})

	// APIRequestsTotal counts REST API requests by method, path, and HTTP status.
	APIRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "hss",
		Subsystem: "api",
		Name:      "requests_total",
		Help:      "Total number of OAM API requests, split by method, path, and HTTP status code.",
	}, []string{"method", "path", "status"})

	// APIRequestDuration measures OAM API handler latency by method and path.
	APIRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "hss",
		Subsystem: "api",
		Name:      "request_duration_seconds",
		Help:      "OAM API request duration in seconds, split by method and path.",
		Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	}, []string{"method", "path"})

	// VectorGenerationDuration measures Milenage vector generation latency.
	// The "type" label is "eutran" (S6a AIR) or "eap_aka" (SWx/Cx/Zh MAR).
	VectorGenerationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "hss",
		Subsystem: "crypto",
		Name:      "vector_generation_seconds",
		Help:      "Time spent generating authentication vectors (Milenage), split by type.",
		Buckets:   []float64{.0001, .0005, .001, .002, .005, .01, .025, .05, .1},
	}, []string{"type"})

	// SubscriberCacheHits counts cache hits vs misses per entity type.
	SubscriberCacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "hss",
		Subsystem: "cache",
		Name:      "hits_total",
		Help:      "Cache lookup results (hit or miss) per entity type.",
	}, []string{"entity", "result"})

	// TACLookupsTotal counts Type Allocation Code lookups by result.
	// "hit" = IMEI matched a known device; "miss" = no TAC entry found.
	TACLookupsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "hss",
		Subsystem: "tac",
		Name:      "lookups_total",
		Help:      "Total Type Allocation Code lookups (IMEI → device make/model), split by result (hit/miss).",
	}, []string{"result"})

	// TACCacheSize is the current number of entries in the in-memory TAC cache.
	TACCacheSize = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "hss",
		Subsystem: "tac",
		Name:      "cache_size",
		Help:      "Number of Type Allocation Code entries currently loaded in memory.",
	})

	// TACImportedTotal counts the cumulative number of records written during
	// CSV imports (inserted + updated combined).
	TACImportedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "hss",
		Subsystem: "tac",
		Name:      "imported_total",
		Help:      "Cumulative number of TAC records written (inserted or updated) via the CSV import API.",
	})
)

// DBPoolCollector is a Prometheus collector that reads live connection pool
// stats from a *sql.DB and exposes them as gauges.
type DBPoolCollector struct {
	db   *sql.DB
	open *prometheus.Desc
	inUse *prometheus.Desc
	idle  *prometheus.Desc
	waitCount    *prometheus.Desc
	waitDuration *prometheus.Desc
}

// NewDBPoolCollector creates a collector for the given *sql.DB.
// Register it with prometheus.MustRegister after opening the DB.
func NewDBPoolCollector(db *sql.DB) *DBPoolCollector {
	return &DBPoolCollector{
		db: db,
		open: prometheus.NewDesc(
			"hss_db_pool_open_connections",
			"Number of open connections in the database pool (in-use + idle).",
			nil, nil,
		),
		inUse: prometheus.NewDesc(
			"hss_db_pool_in_use_connections",
			"Number of connections currently in use by the application.",
			nil, nil,
		),
		idle: prometheus.NewDesc(
			"hss_db_pool_idle_connections",
			"Number of idle connections waiting in the pool.",
			nil, nil,
		),
		waitCount: prometheus.NewDesc(
			"hss_db_pool_wait_count_total",
			"Total number of times the pool was exhausted and a goroutine had to wait.",
			nil, nil,
		),
		waitDuration: prometheus.NewDesc(
			"hss_db_pool_wait_duration_seconds_total",
			"Total time spent waiting for a connection from the pool.",
			nil, nil,
		),
	}
}

func (c *DBPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.open
	ch <- c.inUse
	ch <- c.idle
	ch <- c.waitCount
	ch <- c.waitDuration
}

func (c *DBPoolCollector) Collect(ch chan<- prometheus.Metric) {
	s := c.db.Stats()
	ch <- prometheus.MustNewConstMetric(c.open, prometheus.GaugeValue, float64(s.OpenConnections))
	ch <- prometheus.MustNewConstMetric(c.inUse, prometheus.GaugeValue, float64(s.InUse))
	ch <- prometheus.MustNewConstMetric(c.idle, prometheus.GaugeValue, float64(s.Idle))
	ch <- prometheus.MustNewConstMetric(c.waitCount, prometheus.CounterValue, float64(s.WaitCount))
	ch <- prometheus.MustNewConstMetric(c.waitDuration, prometheus.CounterValue, s.WaitDuration.Seconds())
}
