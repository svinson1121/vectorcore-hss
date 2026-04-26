# VectorCore HSS Web UI User Manual

This manual describes the current web UI under `web/src`. It is organized the same way as the sidebar navigation and the in-page tabs.

## Navigation Overview

- `Dashboard`
- `SIM Cards / AUC`
  - `SIM Cards / AUC`
  - `Algorithm Profiles`
- `Subscribers`
  - `Subscribers`
  - `Attributes`
  - `Routings`
- `IMS Subscribers`
  - `IMS Subscribers`
  - `IFC Profiles`
- `APNs`
  - `APNs`
  - `Charging & TFT`
- `EIR`
- `Roaming`
- `Sessions`
  - `4G PDU Sessions`
  - `5G PDU Sessions`
- `Metrics`
- `OAM`
- `Subscriber Wizard` (sidebar action button)

## Dashboard

Purpose: read-only operational summary for subscribers, active sessions, request counters, uptime, and recent S6a authentication failures.

Inputs:

- None. This page is read-only.

## SIM Cards / AUC

### SIM Cards / AUC Tab

Purpose: manage SIM authentication material and SIM metadata.

Page controls:

- `Search`: filters the list by IMSI, ICCID, SIM vendor, or batch name.
- `Refresh`: reloads the table.
- `Add AUC`: opens the create form.

Form fields:

| Field | Meaning / usage |
| --- | --- |
| `IMSI` | Primary SIM identity. Required on create. Locked on edit. |
| `ICCID` | Physical or eSIM card identifier. Optional inventory field. |
| `SIM Vendor` | Vendor/manufacturer label for stock tracking. |
| `Batch Name` | Batch or lot label for grouped SIM imports. |
| `Algorithm Profile` | Optional custom Milenage profile. Leave blank to use standard Milenage constants. |
| `eSIM` | Marks the SIM as eSIM-capable. Enables the `LPA` field. |
| `LPA (eSIM activation code)` | eSIM activation/bootstrap code used for eSIM workflows. |
| `Ki` | 128-bit authentication key, entered as 32 hex characters. Required on create. On edit, leaving it blank keeps the existing value. |
| `OPc` | Operator variant key, entered as 32 hex characters. Required on create. On edit, leaving it blank keeps the existing value. |
| `AMF` | Authentication Management Field. Usually `8000`. |
| `KID` | Optional key identifier metadata. |
| `PSK` | Optional pre-shared-key field for SIM-related workflows or vendor metadata. |
| `DES` | Optional DES-related metadata field. |
| `ADM1` | Optional administrative code field. |
| `PIN1` | Optional user PIN code 1. |
| `PIN2` | Optional user PIN code 2. |
| `PUK1` | Optional unblock code for PIN1. |
| `PUK2` | Optional unblock code for PIN2. |
| `Misc 1` | Free-form metadata field. |
| `Misc 2` | Free-form metadata field. |
| `Misc 3` | Free-form metadata field. |
| `Misc 4` | Free-form metadata field. |

Notes:

- On edit, key fields can be hidden or shown with the `Show Keys` / `Hide Keys` toggle.
- Delete is blocked if the AUC is still attached to a subscriber.

### Algorithm Profiles Tab

Purpose: define custom Milenage constant sets for SIMs that do not use the default 3GPP constants.

Page controls:

- `Refresh`: reloads profiles.
- `Add Profile`: opens the create form.

Form fields:

| Field | Meaning / usage |
| --- | --- |
| `Profile Name` | Human-readable profile name. Required. |
| `c1` | Milenage constant `c1`, entered as 32 hex characters. |
| `c2` | Milenage constant `c2`, entered as 32 hex characters. |
| `c3` | Milenage constant `c3`, entered as 32 hex characters. |
| `c4` | Milenage constant `c4`, entered as 32 hex characters. |
| `c5` | Milenage constant `c5`, entered as 32 hex characters. |
| `r1` | Milenage rotation value in bits. Default `64`. |
| `r2` | Milenage rotation value in bits. Default `0`. |
| `r3` | Milenage rotation value in bits. Default `32`. |
| `r4` | Milenage rotation value in bits. Default `64`. |
| `r5` | Milenage rotation value in bits. Default `96`. |

Notes:

- Rotation values must be valid Milenage bit rotations.
- Delete is blocked if any AUC currently references the profile.

## Subscribers

### Subscribers Tab

Purpose: manage EPC/5GC subscriber provisioning and core subscriber policy.

Page controls:

- `Search`: filters by IMSI or MSISDN.
- `Refresh`: reloads the list.
- `Add Subscriber`: opens the create form.

Form fields:

