# Sh Interface - 3GPP Compliance Test Report

**Interface:** Sh (3GPP TS 29.328 / TS 29.329)  
**Test file:** `internal/diameter/sh/sh_spec_test.go`  
**Run:** `go test -v ./internal/diameter/sh`  
**Executed:** `2026-04-01T23:26:18Z`  
**Result:** 3/3 PASS

---

## Test Run Output

```text
=== RUN   TestUDR_MSISDNWithAddressHeaderReturnsSubscriberData
--- PASS: TestUDR_MSISDNWithAddressHeaderReturnsSubscriberData (0.00s)
=== RUN   TestUDR_UnsupportedDataReferenceReturns5009
--- PASS: TestUDR_UnsupportedDataReferenceReturns5009 (0.00s)
=== RUN   TestSendPNRStoresPendingSessionUntilPNA
--- PASS: TestSendPNRStoresPendingSessionUntilPNA (0.00s)
PASS
ok  	github.com/svinson1121/vectorcore-hss/internal/diameter/sh	0.035s
```

---

## Coverage

### UDR - User-Data-Request

| Test | Description | Result |
|------|-------------|--------|
| `TestUDR_MSISDNWithAddressHeaderReturnsSubscriberData` | Verifies MSISDN-based lookup succeeds when the `MSISDN` sub-AVP carries address header bytes before the TBCD digits | PASS |
| `TestUDR_UnsupportedDataReferenceReturns5009` | Verifies unsupported `Data-Reference` values return `Experimental-Result-Code = 5009 (Not-Supported-User-Data)` instead of a success profile | PASS |

These tests directly cover the two handler behaviors called out in the review findings:

- MSISDN decoding interoperability for peers that send address header bytes rather than bare TBCD digits
- standards-facing error handling for unsupported `Data-Reference` values

### PNR / PNA - Push-Notification

| Test | Description | Result |
|------|-------------|--------|
| `TestSendPNRStoresPendingSessionUntilPNA` | Verifies outbound `PNR` records pending correlation state and that `PNA` clears it on success | PASS |

This covers the previously missing answer-correlation path for Sh push notifications.

---

## Compliance Status

The current Sh automation demonstrates that the HSS now behaves correctly for the specific issues raised in the review:

- MSISDN-addressed Sh UDR lookup is resilient to common AVP encodings used by peers
- unsupported `Data-Reference` values are rejected with `5009`
- outbound `PNR` transactions are no longer send-only; `PNA` correlation is exercised

---

## Remaining Gaps

The Sh package still does not have broad spec-conformance coverage. The following areas remain untested by automation:

- success-path UDR coverage for each supported `Data-Reference` value
- `RepositoryData` behavior when a stored `ShProfile` blob is present
- explicit unknown-user and missing-identity negative tests
- `PNA` failure-path handling for non-success `Result-Code` and `Experimental-Result`
- server-level startup and handler wiring beyond package-level tests

---

## Summary

| Category | Tests | Pass | Fail |
|----------|-------|------|------|
| UDR | 2 | 2 | 0 |
| PNR / PNA | 1 | 1 | 0 |
| **Total** | **3** | **3** | **0** |

The Sh compliance tests requested for the reviewed findings were executed successfully and the report is now saved in `docs/sh_spec_test_report.md`.
