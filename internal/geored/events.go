package geored

// events.go -- Event type constants, payload structs, and the Publisher interface.
//
// All GeoRed state changes flow through the Publisher interface. Handlers and
// store methods call Publish() after every successful write; the Manager fans
// the event out to all configured peers asynchronously.

import (
	"encoding/json"
	"time"
)

// EventType identifies the kind of state change being replicated.
type EventType string

const (
	// Dynamic state events — fired by Diameter / GSUP handlers.
	EventSQNUpdate     EventType = "sqn_update"
	EventServingMME    EventType = "serving_mme"
	EventServingSGSN   EventType = "serving_sgsn"
	EventServingVLR    EventType = "serving_vlr"
	EventServingMSC    EventType = "serving_msc"
	EventIMSSCSCF      EventType = "ims_scscf"
	EventIMSPCSCF      EventType = "ims_pcscf"
	EventGxSessionAdd  EventType = "gx_session_add"
	EventGxSessionDel  EventType = "gx_session_del"

	// OAM events — fired by API write handlers.
	EventSubscriberPut EventType = "subscriber_put"
	EventSubscriberDel EventType = "subscriber_del"
	EventAUCPut        EventType = "auc_put"
	EventAUCDel        EventType = "auc_del"
	EventAPNPut        EventType = "apn_put"
	EventAPNDel        EventType = "apn_del"
	EventIMSSubPut     EventType = "ims_sub_put"
	EventIMSSubDel     EventType = "ims_sub_del"
	EventEIRPut        EventType = "eir_put"
	EventEIRDel        EventType = "eir_del"
)

