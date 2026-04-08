package testcases

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
	"github.com/fiorix/go-diameter/v4/diam/sm"
	"go.uber.org/zap"
)

// LoadConfig controls the load test parameters.
type LoadConfig struct {
	Workers    int
	Duration   time.Duration
	Count      int64  // if >0, stop after this many requests (overrides Duration)
	IMSIBase   string // 15-digit numeric base IMSI, e.g. "001010000000001"
	IMSICount  int64  // IMSI pool size; workers cycle through base..base+IMSICount-1
	Command    string // "air", "ulr", or "both" (AIR then ULR per iteration)
	ReqTimeout time.Duration
}

// loadConn is a single persistent Diameter connection shared across workers.
// Responses are demultiplexed by session ID so many requests can be in-flight
// concurrently over the same TCP connection.
type loadConn struct {
	cfg    *Config
	conn   diam.Conn
	mux    *sm.StateMachine
	sidSeq atomic.Int64
	mu     sync.Mutex
	pending map[string]chan *diam.Message
}

var errLoadTimeout = fmt.Errorf("timeout")

func newLoadConn(cfg *Config) (*loadConn, error) {
	lc := &loadConn{
		cfg:     cfg,
		pending: make(map[string]chan *diam.Message),
	}

	settings := &sm.Settings{
		OriginHost:       datatype.DiameterIdentity(cfg.OriginHost),
		OriginRealm:      datatype.DiameterIdentity(cfg.OriginRealm),
		VendorID:         datatype.Unsigned32(vendor3GPP),
		ProductName:      datatype.UTF8String("diamtest-load"),
		OriginStateID:    datatype.Unsigned32(uint32(time.Now().Unix())),
		FirmwareRevision: 1,
	}

	mux := sm.New(settings)

	// Route all Diameter answers to the per-session-ID waiting channel.
	route := func(_ diam.Conn, msg *diam.Message) {
		sidAVP, err := msg.FindAVP(avp.SessionID, 0)
		if err != nil {
			return
		}
		sid, ok := sidAVP.Data.(datatype.UTF8String)
		if !ok {
			return
		}
		lc.mu.Lock()
		ch, found := lc.pending[string(sid)]
		lc.mu.Unlock()
		if found {
			ch <- msg // buffered(1) — never blocks
		}
	}
	mux.HandleFunc(diam.AIA, route)
	mux.HandleFunc(diam.ULA, route)

	smClient := &sm.Client{
		Dict:               dict.Default,
		Handler:            mux,
		MaxRetransmits:     0,
		RetransmitInterval: time.Second,
		EnableWatchdog:     true,
		WatchdogInterval:   10 * time.Second,
		SupportedVendorID: []*diam.AVP{
			diam.NewAVP(avp.SupportedVendorID, avp.Mbit, 0, datatype.Unsigned32(vendor3GPP)),
		},
		VendorSpecificApplicationID: []*diam.AVP{
			diam.NewAVP(avp.VendorSpecificApplicationID, avp.Mbit, 0, &diam.GroupedAVP{
				AVP: []*diam.AVP{
					diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(appIDS6a)),
					diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(vendor3GPP)),
				},
			}),
		},
	}

	conn, err := smClient.DialNetwork("tcp", cfg.HSSAddr)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", cfg.HSSAddr, err)
	}
	lc.conn = conn
	lc.mux = mux
	return lc, nil
}

func (lc *loadConn) close() { lc.conn.Close() }

func (lc *loadConn) nextSID() string {
	n := lc.sidSeq.Add(1)
	return fmt.Sprintf("%s;load;%d", lc.cfg.OriginHost, n)
}

// roundTrip sends msg and waits for the answer, keyed by sid.
func (lc *loadConn) roundTrip(msg *diam.Message, sid string, timeout time.Duration) (*diam.Message, time.Duration, error) {
	ch := make(chan *diam.Message, 1)

	lc.mu.Lock()
	lc.pending[sid] = ch
	lc.mu.Unlock()

	start := time.Now()
	if _, err := msg.WriteTo(lc.conn); err != nil {
		lc.mu.Lock()
		delete(lc.pending, sid)
		lc.mu.Unlock()
		return nil, 0, err
	}

	select {
	case ans := <-ch:
		lat := time.Since(start)
		lc.mu.Lock()
		delete(lc.pending, sid)
		lc.mu.Unlock()
		return ans, lat, nil
	case <-time.After(timeout):
		lc.mu.Lock()
		delete(lc.pending, sid)
		lc.mu.Unlock()
		return nil, 0, errLoadTimeout
	}
}

