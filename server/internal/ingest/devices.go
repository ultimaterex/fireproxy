package ingest

import (
	"encoding/json"
	"io"
	"net/http"

	"fireproxy/pkg/device"
	"fireproxy/pkg/inventory"
	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/store"
)

// CatalogHandler accepts full inventory catalog POSTs.
type CatalogHandler struct {
	Store *store.CatalogStore
	Auth  auth.AgentVerifier
}

func (h *CatalogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Auth == nil || !h.Auth.AuthorizeAgent(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	var cat inventory.Catalog
	if err := json.Unmarshal(body, &cat); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	h.Store.Set(cat)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"accepted": true,
		"devices":  len(cat.Devices),
		"network":  len(cat.Network),
		"policies": len(cat.Policies),
		"tags":     len(cat.Tags),
	})
}

// DevicesHandler accepts legacy devices-only POSTs and merges into catalog.
type DevicesHandler struct {
	Store *store.CatalogStore
	Auth  auth.AgentVerifier
}

func (h *DevicesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Auth == nil || !h.Auth.AuthorizeAgent(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	var inv device.Inventory
	if err := json.Unmarshal(body, &inv); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	cat, _ := h.Store.Get()
	cat.TS = inv.TS
	cat.Host = inv.Host
	cat.Devices = make([]inventory.Device, 0, len(inv.Devices))
	for _, d := range inv.Devices {
		cat.Devices = append(cat.Devices, inventory.Device{Device: d})
	}
	h.Store.Set(cat)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"accepted": true,
		"count":    len(inv.Devices),
	})
}