| Field | Meaning / usage |
| --- | --- |
| `IMSI / ICCID` | AUC selector. This links the subscriber to an existing SIM/AUC record and sets the subscriber IMSI from that record. Required. |
| `MSISDN` | Subscriber telephone number. Optional in the form, but usually populated for voice/SMS use cases. |
| `Enabled` | Master service toggle. If cleared, the subscriber is disabled. |
| `Roaming Enabled` | Allows or denies roaming for the subscriber. |
| `APNs` | List of allowed APNs. The first APN added becomes the default APN. |
| `UE AMBR DL (bps)` | Maximum aggregate downlink bitrate for the UE. |
| `UE AMBR UL (bps)` | Maximum aggregate uplink bitrate for the UE. |
| `NAM (Network Access Mode)` | Access mode: `0` for packet and circuit, `2` for packet only. |
| `Subscribed RAU/TAU Timer (s)` | Mobility update timer value stored for the subscriber. |
| `UTRAN (3G)` | Access restriction bit for 3G access. Check to set that bit. |
| `GERAN (2G)` | Access restriction bit for 2G access. |
| `GAN` | Access restriction bit for GAN access. |
| `I-HSPA-Evo` | Access restriction bit for I-HSPA-Evo. |
| `E-UTRAN (4G)` | Access restriction bit for LTE/E-UTRAN access. |
| `HO Non-3GPP` | Access restriction bit for handover to non-3GPP access. |
| `NR as Secondary RAT (NSA)` | Access restriction bit for NSA NR usage. |
| `NR in Unlicensed Spectrum` | Access restriction bit for NR unlicensed operation. |
| `NR in 5GS (SA)` | Access restriction bit for standalone 5G access. |
| `Non-3GPP 5GS` | Access restriction bit for non-3GPP access into 5GS. |
| `Slice/Service Type (SST)` | Used to add an allowed S-NSSAI entry for 5G subscriber data. |
| `Slice Differentiator (SD)` | Optional 6-hex-digit slice differentiator paired with the selected SST. |

Notes:

- The `Raw value` shown under Access Restrictions is the bitmask produced by the selected RAT checkboxes.
- If no NSSAI slices are added, the UI notes that the network default slice is used.
- Delete is blocked if the subscriber is still referenced by IMS subscriber data, subscriber attributes, or subscriber routings.

### Attributes Tab

Purpose: store custom key/value metadata per subscriber.

Page controls:

- `Search`: filters by IMSI, attribute key, or value.
- `Refresh`: reloads the list.
- `Add Attribute`: opens the create form.

Form fields:

| Field | Meaning / usage |
| --- | --- |
| `Subscriber` | Subscriber record the attribute belongs to. Required. Locked on edit. |
| `Key` | Attribute name. Required. Best used for stable machine-readable keys. |
| `Value` | Attribute value. Free-form text. |

### Routings Tab

Purpose: define per-subscriber, per-APN routing overrides such as a static IP.

Page controls:

- `Refresh`: reloads the list.
- `Add Routing`: opens the create form.

Form fields:

| Field | Meaning / usage |
| --- | --- |
| `Subscriber` | Subscriber that the routing override applies to. Required. Locked on edit. |
| `APN` | APN the override applies to. Required. Locked on edit. |
| `IP Version` | IP family for the routing record: IPv4, IPv6, IPv4v6, or IPv4 or v6. |
| `Static IP Address` | Optional statically assigned IP address for that subscriber/APN pair. |

## IMS Subscribers

### IMS Subscribers Tab

Purpose: manage IMS/VoLTE subscriber records.

Page controls:

- `Search`: filters by MSISDN or IMSI.
- `Refresh`: reloads the list.
- `Add IMS Subscriber`: opens the create form.

Form fields:

| Field | Meaning / usage |
| --- | --- |
| `IMSI` | Optional link to an existing HSS subscriber. If selected, the UI can prefill MSISDN-related fields. |
| `MSISDN` | Primary IMS subscriber number. Required. |
| `MSISDN List` | Comma-separated list of additional MSISDNs or aliases. |
| `IFC Profile` | Initial Filter Criteria profile used for IMS service triggering. Required. |

### IFC Profiles Tab

Purpose: manage the Initial Filter Criteria XML templates used by IMS subscribers.

Page controls:

- `Refresh`: reloads profiles.
- `Add Profile`: opens the create form.

Form fields:

| Field | Meaning / usage |
| --- | --- |
| `Profile Name` | Human-readable name for the IFC profile. Required. |
| `IFC XML Data` | XML fragment containing the subscriber's iFC service logic. Required and validated for well-formed XML. |

Notes:

- Delete is blocked if an IMS subscriber is still using the profile.

## APNs

### APNs Tab

Purpose: provision APN/DNN definitions and default QoS behavior.

