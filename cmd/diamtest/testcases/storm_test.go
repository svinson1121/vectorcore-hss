package testcases

import "testing"

// TestParseMetric locks in parseMetric's behavior against real
// client_golang Prometheus exposition output (captured from a live
// /metrics response), independent of storm-run timing.
func TestParseMetric(t *testing.T) {
	body := `# HELP hss_admission_in_flight Number of Diameter handler executions currently holding an admission-control slot, split by application.
# TYPE hss_admission_in_flight gauge
hss_admission_in_flight{app="gx"} 3
hss_admission_in_flight{app="s6a"} 12
# HELP hss_db_pool_in_use_connections Number of connections currently in use by the application.
# TYPE hss_db_pool_in_use_connections gauge
hss_db_pool_in_use_connections 15
# HELP hss_db_pool_wait_count_total Total number of times the pool was exhausted and a goroutine had to wait.
# TYPE hss_db_pool_wait_count_total counter
hss_db_pool_wait_count_total 42
`
	tests := []struct {
		name   string
		metric string
		labels map[string]string
		want   float64
	}{
		{"unlabeled gauge", "hss_db_pool_in_use_connections", nil, 15},
		{"unlabeled counter", "hss_db_pool_wait_count_total", nil, 42},
		{"labeled gx", "hss_admission_in_flight", map[string]string{"app": "gx"}, 3},
		{"labeled s6a", "hss_admission_in_flight", map[string]string{"app": "s6a"}, 12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseMetric(body, tt.metric, tt.labels)
			if !ok {
				t.Fatalf("parseMetric(%q, %v) not found", tt.metric, tt.labels)
			}
			if got != tt.want {
				t.Errorf("parseMetric(%q, %v) = %v, want %v", tt.metric, tt.labels, got, tt.want)
			}
		})
	}

	if _, ok := parseMetric(body, "hss_admission_in_flight", map[string]string{"app": "nonexistent"}); ok {
		t.Error("parseMetric matched a label value that isn't present")
	}
	if _, ok := parseMetric(body, "hss_nonexistent_metric", nil); ok {
		t.Error("parseMetric matched a metric name that isn't present")
	}
	// Prefix collision: hss_db_pool_in_use_connections must not match a
	// query for hss_db_pool_in_use (a prefix of it).
	if _, ok := parseMetric(body, "hss_db_pool_in_use", nil); ok {
		t.Error("parseMetric matched on a bare name prefix instead of the full metric name")
	}
}
