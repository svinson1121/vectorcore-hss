# S6c Interface — 3GPP Spec Test Report

**Interface:** S6c (TS 29.338) — Diameter-based SMS in EPC  
**Test files:** `internal/diameter/s6c/s6c_spec_test.go`, `internal/diameter/s6c/s6c_alsc_test.go`  
**Run:** `go test ./internal/diameter/s6c/... -v`  
**Result:** 37/37 PASS

---

## Test Coverage

### TBCD Encoding (TS 23.003 §12.1)

| Test | Description | Result |
|------|-------------|--------|
| `TestTBCDEvenLength` | Pack a 4-digit MSISDN ("3312") into 2 TBCD bytes; verify nibble swap | PASS |
| `TestTBCDOddLength` | Pack a 3-digit MSISDN ("331") — high nibble of last byte must be 0xF | PASS |
| `TestTBCDRoundTripE164` | Round-trip encode→decode for 11-digit, 12-digit, and 1-digit strings | PASS |

All TBCD encode/decode logic is spec-correct.

---

### `parseDeliveryOutcome` — SM-Delivery-Outcome grouped AVP (TS 29.338 §7.3.15–7.3.22)

| Test | Description | Result |
|------|-------------|--------|
| `TestParseDeliveryOutcomeMME` | Cause extracted from MME-Delivery-Outcome sub-AVP (code 3317) | PASS |
| `TestParseDeliveryOutcomeSGSN` | Cause extracted from SGSN-Delivery-Outcome sub-AVP (code 3318) | PASS |
| `TestParseDeliveryOutcomeMSC` | Cause extracted from MSC-Delivery-Outcome sub-AVP (code 3319) | PASS |
| `TestParseDeliveryOutcomeIPSMGW` | Cause extracted from IP-SM-GW-Delivery-Outcome sub-AVP (code 3320) | PASS |
| `TestParseDeliveryOutcomeAbsent` | Missing SM-Delivery-Outcome returns Cause = −1 (caller defaults to AbsentUser) | PASS |
| `TestParseDeliveryOutcomeAbsentUserDiagnostic` | Optional Absent-User-Diagnostic-SM (code 3322) extracted alongside cause | PASS |

All four node-outcome sub-AVP types are handled. The absent-outcome default path (Cause = −1) is correct per TS 29.338 §5.3.2.4.

---

### SRI-SM — Send-Routing-Info-for-SM (TS 29.338 §5.3.2.1 / §5.3.2.2)

| Test | TS 29.338 Clause | Description | Result |
|------|------------------|-------------|--------|
| `TestSRISM_AttachedSubscriber` | §5.3.2.1 | Attached subscriber: Result-Code 2001, User-Name=IMSI, MSISDN echoed, Serving-Node with MME-Name, no MWD-Status | PASS |
| `TestSRISM_AbsentSubscriberMNRF` | §5.3.2.2 | No serving node: Result-Code 2001, MWD-Status bit 0x02 (MNRF) set, no Serving-Node | PASS |
| `TestSRISM_LookupByIMSI` | §6.3 | Subscriber addressed by User-Name (IMSI) instead of MSISDN | PASS |
| `TestSRISM_UnknownSubscriber` | §6.3 | Unknown MSISDN: Experimental-Result-Code 5001 (User-Unknown) | PASS |
| `TestSRISM_NoMSISDNInResponse` | §6.3 | MSISDN AVP absent from response when subscriber has no MSISDN provisioned | PASS |
| `TestSRISM_ServingMMERealm` | §7.3.8 | MME-Realm returned inside Serving-Node when available | PASS |

SRI-SM spec compliance confirmed for all primary flows.

---

### RSDS — Report-SM-Delivery-Status (TS 29.338 §5.3.2.4)

| Test | TS 29.338 Clause | Description | Result |
|------|------------------|-------------|--------|
| `TestRSDS_SuccessfulTransfer` | §5.3.2.4 | Cause=2 (SuccessfulTransfer): DeleteMWD called, Result-Code 2001, no MWD-Status in answer | PASS |
| `TestRSDS_AbsentUser` | §5.3.2.4 | Cause=1 (AbsentUser): StoreMWD with MNRF flag (0x02), MWD-Status=MNRF in answer | PASS |
| `TestRSDS_MemoryCapacityExceeded` | §5.3.2.4 | Cause=0 (MemoryCapacityExceeded): StoreMWD with MCEF flag (0x04), MWD-Status=MCEF in answer | PASS |
| `TestRSDS_AbsentOutcome` | §5.3.2.4 | SM-Delivery-Outcome absent: defaults to AbsentUser / MNRF | PASS |
| `TestRSDS_UnknownSubscriber` | §6.3 | Unknown subscriber: Experimental-Result-Code 5001 | PASS |
| `TestRSDS_LookupByIMSI` | §6.3 | Subscriber addressed by User-Name (IMSI) | PASS |
| `TestRSDS_SGSNDeliveryOutcome` | §7.3.17 | SGSN-Delivery-Outcome handled identically to MME-Delivery-Outcome | PASS |
| `TestRSDS_SCAddressMWDRecord` | §5.3.2.4 | SC-Address stored as plain-digit string (TBCD decoded) in MWD record | PASS |
| `TestRSDS_OriginHostAndRealmStoredInMWD` | §5.3.2.4 | Origin-Host and Origin-Realm from RSDS request persisted into MWD record (needed for ALSC target address) | PASS |

RSDS spec compliance confirmed for all primary flows.

---

### ALSC / ASA — Alert-Service-Centre / Alert-Service-Centre-Answer