func (lc *loadConn) doAIR(imsi string, plmn []byte, timeout time.Duration) (uint32, time.Duration, error) {
	sid := lc.nextSID()
	req := buildAIRLoad(lc.cfg, sid, imsi, plmn)
	ans, lat, err := lc.roundTrip(req, sid, timeout)
	if err != nil {
		return 0, 0, err
	}
	rc, _ := getResultCode(ans)
	return rc, lat, nil
}

func (lc *loadConn) doULR(imsi string, plmn []byte, timeout time.Duration) (uint32, time.Duration, error) {
	sid := lc.nextSID()
	req := buildULRLoad(lc.cfg, sid, imsi, plmn)
	ans, lat, err := lc.roundTrip(req, sid, timeout)
	if err != nil {
		return 0, 0, err
	}
	rc, _ := getResultCode(ans)
	return rc, lat, nil
}

// buildAIRLoad builds an AIR using a caller-supplied session ID.
func buildAIRLoad(cfg *Config, sid, imsi string, plmn []byte) *diam.Message {
	req := diam.NewRequest(diam.AuthenticationInformation, appIDS6a, nil)
	req.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
	req.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(cfg.OriginHost))
	req.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(cfg.OriginRealm))
	req.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity("epc.test.net"))
	req.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))
	req.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	req.NewAVP(avp.VisitedPLMNID, avp.Mbit|avp.Vbit, vendor3GPP, datatype.OctetString(plmn))
	req.NewAVP(avp.RequestedEUTRANAuthenticationInfo, avp.Mbit|avp.Vbit, vendor3GPP, &diam.GroupedAVP{
		AVP: []*diam.AVP{
			diam.NewAVP(avp.NumberOfRequestedVectors, avp.Mbit|avp.Vbit, vendor3GPP, datatype.Unsigned32(1)),
			diam.NewAVP(avp.ImmediateResponsePreferred, avp.Mbit|avp.Vbit, vendor3GPP, datatype.Unsigned32(0)),
		},
	})
	req.NewAVP(avp.VendorSpecificApplicationID, avp.Mbit, 0, &diam.GroupedAVP{
		AVP: []*diam.AVP{
			diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(vendor3GPP)),
			diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(appIDS6a)),
		},
	})
	return req
}

// buildULRLoad builds a ULR using a caller-supplied session ID.
func buildULRLoad(cfg *Config, sid, imsi string, plmn []byte) *diam.Message {
	req := diam.NewRequest(diam.UpdateLocation, appIDS6a, nil)
	req.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sid))
	req.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(cfg.OriginHost))
	req.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(cfg.OriginRealm))
	req.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity("epc.test.net"))
	req.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String(imsi))
	req.NewAVP(avp.AuthSessionState, avp.Mbit, 0, datatype.Enumerated(1))
	req.NewAVP(avp.RATType, avp.Mbit|avp.Vbit, vendor3GPP, datatype.Unsigned32(1004))
	req.NewAVP(avp.ULRFlags, avp.Mbit|avp.Vbit, vendor3GPP, datatype.Unsigned32(34))
	req.NewAVP(avp.VisitedPLMNID, avp.Mbit|avp.Vbit, vendor3GPP, datatype.OctetString(plmn))
	req.NewAVP(avp.VendorSpecificApplicationID, avp.Mbit, 0, &diam.GroupedAVP{
		AVP: []*diam.AVP{
			diam.NewAVP(avp.VendorID, avp.Mbit, 0, datatype.Unsigned32(vendor3GPP)),
			diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(appIDS6a)),
		},
	})
	return req
}