Page controls:

- `Refresh`: reloads the APN list.
- `Add APN`: opens the create form.

Form fields:

| Field | Meaning / usage |
| --- | --- |
| `APN Name` | APN/DNN name presented to the network, for example `internet`. Required. |
| `IP Version` | Allowed IP family for sessions on this APN. |
| `Charging Characteristics` | 4-digit charging characteristics value stored on the APN. |
| `PGW Address` | Optional PGW address to associate with the APN. |
| `SGW Address` | Optional SGW address to associate with the APN. |
| `APN AMBR DL (bps)` | APN-level aggregate maximum downlink bitrate. |
| `APN AMBR UL (bps)` | APN-level aggregate maximum uplink bitrate. |
| `QCI` | Default QoS Class Identifier. |
| `ARP Priority` | Allocation and Retention Priority level. |
| `ARP Preemption Capability` | Whether bearers on this APN can preempt lower-priority bearers. |
| `ARP Preemption Vulnerability` | Whether bearers on this APN may be preempted by higher-priority ones. |
| `Charging Rule List` | Set of charging rules attached to this APN. |
| `NB-IoT Enabled` | Enables NB-IoT-specific fields for the APN. |
| `NIDD SCEF ID` | Optional SCEF identifier for NIDD service. |
| `NIDD SCEF Realm` | Optional SCEF realm for NIDD service. |
| `NIDD Mechanism` | Selects the NIDD path, for example SGi or SCEF. |
| `NIDD RDS` | Enables or disables NIDD RDS behavior. |
| `NIDD Preferred Data Mode` | Preferred mode flag for NIDD traffic. |

Notes:

- Delete is blocked if the APN is still used by a subscriber or subscriber routing entry.

### Charging & TFT Tab

Purpose: manage charging rules and traffic flow templates.

#### Charging Rules

Page controls:

- `Refresh`: reloads both charging rules and TFTs.
- `Add Rule`: opens the charging rule form.

Form fields:

| Field | Meaning / usage |
| --- | --- |
| `Rule Name` | Charging rule identifier. Required. |
| `QCI` | QoS Class Identifier for this rule. |
| `ARP Priority` | Allocation and Retention Priority for this rule. |
| `ARP Preemption Capability` | Whether this rule can preempt lower-priority bearers. |
| `ARP Preemption Vulnerability` | Whether this rule can itself be preempted. |
| `MBR DL (bps)` | Maximum downlink bitrate. |
| `MBR UL (bps)` | Maximum uplink bitrate. |
| `GBR DL (bps)` | Guaranteed downlink bitrate. |
| `GBR UL (bps)` | Guaranteed uplink bitrate. |
| `TFT Group ID` | Optional TFT group that links this charging rule to one or more TFT entries. |
| `Precedence` | Optional precedence value used when multiple rules can match. |
| `Rating Group` | Optional charging/rating group identifier. |

Notes:

- Delete is blocked if an APN still references the charging rule.

#### TFT Entries

Page controls:

- `Add TFT`: opens the TFT form.

Form fields:

| Field | Meaning / usage |
| --- | --- |
| `TFT Group ID` | Group number used to associate one or more TFT rows with a charging rule. Required. |
| `Direction` | Traffic direction the filter applies to: downlink, uplink, or bidirectional. |
| `TFT String` | The actual traffic filter rule string. Required. |

Notes:

- Delete is blocked if a charging rule still references that TFT group.

## EIR

Purpose: manage IMEI/IMSI equipment rules and the TAC device database.

### EIR Rules

Page controls:

- `Search`: filters by IMEI or IMSI.
- `Refresh`: reloads rules.
- `Add Rule`: opens the EIR rule form.

Form fields:

| Field | Meaning / usage |
| --- | --- |
| `IMEI / Pattern` | IMEI value or regex pattern to match. Optional if IMSI is supplied. |
| `IMSI / Pattern` | IMSI value or regex pattern to match. Optional if IMEI is supplied. |
| `Match Mode` | `Exact match` compares literal values. `Regex` interprets the patterns as regular expressions. |
| `Action on match` | Result returned on a match: allow, block, or greylist. |

### TAC Device Database

Purpose: enrich IMEI/TAC values with make and model data.

Controls:

- `Add Entry`: opens the TAC create form.
- `Import CSV`: opens a file selector for TAC CSV import.
- `Export CSV`: downloads the TAC database as CSV.
- `TAC Lookup / Edit`: lets you search by IMEI and then edit or delete the matched TAC.

TAC add/edit fields:

| Field | Meaning / usage |
| --- | --- |
| `TAC` | 8-digit Type Allocation Code. Required on create. Locked on edit. |
| `Make` | Device vendor/manufacturer name. |
| `Model` | Device model name. |

