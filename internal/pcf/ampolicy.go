package pcf

// ampolicy.go — Npcf_AMPolicyControl handlers.
//
// POST   /npcf-am-policy-control/v1/policies              — create association
// GET    /npcf-am-policy-control/v1/policies/{polAssoId}  — read
// POST   /npcf-am-policy-control/v1/policies/{polAssoId}/update — update
// DELETE /npcf-am-policy-control/v1/policies/{polAssoId}  — terminate
//
// AMF calls Create during every UE registration. Open5GS AMF will time out
// (10 s) and reject with PAYLOAD_NOT_FORWARDED if this endpoint is absent.
// We return a minimal PolicyAssociation with no triggers or restrictions.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type amPolicyAssociation struct {
	ID      string
	Request json.RawMessage
}

type amPolicyStore struct {
	mu   sync.RWMutex
	data map[string]*amPolicyAssociation
	seq  atomic.Uint64
}

func newAMPolicyStore() *amPolicyStore {
	return &amPolicyStore{data: make(map[string]*amPolicyAssociation)}
}

func (s *Server) registerAMPolicyRoutes(r chi.Router) {
	r.Post("/npcf-am-policy-control/v1/policies",
		s.wrapOAuth("npcf-am-policy-control", s.handleAMPolicyCreate))
	r.Get("/npcf-am-policy-control/v1/policies/{polAssoId}",
		s.wrapOAuth("npcf-am-policy-control", s.handleAMPolicyGet))
	r.Post("/npcf-am-policy-control/v1/policies/{polAssoId}/update",
		s.wrapOAuth("npcf-am-policy-control", s.handleAMPolicyUpdate))
	r.Delete("/npcf-am-policy-control/v1/policies/{polAssoId}",
		s.wrapOAuth("npcf-am-policy-control", s.handleAMPolicyDelete))
}

func (s *Server) handleAMPolicyCreate(w http.ResponseWriter, r *http.Request) {
	var body json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body = json.RawMessage(`{}`)
	}
	s.log.Info("pcf: AM policy create", zap.String("remote", r.RemoteAddr))

	id := fmt.Sprintf("%d", s.amPolicy.seq.Add(1))
	assoc := &amPolicyAssociation{ID: id, Request: body}
	s.amPolicy.mu.Lock()
	s.amPolicy.data[id] = assoc
	s.amPolicy.mu.Unlock()

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	location := fmt.Sprintf("%s://%s/npcf-am-policy-control/v1/policies/%s", scheme, r.Host, id)

	// Minimal PolicyAssociation — suppFeat is required by Open5GS OpenAPI parser.
	resp := map[string]interface{}{
		"request":  body,
		"suppFeat": "0",
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", location)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleAMPolicyGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "polAssoId")
	s.amPolicy.mu.RLock()
	assoc, ok := s.amPolicy.data[id]
	s.amPolicy.mu.RUnlock()
	if !ok {
		http.Error(w, `{"cause":"CONTEXT_NOT_FOUND"}`, http.StatusNotFound)
		return
	}
	resp := map[string]interface{}{"request": assoc.Request, "suppFeat": "0"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleAMPolicyUpdate(w http.ResponseWriter, r *http.Request) {
	// AMF sends update on location/RAT change; acknowledge with empty policy delta.
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{}`))
}

func (s *Server) handleAMPolicyDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "polAssoId")
	s.amPolicy.mu.Lock()
	delete(s.amPolicy.data, id)
	s.amPolicy.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}
