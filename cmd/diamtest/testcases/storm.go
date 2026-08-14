// storm.go simulates a mass eNB/MME reconnect: it opens many separate real
// Diameter TCP connections concurrently (unlike RunLoad, which multiplexes
// many requests over one shared connection) and drives each through an
// AIR+ULR attach, while a concurrent stream of Gx CCR-I requests measures
// whether that burst starves them of database access. It requires a real
// running hss process with a real Postgres behind it — the in-memory
// s6aTestStore fake used by internal/diameter/s6a/*_test.go can't exercise
// real connection-pool contention or lock waits, which is exactly what this
// tool reproduces (or, after the fix, proves no longer happens).
package testcases

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"go.uber.org/zap"
)

// StormConfig controls the reconnect-storm load test.
type StormConfig struct {
	APIAddr    string        // OAM REST API base, e.g. "http://localhost:8080" (for provisioning + /metrics)
	Clients    int           // number of separate Diameter connections to open (200-500 to reproduce the incident)
	RampUp     time.Duration // spread the Clients connections' start times across this window (0 = all at once)
	HoldFor    time.Duration // how long to keep sending CCR-I traffic after all clients have attached
	CCRRate    int           // Gx CCR-I requests per second injected for the duration of the run
	ReqTimeout time.Duration
	IMSIBase   string
	APNName    string
	NoCleanup  bool
}

// RunStorm provisions synthetic test data, fires the connection storm, and
// prints a three-block report (AIR / ULR / CCR-I) plus a pool-saturation
// summary sampled from /metrics during the run.
func RunStorm(cfg *Config, sc *StormConfig) error {
	if sc.APIAddr == "" {
		return fmt.Errorf("--api is required (used for test-data provisioning and /metrics polling)")
	}
	if sc.Clients <= 0 {
		return fmt.Errorf("--clients must be > 0")
	}

	plmn, err := encodePLMN(cfg.MCC, cfg.MNC)
	if err != nil {
		return err
	}
	imsiBase, err := strconv.ParseInt(sc.IMSIBase, 10, 64)
	if err != nil {
		return fmt.Errorf("--imsi-base must be numeric: %w", err)
	}

	// Must happen once, synchronously, before any goroutine dials a
	// connection: it resets the shared dict.Default package variable, which
	// every S6a/Gx connection reads. See ensureGxDict's doc comment (gx.go).
	if err := ensureGxDict(); err != nil {
		return err
	}

	// Ctrl-C during either provisioning or the run still triggers cleanup —
	// self-provisioned data must not be left behind on early exit.
	sigCtx, stopSig := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSig()

	provCtx, cancelProv := context.WithTimeout(sigCtx, 2*time.Minute)
	data, err := provisionTestData(provCtx, sc.APIAddr, imsiBase, sc.Clients, sc.APNName, cfg.Log)
	cancelProv()
	if err != nil {
		return fmt.Errorf("provision test data: %w", err)
	}
	if !sc.NoCleanup {
		defer func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			data.cleanup(cleanupCtx, cfg.Log)
		}()
	} else {
		cfg.Log.Warn("storm: --no-cleanup set, leaving provisioned test data in place",
			zap.Int("apn_id", data.apnID), zap.Int("subscribers", len(data.subIDs)))
	}
	if len(data.imsis) == 0 {
		return fmt.Errorf("no test subscribers were provisioned, nothing to run")
	}

	airRec := newRecorder("AIR")
	ulrRec := newRecorder("ULR")
	ccrRec := newRecorder("CCR-I")

	runCtx, cancelRun := context.WithCancel(sigCtx)
	defer cancelRun()

	sampler := newMetricsSampler(sc.APIAddr)
	go sampler.run(runCtx, time.Second)

	cfg.Log.Info("storm: starting",
		zap.Int("clients", sc.Clients),
		zap.Duration("ramp_up", sc.RampUp),
		zap.Duration("hold_for", sc.HoldFor),
		zap.Int("ccr_rate", sc.CCRRate),
	)

	start := time.Now()

	// ── AIR/ULR connection storm ────────────────────────────────────────────
	var attachWG sync.WaitGroup
	var connected atomic.Int64
	spacing := time.Duration(0)
	if sc.Clients > 1 && sc.RampUp > 0 {
		spacing = sc.RampUp / time.Duration(sc.Clients)
	}
	for i, imsi := range data.imsis {
		attachWG.Add(1)
		go func(i int, imsi string) {
			defer attachWG.Done()
			if spacing > 0 {
				time.Sleep(spacing * time.Duration(i))
			}
			runAttach(cfg, imsi, plmn, sc.ReqTimeout, airRec, ulrRec)
			connected.Add(1)
		}(i, imsi)
	}

	// ── concurrent CCR-I stream ──────────────────────────────────────────────
	// Starts immediately (not after the AIR/ULR storm) since the whole point
	// is to observe CCR-I behavior *during* the burst, matching the incident
	// report: "a CCR-I arrives in that time [and] timeouts".
	var ccrWG sync.WaitGroup
	if sc.CCRRate > 0 {
		ccrCtx, cancelCCR := context.WithCancel(runCtx)
		ccrWG.Add(1)
		go func() {
			defer ccrWG.Done()
			runCCRStream(ccrCtx, cfg, data, sc, ccrRec)
		}()
		// Stop CCR-I generation once clients have attached and HoldFor elapses.
		go func() {
			attachWG.Wait()
			select {
			case <-time.After(sc.HoldFor):
			case <-runCtx.Done():
			}
			cancelCCR()
		}()
	}

	attachWG.Wait()
	elapsed := time.Since(start)
	cfg.Log.Info("storm: all clients attached", zap.Int64("connected", connected.Load()), zap.Duration("elapsed", elapsed))

	if sc.CCRRate > 0 {
		ccrWG.Wait()
	}
	cancelRun()

	fmt.Printf("\n=== Storm Test Results ===\n")
	fmt.Printf("Clients:      %d\n", sc.Clients)
	fmt.Printf("Ramp-up:      %s\n", sc.RampUp)
	fmt.Printf("Hold-for:     %s\n", sc.HoldFor)
	fmt.Printf("CCR-I rate:   %d/s\n", sc.CCRRate)
	fmt.Printf("Total time:   %s\n", time.Since(start).Round(time.Millisecond))

	airRec.report()
	ulrRec.report()
	ccrRec.report()

	sampler.report()

	return nil
}