// RunLoad runs the Diameter load test and prints a final report.
func RunLoad(cfg *Config, lc *LoadConfig) error {
	plmn, err := encodePLMN(cfg.MCC, cfg.MNC)
	if err != nil {
		return err
	}

	baseIMSI, err := strconv.ParseInt(lc.IMSIBase, 10, 64)
	if err != nil {
		return fmt.Errorf("--imsi-base must be numeric: %w", err)
	}
	if lc.IMSICount <= 0 {
		lc.IMSICount = 1
	}

	conn, err := newLoadConn(cfg)
	if err != nil {
		return err
	}
	defer conn.close()

	cfg.Log.Info("load test starting",
		zap.String("command", lc.Command),
		zap.Int("workers", lc.Workers),
		zap.String("imsi_base", lc.IMSIBase),
		zap.Int64("imsi_pool", lc.IMSICount),
	)

	// ── stats ─────────────────────────────────────────────────────────────────

	var (
		nSent    atomic.Int64
		nSuccess atomic.Int64
		nError   atomic.Int64
		nTimeout atomic.Int64
	)
	var (
		latMu     sync.Mutex
		latencies []int64 // nanoseconds of successful requests
	)

	recordOK := func(lat time.Duration) {
		nSent.Add(1)
		nSuccess.Add(1)
		latMu.Lock()
		latencies = append(latencies, lat.Nanoseconds())
		latMu.Unlock()
	}
	recordFail := func(isTimeout bool) {
		nSent.Add(1)
		nError.Add(1)
		if isTimeout {
			nTimeout.Add(1)
		}
	}

	// ── stop condition ────────────────────────────────────────────────────────

	var (
		ctx    context.Context
		cancel context.CancelFunc
		reqSeq atomic.Int64
	)
	if lc.Count > 0 {
		ctx, cancel = context.WithCancel(context.Background())
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), lc.Duration)
	}
	defer cancel()

	var imsiSeq atomic.Int64

	// ── workers ───────────────────────────────────────────────────────────────

	var wg sync.WaitGroup
	for i := 0; i < lc.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if lc.Count > 0 && reqSeq.Add(1) > lc.Count {
					cancel()
					return
				}

				idx := imsiSeq.Add(1) % lc.IMSICount
				imsi := fmt.Sprintf("%015d", baseIMSI+idx)

				switch lc.Command {
				case "air":
					rc, lat, err := conn.doAIR(imsi, plmn, lc.ReqTimeout)
					if err != nil {
						recordFail(err == errLoadTimeout)
					} else if rc == ResultSuccess {
						recordOK(lat)
					} else {
						recordFail(false)
					}

				case "ulr":
					rc, lat, err := conn.doULR(imsi, plmn, lc.ReqTimeout)
					if err != nil {
						recordFail(err == errLoadTimeout)
					} else if rc == ResultSuccess {
						recordOK(lat)
					} else {
						recordFail(false)
					}

				case "both":
					rc, lat, err := conn.doAIR(imsi, plmn, lc.ReqTimeout)
					if err != nil {
						recordFail(err == errLoadTimeout)
						continue
					}
					if rc == ResultSuccess {
						recordOK(lat)
					} else {
						recordFail(false)
						continue
					}
					rc, lat, err = conn.doULR(imsi, plmn, lc.ReqTimeout)
					if err != nil {
						recordFail(err == errLoadTimeout)
					} else if rc == ResultSuccess {
						recordOK(lat)
					} else {
						recordFail(false)
					}
				}
			}
		}()
	}

	// ── progress ──────────────────────────────────────────────────────────────

	start := time.Now()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		prev, prevT := int64(0), start
		for {
			select {
			case t := <-ticker.C:
				sent := nSent.Load()
				rps := float64(sent-prev) / t.Sub(prevT).Seconds()
				prev, prevT = sent, t
				fmt.Printf("\r[%5.0fs] sent=%-8d ok=%-8d err=%-8d to=%-6d rps=%-8.0f",
					time.Since(start).Seconds(),
					sent, nSuccess.Load(), nError.Load(), nTimeout.Load(), rps,
				)
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
	elapsed := time.Since(start)
	fmt.Println()

	// ── report ────────────────────────────────────────────────────────────────

	sent := nSent.Load()
	ok := nSuccess.Load()
	errs := nError.Load() - nTimeout.Load()
	tos := nTimeout.Load()

	fmt.Printf("\n=== Load Test Results ===\n")
	fmt.Printf("Command:     %s\n", lc.Command)
	fmt.Printf("Workers:     %d\n", lc.Workers)
	fmt.Printf("Duration:    %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Sent:        %d\n", sent)
	fmt.Printf("Success:     %d  (%.1f%%)\n", ok, pct(ok, sent))
	fmt.Printf("Errors:      %d  (%.1f%%)\n", errs, pct(errs, sent))
	fmt.Printf("Timeouts:    %d  (%.1f%%)\n", tos, pct(tos, sent))
	fmt.Printf("Avg RPS:     %.1f\n", float64(sent)/elapsed.Seconds())

	latMu.Lock()
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		n := len(latencies)
		fmt.Printf("\nLatency (successful requests):\n")
		fmt.Printf("  p50:  %s\n", time.Duration(latencies[n*50/100]))
		fmt.Printf("  p95:  %s\n", time.Duration(latencies[n*95/100]))
		fmt.Printf("  p99:  %s\n", time.Duration(latencies[n*99/100]))
		fmt.Printf("  max:  %s\n", time.Duration(latencies[n-1]))
	}
	latMu.Unlock()

	return nil
}

func pct(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}