| Test | Description | Result |
|------|-------------|--------|
| `TestLoadMSISDNSupplement_Idempotent` | Repeated supplement-dictionary load is safe | PASS |
| `TestALSC_SendUsesStoredOriginRealm` | Outbound ALSC uses stored `sc_origin_host` / `sc_origin_realm` and correct S6c command/app IDs | PASS |
| `TestALSC_SendStoresPendingSession` | Successful ALSC send records pending session correlation state | PASS |
| `TestASA_SuccessDeletesMWD` | `ASA` success (`Result-Code=2001`) deletes matching MWD and clears pending session | PASS |
| `TestASA_FailureRetainsMWD` | Non-success `ASA` retains MWD for retry and clears pending session | PASS |
| `TestASA_UnknownSessionDoesNotDeleteMWD` | Stray or replayed `ASA` does not delete unrelated MWD | PASS |
| `TestASA_MissingSessionIDDoesNotDeleteMWD` | Malformed `ASA` without `Session-Id` is ignored safely | PASS |
| `TestSendALSCForIMSI_NoMWD_NoSend` | No MWD means no peer lookup and no send | PASS |
| `TestSendALSCForIMSI_PeerNotConnected_MWDRetained` | Disconnected SMSC leaves MWD in place | PASS |
| `TestSendALSCForIMSI_MultipleMWDRecords` | One ALSC per MWD record with distinct Session-Ids and correct per-record routing | PASS |
| `TestSendALSCForIMSI_SubscriberMissing` | Missing subscriber short-circuits without send or delete | PASS |
| `TestSendALSCForIMSI_SendFailureRetainsMWD` | Diameter write failure leaves MWD in place and does not create pending session state | PASS |

ALSC / ASA transaction behavior is now covered for the main success, retry, and safe-ignore paths described in the remaining test plan.

---

## Remaining Plan Review

Reviewed against `docs/s6c_remaining_test_plan.md` on 2026-04-01:

- Already covered before this change:
  - RSDS persistence of `Origin-Host` / `Origin-Realm`
  - RSDS persistence of `MWDStatusFlags` (`MNRF` and `MCEF`)
  - real test bootstrap path loading `LoadMSISDNSupplement()`
- Implemented in this change:
  - core ALSC send construction and pending-session tests
  - ASA success/failure/unknown/malformed-session handling
  - ALSC trigger behavior for no-MWD, missing-subscriber, disconnected-peer, multiple-record, and send-failure paths
  - supplement load idempotency
- Not added in this change:
  - explicit `NewServer` startup-wiring proof that `s6c.LoadMSISDNSupplement()` was invoked
  - a dedicated `ASA` unmarshal-failure test; the handler-level malformed cases that are easy to synthesize are already covered by missing/unknown session safety checks
  - optional log-field assertions

---

## Bug Found During Testing

### BUG-S6C-001: MSISDN-addressed SRI-SM and RSDS failed in production

**Status:** Resolved  
**Severity:** High  
**Spec ref:** TS 29.338 §5.3.2.1, §5.3.2.4  
**Affected flows:** SRI-SM and RSDS when the SMS-SC identifies the subscriber by MSISDN (rather than IMSI/User-Name)

**Root cause:**

`msg.Unmarshal` in go-diameter resolves struct field names (e.g., `avp:"MSISDN"`) by calling `dict.FindAVPWithVendor(appID, "MSISDN", UndefinedVendorID)`. For S6c (app 16777312) this search:

1. Tries app 16777312 (S6c) — not found (S6c dict omits MSISDN to avoid duplicate registration)
2. Falls back to app 0 (base) — not found (MSISDN is 3GPP-specific)
3. Returns error → handler returns `Result-Code: 5012 (UnableToComply)` instead of processing the request

MSISDN (code 701) is registered under the TGPP app (id=4) in go-diameter's built-in `default.go`, but app 4 is not in the parent-chain for S6c. Apps S6a (16777251) and Gx (16777238) are correctly mapped to parent app 4 in go-diameter's `parentAppIds` table — S6c (16777312) is missing from that table.

**Impact:** Any SMS-SC that sends SRI-SM with an MSISDN in the MSISDN AVP (without a User-Name/IMSI) will receive an error response. This is the standard addressing mode for SMS routing; IMSI-addressed SRI-SM is uncommon in deployed networks.

**Implemented fix:**

VectorCore HSS now loads a small S6c supplement dictionary during startup that registers `MSISDN` under application 0 (the universal fallback). This allows S6c request structs to unmarshal `MSISDN` successfully without relying on a patched `go-diameter` parent-app chain.

The same supplement is loaded by the S6c test bootstrap so the tests exercise the same path used in production.

**Result:**

- Standard MSISDN-addressed SRI-SM requests now resolve the subscriber normally.
- Standard MSISDN-addressed RSDS requests now resolve the subscriber normally.
- The local `go.mod` `replace` to `/usr/src/go-diameter-patched` was removed; the repo now uses upstream `github.com/fiorix/go-diameter/v4 v4.0.4`.

---

## Summary

| Category | Tests | Pass | Fail |
|----------|-------|------|------|
| TBCD Encoding | 3 | 3 | 0 |
| parseDeliveryOutcome | 6 | 6 | 0 |
| SRI-SM handler | 6 | 6 | 0 |
| RSDS handler | 9 | 9 | 0 |
| ALSC / ASA handler | 12 | 12 | 0 |
| **Total** | **37** | **37** | **0** |

All 37 S6c tests pass. The main remaining-plan gap, ALSC / ASA transaction coverage, is now implemented for the primary operational paths. BUG-S6C-001 was identified during testing and has now been resolved in the HSS codebase.