TAC lookup field:

| Field | Meaning / usage |
| --- | --- |
| `IMEI` | 15-digit IMEI used to extract the TAC and look up device metadata. |

File input:

| Field | Meaning / usage |
| --- | --- |
| `CSV file` | CSV source for bulk TAC import. Triggered by `Import CSV`. |

## Roaming

Purpose: define PLMN-specific roaming policy rules.

Page controls:

- `Search`: filters by partner name, MCC, or MNC.
- `Refresh`: reloads rules.
- `Add Rule`: opens the create form.

Form fields:

| Field | Meaning / usage |
| --- | --- |
| `Name` | Human-readable roaming partner or rule name. |
| `MCC` | Mobile Country Code for the target PLMN. |
| `MNC` | Mobile Network Code for the target PLMN. |
| `Allow Roaming` | If checked, matching roaming attempts are allowed. If cleared, they are blocked. |
| `Enabled` | Enables or disables the rule without deleting it. |

## Sessions

Purpose: view live session state.

Submenus:

- `4G PDU Sessions`
- `5G PDU Sessions`

Inputs:

- No editable provisioning fields.
- `Refresh` reloads both session tables.
- Tab buttons switch between 4G and 5G views.

## Metrics

Purpose: show Prometheus-derived counters, charts, and raw `hss_` metric summaries.

Inputs:

- No editable fields.
- `Refresh` reloads the metric scrape.

## OAM

Purpose: operations, administration, and maintenance functions.

Read-only sections:

- `System Identity`
- `Health`
- `Connected Diameter Peers`
- `Connected GSUP Peers`
- `Connected SBI Peers`
- `Active Emergency Sessions`
- `Operation Log` viewer

### Send Cancel Location Request (CLR)

Purpose: force a UE detach by sending a CLR to the serving MME.

Fields:

| Field | Meaning / usage |
| --- | --- |
| `Subscriber (known IMSI)` | Dropdown for locally provisioned subscribers. Use when the IMSI already exists in the HSS database. |
| `Manual IMSI (roaming UE)` | Manual IMSI entry for a roaming or otherwise not locally selectable UE. |

Notes:

- Use either the dropdown or the manual IMSI field.
- `Send CLR` is disabled until one of those fields is populated.

### Database Backup & Restore

Purpose: export and restore provisioning data.

Fields:

| Field | Meaning / usage |
| --- | --- |
| `Restore file` | Hidden file selector triggered by `Restore Database`. Accepts `.zip`, `.sql`, `.db`, `.sqlite`, and `.json`. |

Notes:

- `Backup Database` downloads a JSON backup.
- Restore overwrites current data.

## Subscriber Wizard

Purpose: guided multi-step subscriber creation flow opened from the sidebar button.

### Step 1: SIM / AUC

| Field | Meaning / usage |
| --- | --- |
| `IMSI` | SIM identity to create. Required. |
| `ICCID` | Card identifier. Optional. |
| `SIM Vendor` | Inventory/vendor label. |
| `Batch Name` | Batch/lot label. |
| `Algorithm Profile` | Optional custom Milenage profile. |
| `eSIM` | Marks the SIM as eSIM-capable. |
| `Ki` | 32-hex-character authentication key. Required. |
| `OPc` | 32-hex-character operator key. Required. |
| `AMF` | Authentication Management Field. |
| `PIN1` | Optional SIM PIN1. |
| `PIN2` | Optional SIM PIN2. |
| `PUK1` | Optional PUK1. |
| `PUK2` | Optional PUK2. |

### Step 2: Subscriber

| Field | Meaning / usage |
| --- | --- |
| `MSISDN` | Primary subscriber number. Required. |
| `Enabled` | Subscriber service enabled flag. |
| `Roaming` | Subscriber roaming enabled flag. |
| `APNs` | Allowed APN list; first added APN becomes default. |
| `UE AMBR DL (bps)` | Subscriber downlink aggregate bitrate. |
| `UE AMBR UL (bps)` | Subscriber uplink aggregate bitrate. |
| `NAM` | Network access mode. |
| `RAU/TAU Timer (s)` | Mobility update timer. |

### Step 3: IMS Subscriber

| Field | Meaning / usage |
| --- | --- |
| `Additional MSISDNs (MSISDN List)` | Optional comma-separated alias numbers. |
| `IFC Profile` | Optional IFC profile to attach during IMS subscriber creation. |

Notes:

- This step is optional. `Skip IMS` creates only the AUC and subscriber records.

### Step 4: Review

Purpose: final confirmation screen before the wizard creates records.

Inputs:

- No editable fields.
- `Finish` creates the AUC, subscriber, and optionally the IMS subscriber in sequence.
