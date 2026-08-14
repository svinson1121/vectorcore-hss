package testcases

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

// TS 35.207 Test Set 1 Ki/OPc — the same fixed test vectors used elsewhere
// in this repo's test suites (see internal/gsup/gsup_test.go). Real key
// material has no bearing on what the storm test measures (DB/pool
// contention under load), so reusing a well-known test set avoids needing
// real subscriber credentials.
const (
	provisionKi  = "465b5ce8b199b49faa5f0a2ee238a6bc"
	provisionOPc = "cd63cb71954a9f4e48a5994e37a02baf"
	provisionAMF = "8000"
)

// provisioned tracks everything the storm test created via the REST API, so
// it can be torn down afterward regardless of how the run ended.
type provisioned struct {
	apiAddr string
	apnID   int
	apnName string

	mu     sync.Mutex
	aucIDs []int
	subIDs []int
	imsis  []string
}

func newProvisioned(apiAddr string) *provisioned {
	return &provisioned{apiAddr: apiAddr}
}

func httpJSON(ctx context.Context, method, url string, body, out interface{}) (int, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}
	if resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("%s %s: HTTP %d: %s", method, url, resp.StatusCode, string(respBody))
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode response: %w", err)
		}
	}
	return resp.StatusCode, nil
}

// provisionTestData creates one synthetic APN and count synthetic
// subscribers (each with its own AUC, since AUC.IMSI is a unique 1:1 link)
// via the REST API, so the storm test is self-contained and never touches
// hand-provisioned data. Everything is prefixed "loadtest-" so cleanup is
// unambiguous. Creation is parallelized (bounded) since 500 subscribers is
// 1000 sequential POSTs (AUC + subscriber each) at 1-by-1 speed.
func provisionTestData(ctx context.Context, apiAddr string, imsiBase int64, count int, apnName string, log *zap.Logger) (*provisioned, error) {
	p := newProvisioned(apiAddr)
	p.apnName = apnName

	apn := &models.APN{
		APN:                     apnName,
		IPVersion:               0,
		ChargingCharacteristics: "0800",
		APNAMBRDown:             1000000,
		APNAMBRUp:               1000000,
		QCI:                     9,
		ARPPriority:             4,
	}
	var apnOut models.APN
	if _, err := httpJSON(ctx, http.MethodPost, apiAddr+"/api/v1/apn", apn, &apnOut); err != nil {
		return nil, fmt.Errorf("create test APN: %w", err)
	}
	p.apnID = apnOut.APNID
	log.Info("storm: provisioned test APN", zap.Int("apn_id", p.apnID), zap.String("apn", apnName))

	const concurrency = 20
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex

	for i := 0; i < count; i++ {
		imsi := fmt.Sprintf("%015d", imsiBase+int64(i))
		wg.Add(1)
		sem <- struct{}{}
		go func(imsi string) {
			defer wg.Done()
			defer func() { <-sem }()

			aucID, subID, err := provisionOne(ctx, apiAddr, imsi, p.apnID)
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
				return
			}
			p.mu.Lock()
			p.aucIDs = append(p.aucIDs, aucID)
			p.subIDs = append(p.subIDs, subID)
			p.imsis = append(p.imsis, imsi)
			p.mu.Unlock()
		}(imsi)
	}
	wg.Wait()

	if firstErr != nil && len(p.subIDs) == 0 {
		return nil, fmt.Errorf("provisioning failed: %w", firstErr)
	}
	if firstErr != nil {
		log.Warn("storm: some test subscribers failed to provision, continuing with the rest",
			zap.Int("provisioned", len(p.subIDs)), zap.Int("requested", count), zap.Error(firstErr))
	}
	log.Info("storm: provisioned test subscribers", zap.Int("count", len(p.subIDs)))
	return p, nil
}

func provisionOne(ctx context.Context, apiAddr, imsi string, apnID int) (aucID, subID int, err error) {
	auc := &models.AUC{Ki: provisionKi, OPc: provisionOPc, AMF: provisionAMF, IMSI: &imsi}
	var aucOut models.AUC
	if _, err := httpJSON(ctx, http.MethodPost, apiAddr+"/api/v1/subscriber/auc", auc, &aucOut); err != nil {
		return 0, 0, fmt.Errorf("create AUC for %s: %w", imsi, err)
	}

	sub := &models.Subscriber{
		IMSI:                  imsi,
		AUCID:                 aucOut.AUCID,
		DefaultAPN:            apnID,
		APNList:               fmt.Sprintf("%d", apnID),
		UEAMBRDown:            999999,
		UEAMBRUp:              999999,
		SubscribedRAUTAUTimer: 300,
	}
	var subOut models.Subscriber
	if _, err := httpJSON(ctx, http.MethodPost, apiAddr+"/api/v1/subscriber", sub, &subOut); err != nil {
		return 0, 0, fmt.Errorf("create subscriber %s: %w", imsi, err)
	}
	return aucOut.AUCID, subOut.SubscriberID, nil
}

// cleanup deletes everything provisionTestData created, in FK-safe order
// (subscribers, then AUCs, then the APN — matching the REST API's own
// delete-conflict checks). Best-effort: every failure is logged, none abort
// the rest of the cleanup, so a partial failure never leaves the run silently
// unclean.
func (p *provisioned) cleanup(ctx context.Context, log *zap.Logger) {
	if p == nil {
		return
	}
	const concurrency = 20
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	del := func(path string) error {
		_, err := httpJSON(ctx, http.MethodDelete, p.apiAddr+path, nil, nil)
		return err
	}

	for _, id := range p.subIDs {
		wg.Add(1)
		sem <- struct{}{}
		go func(id int) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := del(fmt.Sprintf("/api/v1/subscriber/%d", id)); err != nil {
				log.Warn("storm cleanup: failed to delete subscriber", zap.Int("id", id), zap.Error(err))
			}
		}(id)
	}
	wg.Wait()

	for _, id := range p.aucIDs {
		wg.Add(1)
		sem <- struct{}{}
		go func(id int) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := del(fmt.Sprintf("/api/v1/subscriber/auc/%d", id)); err != nil {
				log.Warn("storm cleanup: failed to delete AUC", zap.Int("id", id), zap.Error(err))
			}
		}(id)
	}
	wg.Wait()

	if p.apnID != 0 {
		if err := del(fmt.Sprintf("/api/v1/apn/%d", p.apnID)); err != nil {
			log.Warn("storm cleanup: failed to delete test APN", zap.Int("apn_id", p.apnID), zap.Error(err))
		}
	}
	log.Info("storm: cleanup complete",
		zap.Int("subscribers_deleted", len(p.subIDs)),
		zap.Int("aucs_deleted", len(p.aucIDs)),
	)
}
