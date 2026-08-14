package ingest_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"fireproxy/pkg/snapshot"
	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/ingest"
	"fireproxy/server/internal/store"
)

func TestIngestAuthAndAccept(t *testing.T) {
	mem := store.NewMemoryStore(8)
	h := &ingest.Handler{Store: mem, Auth: auth.StaticToken("tok")}

	body, _ := json.Marshal(snapshot.Snapshot{
		TS: 1, Host: "Firewalla",
		Ifaces: map[string]snapshot.IfaceStats{"eth1": {RxBytes: 10, TxBytes: 20}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/ingest", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("want 202 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestIngestAPIKey(t *testing.T) {
	mem := store.NewMemoryStore(8)
	h := &ingest.Handler{Store: mem, Auth: auth.StaticToken("tok")}
	body, _ := json.Marshal(snapshot.Snapshot{TS: 2, Host: "x", Ifaces: map[string]snapshot.IfaceStats{}})
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "tok")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("got %d", rr.Code)
	}
}

func TestIngestAgentTokenSupersede(t *testing.T) {
	p, err := store.OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 7)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })

	first, err := auth.MintAgentCredential(p)
	if err != nil {
		t.Fatal(err)
	}
	v := &auth.AgentCreds{Persist: p}
	mem := store.NewMemoryStore(8)
	h := &ingest.Handler{Store: mem, Auth: v}
	body, _ := json.Marshal(snapshot.Snapshot{TS: 1, Host: "x", Ifaces: map[string]snapshot.IfaceStats{}})

	req := httptest.NewRequest(http.MethodPost, "/v1/ingest", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+first)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("first token want 202 got %d", rr.Code)
	}

	second, err := auth.MintAgentCredential(p)
	if err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/ingest", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+first)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("superseded want 401 got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/ingest", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+second)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("new token want 202 got %d", rr.Code)
	}
}