// runAttach dials one connection and drives a single AIR+ULR attach over it,
// mirroring how one MME/SGSN peer behaves on reconnect. The connection is
// closed once the attach completes; a real MME would hold it open, but for
// measuring pool contention during the burst window, the DB work happens up
// front regardless.
func runAttach(cfg *Config, imsi string, plmn []byte, timeout time.Duration, airRec, ulrRec *recorder) {
	c, err := connect(cfg)
	if err != nil {
		airRec.fail(false)
		ulrRec.fail(false)
		return
	}
	defer c.close()

	airCh := make(chan *diam.Message, 1)
	c.mux.HandleFunc(diam.AIA, func(_ diam.Conn, msg *diam.Message) { airCh <- msg })
	airStart := time.Now()
	if err := c.send(buildAIR(cfg, imsi, plmn, 1)); err != nil {
		airRec.fail(false)
	} else {
		select {
		case ans := <-airCh:
			if rc, ok := getResultCode(ans); ok && rc == ResultSuccess {
				airRec.ok(time.Since(airStart))
			} else {
				airRec.fail(false)
			}
		case <-time.After(timeout):
			airRec.fail(true)
		}
	}

	ulaCh := make(chan *diam.Message, 1)
	c.mux.HandleFunc(diam.ULA, func(_ diam.Conn, msg *diam.Message) { ulaCh <- msg })
	ulrStart := time.Now()
	if err := c.send(buildULR(cfg, imsi, plmn, 1004)); err != nil {
		ulrRec.fail(false)
		return
	}
	select {
	case ans := <-ulaCh:
		if rc, ok := getResultCode(ans); ok && rc == ResultSuccess {
			ulrRec.ok(time.Since(ulrStart))
		} else {
			ulrRec.fail(false)
		}
	case <-time.After(timeout):
		ulrRec.fail(true)
	}
}

// runCCRStream sends one CCR-I every interval, cycling through the
// provisioned IMSI pool, until ctx is cancelled.
func runCCRStream(ctx context.Context, cfg *Config, data *provisioned, sc *StormConfig, rec *recorder) {
	interval := time.Second / time.Duration(sc.CCRRate)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var i int
	var ipSeq atomic.Uint32
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			imsi := data.imsis[i%len(data.imsis)]
			i++
			n := ipSeq.Add(1)
			ueIP := net.IPv4(10, 200, byte(n>>8), byte(n))
			go func(imsi string, ueIP net.IP) {
				rc, lat, err := sendCCRI(cfg, imsi, data.apnName, ueIP, sc.ReqTimeout)
				if err != nil {
					isTimeout := err == errLoadTimeout
					if !isTimeout {
						cfg.Log.Debug("storm: CCR-I failed", zap.String("imsi", imsi), zap.Error(err))
					}
					rec.fail(isTimeout)
					return
				}
				if rc == ResultSuccess {
					rec.ok(lat)
				} else {
					rec.fail(false)
				}
			}(imsi, ueIP)
		}
	}
}

// ── /metrics sampling ────────────────────────────────────────────────────────

