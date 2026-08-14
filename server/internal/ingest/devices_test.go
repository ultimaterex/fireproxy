package ingest_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"fireproxy/pkg/device"
	"fireproxy/pkg/inventory"
	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/ingest"
	"fireproxy/server/internal/store"
)

func TestCatalogPost(t *testing.T) {
	cs := store.NewCatalogStore()
	h := &ingest.CatalogHandler{Store: cs, Auth: auth.StaticToken("tok")}
	body, _ := json.Marshal(inventory.Catalog{
		TS:   1,
		Host: "Firewalla",
		Devices: []inventory.Device{
			{Device: device.Device{MAC: "AA:BB", Name: "Test", IP: "10.0.0.1"}},
		},
		Network:  []inventory.NetworkIface{{Name: "br0", Desc: "Core", Type: "lan", UUID: "u1"}},
		Policies: []inventory.Policy{{ID: "1", Action: "block", HitCount: 10}},
		Tags:     []inventory.Tag{{ID: "10", Name: "Routers"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/catalog", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("got %d %s", rr.Code, rr.Body.String())
	}
	cat, ok := cs.Get()
	if !ok || len(cat.Devices) != 1 || len(cat.Network) != 1 {
		t.Fatalf("%+v", cat)
	}
}

func TestDevicesPostAndStore(t *testing.T) {
	cs := store.NewCatalogStore()
	h := &ingest.DevicesHandler{Store: cs, Auth: auth.StaticToken("tok")}
	body, _ := json.Marshal(device.Inventory{
		TS:   1,
		Host: "Firewalla",
		Devices: []device.Device{
			{MAC: "AA:BB:CC:DD:EE:FF", Name: "Test", IP: "10.0.0.1"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/devices", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("got %d %s", rr.Code, rr.Body.String())
	}
	cat, ok := cs.Get()
	if !ok || len(cat.Devices) != 1 || cat.Devices[0].Name != "Test" {
		t.Fatalf("%+v ok=%v", cat, ok)
	}
}
