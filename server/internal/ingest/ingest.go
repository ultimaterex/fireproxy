package ingest

import (
	"encoding/json"
	"io"
	"net/http"

	"fireproxy/pkg/snapshot"
	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/store"
)

// Handler accepts agent POSTs.
type Handler struct {
	Store *store.MemoryStore
	Auth  auth.AgentVerifier
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Auth == nil || !h.Auth.AuthorizeAgent(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	var snap snapshot.Snapshot
	if err := json.Unmarshal(body, &snap); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if snap.Ifaces == nil {
		snap.Ifaces = map[string]snapshot.IfaceStats{}
	}
	view := h.Store.Add(snap)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(view)
}