// metricsSampler periodically scrapes the HSS's /metrics endpoint during the
// storm and tracks peak DB-pool and admission-control saturation, which is
// the direct evidence connecting an S6a burst to CCR-I contention (or, once
// the fix is in place, evidence that admission control kept the pool from
// saturating).
type metricsSampler struct {
	apiAddr string

	mu               sync.Mutex
	maxInUse         float64
	maxWaitCount     float64
	maxS6aInFlight   float64
	maxGxInFlight    float64
	s6aRejectedDelta float64
	gxRejectedDelta  float64
	haveSample       bool
	firstS6aRejected float64
	firstGxRejected  float64
}

func newMetricsSampler(apiAddr string) *metricsSampler {
	return &metricsSampler{apiAddr: apiAddr}
}

func (m *metricsSampler) run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.sampleOnce()
		}
	}
}

func (m *metricsSampler) sampleOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	body, err := fetchMetrics(ctx, m.apiAddr)
	if err != nil {
		return
	}

	inUse, _ := parseMetric(body, "hss_db_pool_in_use_connections", nil)
	waitCount, _ := parseMetric(body, "hss_db_pool_wait_count_total", nil)
	s6aInFlight, _ := parseMetric(body, "hss_admission_in_flight", map[string]string{"app": "s6a"})
	gxInFlight, _ := parseMetric(body, "hss_admission_in_flight", map[string]string{"app": "gx"})
	s6aRejected, _ := parseMetric(body, "hss_admission_rejected_total", map[string]string{"app": "s6a"})
	gxRejected, _ := parseMetric(body, "hss_admission_rejected_total", map[string]string{"app": "gx"})

	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.haveSample {
		m.firstS6aRejected = s6aRejected
		m.firstGxRejected = gxRejected
		m.haveSample = true
	}
	if inUse > m.maxInUse {
		m.maxInUse = inUse
	}
	if waitCount > m.maxWaitCount {
		m.maxWaitCount = waitCount
	}
	if s6aInFlight > m.maxS6aInFlight {
		m.maxS6aInFlight = s6aInFlight
	}
	if gxInFlight > m.maxGxInFlight {
		m.maxGxInFlight = gxInFlight
	}
	m.s6aRejectedDelta = s6aRejected - m.firstS6aRejected
	m.gxRejectedDelta = gxRejected - m.firstGxRejected
}

func (m *metricsSampler) report() {
	m.mu.Lock()
	defer m.mu.Unlock()
	fmt.Printf("\n--- DB pool / admission (sampled from /metrics) ---\n")
	if !m.haveSample {
		fmt.Printf("  (no samples collected — is %s/metrics reachable?)\n", m.apiAddr)
		return
	}
	fmt.Printf("  Peak DB pool in-use connections: %.0f\n", m.maxInUse)
	fmt.Printf("  DB pool wait_count (cumulative): %.0f\n", m.maxWaitCount)
	fmt.Printf("  Peak S6a admission in-flight:    %.0f\n", m.maxS6aInFlight)
	fmt.Printf("  Peak Gx admission in-flight:     %.0f\n", m.maxGxInFlight)
	fmt.Printf("  S6a admission rejections during run: %.0f\n", m.s6aRejectedDelta)
	fmt.Printf("  Gx admission rejections during run:  %.0f\n", m.gxRejectedDelta)
}

// fetchMetrics GETs the HSS's /metrics endpoint as plain text (Prometheus
// exposition format, not JSON, so this doesn't reuse httpJSON from
// provision.go).
func fetchMetrics(ctx context.Context, apiAddr string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiAddr+"/metrics", nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// parseMetric extracts the value of a Prometheus text-format metric line
// matching name and (if given) every label in labels. It's intentionally
// minimal — just enough to read the specific gauges/counters this tool
// needs from a client_golang-formatted /metrics response — not a general
// Prometheus exposition parser.
func parseMetric(body, name string, labels map[string]string) (float64, bool) {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, name) {
			continue
		}
		rest := line[len(name):]
		if len(rest) > 0 && rest[0] != ' ' && rest[0] != '{' {
			continue // name is a prefix of a different metric
		}
		labelPart := ""
		valuePart := rest
		if idx := strings.Index(rest, "{"); idx == 0 {
			end := strings.Index(rest, "}")
			if end < 0 {
				continue
			}
			labelPart = rest[1:end]
			valuePart = rest[end+1:]
		}
		if labels != nil {
			ok := true
			for k, v := range labels {
				if !strings.Contains(labelPart, fmt.Sprintf(`%s="%s"`, k, v)) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
		}
		valuePart = strings.TrimSpace(valuePart)
		f, err := strconv.ParseFloat(valuePart, 64)
		if err != nil {
			continue
		}
		return f, true
	}
	return 0, false
}