// Event is a single replication unit sent between peers.
type Event struct {
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// Batch is the wire format for a single HTTP push — multiple events in one request.
type Batch struct {
	Source string  `json:"source"` // sending node_id, used for loop prevention
	Events []Event `json:"events"`
}

// ── Payload structs ───────────────────────────────────────────────────────────

type PayloadSQNUpdate struct {
	AUCID int   `json:"auc_id"`
	SQN   int64 `json:"sqn"`
}

type PayloadServingMME struct {
	IMSI              string     `json:"imsi"`
	ServingMME        *string    `json:"serving_mme"`
	ServingMMERealm   *string    `json:"serving_mme_realm"`
	ServingMMEPeer    *string    `json:"serving_mme_peer"`
	Timestamp         *time.Time `json:"timestamp"`
	MCC               *string    `json:"mcc,omitempty"`
	MNC               *string    `json:"mnc,omitempty"`
	TAC               *string    `json:"tac,omitempty"`
	ENodeBID          *string    `json:"enodeb_id,omitempty"`
	CellID            *string    `json:"cell_id,omitempty"`
	ECI               *string    `json:"eci,omitempty"`
	LocationTimestamp *time.Time `json:"location_timestamp,omitempty"`
}

type PayloadServingSGSN struct {
	IMSI        string     `json:"imsi"`
	ServingSGSN *string    `json:"serving_sgsn"`
	Timestamp   *time.Time `json:"timestamp"`
}

type PayloadServingVLR struct {
	IMSI       string     `json:"imsi"`
	ServingVLR *string    `json:"serving_vlr"`
	Timestamp  *time.Time `json:"timestamp"`
}

type PayloadServingMSC struct {
	IMSI       string     `json:"imsi"`
	ServingMSC *string    `json:"serving_msc"`
	Timestamp  *time.Time `json:"timestamp"`
}

type PayloadIMSSCSCF struct {
	MSISDN    string     `json:"msisdn"`
	SCSCF     *string    `json:"scscf"`
	Realm     *string    `json:"realm"`
	Peer      *string    `json:"peer"`
	Timestamp *time.Time `json:"timestamp"`
}

type PayloadIMSPCSCF struct {
	MSISDN    string     `json:"msisdn"`
	PCSCF     *string    `json:"pcscf"`
	Realm     *string    `json:"realm"`
	Peer      *string    `json:"peer"`
	Timestamp *time.Time `json:"timestamp"`
}

type PayloadGxSessionAdd struct {
	PCRFSessionID string     `json:"pcrf_session_id"`
	IMSI          string     `json:"imsi"`
	MSISDN        *string    `json:"msisdn,omitempty"`
	APNID         *int       `json:"apn_id,omitempty"`
	APNName       *string    `json:"apn_name,omitempty"`
	PGWIP         *string    `json:"pgw_ip,omitempty"`
	UEIP          *string    `json:"ue_ip,omitempty"`
	Timestamp     *time.Time `json:"timestamp,omitempty"`
}

type PayloadGxSessionDel struct {
	PCRFSessionID string `json:"pcrf_session_id"`
}

// OAM payloads carry the full JSON-serialised model object so the peer can
// upsert or delete it directly. We use json.RawMessage to avoid importing
// models here (would create a cycle since models has no knowledge of geored).
type PayloadOAMPut struct {
	Record json.RawMessage `json:"record"`
}

type PayloadOAMDel struct {
	ID interface{} `json:"id"` // int or string depending on resource
}

// ── Publisher interface ───────────────────────────────────────────────────────

// Publisher is implemented by *Manager. Inject a NoopPublisher when GeoRed is
// disabled so callers never need nil checks.
type Publisher interface {
	Publish(e Event)
}

// NoopPublisher silently discards all events. Used when geored.enabled = false.
type NoopPublisher struct{}

func (NoopPublisher) Publish(Event) {}

// TypedPublisher exposes convenience publish methods for every event type.
// *Manager implements this interface. Inject NoopTypedPublisher when GeoRed is
// disabled so callers never need nil checks.
type TypedPublisher interface {
	PublishSQNUpdate(aucID int, sqn int64)
	PublishServingMME(p PayloadServingMME)
	PublishServingSGSN(p PayloadServingSGSN)
	PublishServingVLR(p PayloadServingVLR)
	PublishServingMSC(p PayloadServingMSC)
	PublishIMSSCSCF(p PayloadIMSSCSCF)
	PublishIMSPCSCF(p PayloadIMSPCSCF)
	PublishGxSessionAdd(p PayloadGxSessionAdd)
	PublishGxSessionDel(sessionID string)
	PublishOAMPut(evType EventType, record interface{})
	PublishOAMDel(evType EventType, id interface{})
}

// NoopTypedPublisher silently discards all events.
type NoopTypedPublisher struct{}

func (NoopTypedPublisher) PublishSQNUpdate(int, int64)             {}
func (NoopTypedPublisher) PublishServingMME(PayloadServingMME)     {}
func (NoopTypedPublisher) PublishServingSGSN(PayloadServingSGSN)   {}
func (NoopTypedPublisher) PublishServingVLR(PayloadServingVLR)     {}
func (NoopTypedPublisher) PublishServingMSC(PayloadServingMSC)     {}
func (NoopTypedPublisher) PublishIMSSCSCF(PayloadIMSSCSCF)         {}
func (NoopTypedPublisher) PublishIMSPCSCF(PayloadIMSPCSCF)         {}
func (NoopTypedPublisher) PublishGxSessionAdd(PayloadGxSessionAdd) {}
func (NoopTypedPublisher) PublishGxSessionDel(string)              {}
func (NoopTypedPublisher) PublishOAMPut(EventType, interface{})    {}
func (NoopTypedPublisher) PublishOAMDel(EventType, interface{})    {}

// ── helpers ───────────────────────────────────────────────────────────────────

// newEvent builds an Event with the current timestamp and JSON-encodes payload.
func newEvent(t EventType, payload interface{}) Event {
	raw, _ := json.Marshal(payload)
	return Event{
		Type:      t,
		Timestamp: time.Now().UTC(),
		Payload:   raw,
	}
}
