package publish

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fireproxy/pkg/snapshot"
)

func TestPostJSONSuccess(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			t.Errorf("empty body")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	p := &Publisher{
		URL:        srv.URL,
		AgentToken: "secret",
		Client:     &http.Client{Timeout: 2 * time.Second},
	}
	err := p.PostJSON(snapshot.Snapshot{Host: "Firewalla", TS: 1, Ifaces: map[string]snapshot.IfaceStats{}})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("auth: %q", gotAuth)
	}
}

func TestPostJSONNoRetryOnFailure(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := &Publisher{URL: srv.URL, Client: &http.Client{Timeout: time.Second}}
	_ = p.PostJSON(snapshot.Snapshot{Host: "x", Ifaces: map[string]snapshot.IfaceStats{}})
	_ = p.PostJSON(snapshot.Snapshot{Host: "x", Ifaces: map[string]snapshot.IfaceStats{}})
	if n != 2 {
		t.Fatalf("expected exactly 2 attempts (no internal retry), got %d", n)
	}
}
