# HSS S6c Remaining Test Plan

Date: 2026-04-01
Project: `/usr/src/vectorcore-hss`
Scope: additional HSS-side tests still needed after the current `s6c_spec_test.go` coverage

## Purpose

The current HSS S6c spec tests give strong coverage for:

- TBCD encode/decode
- `parseDeliveryOutcome`
- `SRI-SM`
- `RSDS`
- the MSISDN supplement-dictionary startup path

The main remaining coverage gap is the HSS-initiated `ALSC` / SMSC-returned `ASA` flow, plus a few negative-path and integration checks around startup and persistence.

## Priority 1: ALSC / ASA Transaction Tests

These are the most important missing tests.

### 1. `TestALSC_SendUsesStoredOriginRealm`

Goal:
- verify HSS uses `message_waiting_data.sc_origin_realm` as `Destination-Realm`

Checks:
- `Destination-Host` equals stored `sc_origin_host`
- `Destination-Realm` equals stored `sc_origin_realm`
- `Origin-Host` and `Origin-Realm` are HSS identity values
- request app ID is `16777312`
- command code is `8388648`

Why:
- this was one of the earlier compliance gaps and should stay protected.

### 2. `TestALSC_SendStoresPendingSession`

Goal:
- verify `SendALSCForIMSI` records an in-flight pending ALSC session after successful send

Checks:
- pending entry exists for generated Session-Id
- pending entry holds the expected IMSI and SC address

Why:
- `ASA` cleanup now depends on this correlation state.

### 3. `TestASA_SuccessDeletesMWD`

Goal:
- verify `ASA` with `Result-Code=2001` deletes the matching MWD record

Checks:
- MWD delete called once
- correct IMSI + SC address deleted
- pending session removed

Why:
- this is now the success criterion for HSS-side ALSC completion.

### 4. `TestASA_FailureRetainsMWD`

Goal:
- verify non-success `ASA` leaves MWD in place for retry

Checks:
- no delete call
- pending session removed or handled consistently
- failure result code logged or observable

Why:
- this is explicitly claimed by docs and should be enforced.

### 5. `TestASA_UnknownSessionDoesNotDeleteMWD`

Goal:
- verify stray or duplicate `ASA` does not delete unrelated MWD

Checks:
- no delete call
- function exits safely

Why:
- protects against unexpected or replayed answers.

### 6. `TestASA_MissingSessionIDDoesNotDeleteMWD`

Goal:
- verify malformed `ASA` with no `Session-Id` is ignored safely

Checks:
- no delete call
- no panic

### 7. `TestASA_UnmarshalFailureDoesNotDeleteMWD`

Goal:
- verify malformed `ASA` payload does not corrupt state

Checks:
- no delete call
- no panic

## Priority 2: ALSC Triggering Behavior

### 8. `TestSendALSCForIMSI_NoMWD_NoSend`

Goal:
- verify no ALSC is sent when subscriber has no pending MWD records

Checks:
- no peer lookup
- no send attempt

### 9. `TestSendALSCForIMSI_PeerNotConnected_MWDRetained`

Goal:
- verify disconnected SMSC does not cause MWD deletion

Checks:
- peer lookup fails
- no send attempt
- no delete call

### 10. `TestSendALSCForIMSI_MultipleMWDRecords`

Goal:
- verify HSS sends one ALSC per pending MWD record / SMSC target

Checks:
- all pending records processed
- separate Session-Ids generated
- each send uses the right host/realm/sc-address pair

### 11. `TestSendALSCForIMSI_SubscriberMissing`

Goal:
- verify missing subscriber short-circuits safely

Checks:
- no send attempt
- no delete call

### 12. `TestSendALSCForIMSI_SendFailureRetainsMWD`

Goal:
- verify write failure leaves MWD for retry on next ULR

Checks:
- no delete call
- no pending session recorded if send never completed

## Priority 3: RSDS Negative and Persistence Cases

### 13. `TestRSDS_StoreMWDFailureReturnsUnableToComply`

Goal:
- verify store-layer failure returns failure answer and does not silently succeed

Checks:
- error answer generated
- no false success result

### 14. `TestRSDS_DeleteMWDFailureStillReturnsSuccessOrChosenBehavior`

Goal:
- pin down the intended behavior when `SuccessfulTransfer` arrives but MWD deletion fails

Checks:
- whatever behavior is chosen is asserted explicitly

Why:
- current code warns and still returns success; if that is intentional, it should be tested.

### 15. `TestRSDS_PersistsOriginRealm`

Goal:
- preserve the fix that stores `Origin-Realm` for later ALSC routing

Checks:
- persisted MWD record contains exact realm from request

### 16. `TestRSDS_PersistsMWDStatusFlags`

Goal:
- verify MNRF vs MCEF persistence in `mwd_status_flags`

Checks:
- absent-user stores `0x02`
- memory-capacity stores `0x04`

## Priority 4: Startup / Dictionary / Integration Tests

### 17. `TestLoadMSISDNSupplement_Idempotent`

Goal:
- verify repeated startup calls do not fail or double-register AVPs

Checks:
- first call succeeds
- second call succeeds
- no duplicate registration error

### 18. `TestServerStartup_LoadsMSISDNSupplement`

Goal:
- verify `NewServer` actually calls `s6c.LoadMSISDNSupplement()`

Checks:
- startup path includes supplement
- MSISDN-addressed `SRI-SM` request can unmarshal using the real server startup path

Why:
- this closes the gap between unit tests and runtime wiring.

### 19. `TestMSISDNAddressedSRISM_RealStartupPath`

Goal:
- verify the resolved production bug stays fixed without test-only setup drift

Checks:
- initialize dictionaries exactly as production startup does
- unmarshal `SRI-SM` by MSISDN successfully

### 20. `TestMSISDNAddressedRSDS_RealStartupPath`

Goal:
- same as above, but for `RSDS`

## Priority 5: Optional Logging Assertions

These are lower priority, but useful if log stability matters operationally.

### 21. `TestALSCSendLogFields`

Checks:
- IMSI
- `sc_origin_host`
- `sc_addr`
- `session_id`

### 22. `TestASASuccessLogFields`

Checks:
- IMSI
- `sc_addr`
- `origin_host`

### 23. `TestASAFailureLogFields`

Checks:
- failure `Result-Code`
- IMSI
- `sc_addr`

## Suggested Implementation Order

1. Add direct unit tests for `ALSC` / `ASA`
2. Add negative-path tests for send failure and unknown session handling
3. Add persistence tests for `Origin-Realm` and `MWDStatusFlags`
4. Add startup-path tests for `LoadMSISDNSupplement`
5. Add optional logging assertions

## Completion Criteria

The HSS S6c test story should be considered complete enough for SMSC integration when:

- outbound `SRI-SM` and `RSDS` behavior remains covered
- inbound `ASA` success/failure handling is covered
- `ALSC` request construction is covered
- MWD retention/deletion semantics are covered
- production startup path for MSISDN-addressed requests is covered

## Recommended Test File Layout

Option A:
- keep everything in `internal/diameter/s6c/s6c_spec_test.go`

Option B:
- split by concern:
  - `s6c_spec_test.go` for `SRI-SM` / `RSDS`
  - `s6c_alsc_test.go` for `ALSC` / `ASA`
  - `s6c_startup_test.go` for supplement-dictionary / startup-path checks

Recommended: Option B, to keep the spec tests easier to maintain.
