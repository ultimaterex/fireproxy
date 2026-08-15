package agenthub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fireproxy/pkg/agentws"
	"fireproxy/server/internal/agentpkg"
	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/store"

	"github.com/gorilla/websocket"
)

func TestRequestFetchOffline(t *testing.T) {
	h := &Hub{Auth: auth.StaticToken("t")}
	st, n := h.RequestFetch(context.Background())
	if st != "agent_offline" || n != 0 {
		t.Fatalf("%s %d", st, n)
	}
}

func TestRequestCatalogPushRejectsConcurrent(t *testing.T) {
	h := &Hub{Auth: auth.StaticToken("t")}
	h.mu.Lock()
	h.online = true
	h.mu.Unlock()

	// Simulate an in-flight catalog request holding the waiter.
	inflight := make(chan string, 1)
	h.catalogMu.Lock()
	h.catalogWait = inflight
	h.catalogMu.Unlock()

	if st := h.RequestCatalogPush(context.Background()); st != "busy" {
		t.Fatalf("concurrent request: want busy, got %q", st)
	}
	// The original waiter must be untouched — the previous bug overwrote it,
	// orphaning the first caller.
	h.catalogMu.Lock()
	same := h.catalogWait == inflight
	h.catalogMu.Unlock()
	if !same {
		t.Fatal("busy request clobbered the in-flight waiter")
	}
}

func TestRequestCatalogPushClearsWaiterOnExit(t *testing.T) {
	h := &Hub{Auth: auth.StaticToken("t")}
	h.mu.Lock()
	h.online = true // online, but no conn → write fails
	h.mu.Unlock()

	if st := h.RequestCatalogPush(context.Background()); st != "agent_offline" {
		t.Fatalf("want agent_offline (no conn), got %q", st)
	}
	h.catalogMu.Lock()
	w := h.catalogWait
	h.catalogMu.Unlock()
	if w != nil {
		t.Fatal("waiter not cleared on exit; a timeout would wedge future requests as busy")
	}
	// A subsequent request must not be spuriously rejected as busy.
	if st := h.RequestCatalogPush(context.Background()); st != "agent_offline" {
		t.Fatalf("second request: want agent_offline, got %q", st)
	}
}

func TestFetchRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "p.db")
	p, err := store.OpenPersist(dbPath, 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	h := &Hub{Auth: auth.StaticToken("secret"), Persist: p}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/agent/ws", h.ServeWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/agent/ws"
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer secret")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Agent read loop
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var env agentws.Envelope
			if err := json.Unmarshal(data, &env); err != nil {
				continue
			}
			if env.Type == agentws.TypeLogsFetch {
				batch := agentws.Envelope{
					Type: agentws.TypeLogsBatch,
					LogsBatch: &agentws.LogsBatch{
						ID:   "b0",
						Done: true,
						Lines: []agentws.LogLine{
							{TS: time.Now().Unix(), Source: "unbound", Severity: "err", Line: "boom"},
						},
					},
				}
				b, _ := json.Marshal(batch)
				_ = conn.WriteMessage(websocket.TextMessage, b)
			}
		}
	}()

	// wait until hub marks online
	deadline := time.Now().Add(2 * time.Second)
	for !h.Online() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !h.Online() {
		t.Fatal("not online")
	}

	status, dropped := h.RequestFetch(context.Background())
	if status != "ok" {
		t.Fatalf("status %s dropped %d", status, dropped)
	}
	lines, err := p.QueryLogs(time.Now().Unix()-60, time.Now().Unix()+60, "", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0].Line != "boom" {
		t.Fatalf("%+v", lines)
	}
	_ = conn.Close()
	<-done
}

func TestHelloRecordsUpdateAfterOffline(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "p.db")
	p, err := store.OpenPersist(dbPath, 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	_ = p.SetAgentLastVersion("0.1.6")
	_ = p.SetAgentLastOfflineTS(time.Now().Unix() - 60)

	h := &Hub{Auth: auth.StaticToken("secret"), Persist: p}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/agent/ws", h.ServeWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/agent/ws"
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer secret")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	hello, _ := json.Marshal(agentws.Envelope{
		Type:  agentws.TypeHello,
		Hello: &agentws.Hello{Host: "fw", Version: "0.1.7"},
	})
	if err := conn.WriteMessage(websocket.TextMessage, hello); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rows, _ := p.QueryAgentEvents(0, 10)
		if len(rows) >= 2 {
			kinds := []string{rows[0].Kind, rows[1].Kind}
			// newest first: updated then offline (or reverse insert order same ts — both id order)
			got := map[string]bool{}
			for _, r := range rows {
				got[r.Kind] = true
			}
			if got["offline"] && got["updated"] {
				v, _ := p.AgentLastVersion()
				if v != "0.1.7" {
					t.Fatalf("last ver %q", v)
				}
				ts, _ := p.AgentLastOfflineTS()
				if ts != 0 {
					t.Fatalf("offline ts %d", ts)
				}
				return
			}
			_ = kinds
		}
		time.Sleep(20 * time.Millisecond)
	}
	rows, _ := p.QueryAgentEvents(0, 10)
	t.Fatalf("events %+v", rows)
}

func TestSelfUpdateArchSelects(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"fireproxy-agent-arm64": "arm-bytes",
		"fireproxy-agent-amd64": "x86-bytes",
		"VERSION":               "9.9.9\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := agentpkg.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	wantBin, _ := pkg.Bin("amd64")

	h := &Hub{Auth: auth.StaticToken("secret"), Package: pkg, PublicBase: func() string { return "http://fp.local" }}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/agent/ws", h.ServeWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/agent/ws"
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer secret")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	updates := make(chan *agentws.AgentUpdate, 1)
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var env agentws.Envelope
			if err := json.Unmarshal(data, &env); err != nil {
				continue
			}
			if env.Type == agentws.TypeAgentUpdate {
				updates <- env.AgentUpdate
			}
		}
	}()

	// Announce as an amd64 (x86) Firewalla.
	hello := agentws.Envelope{Type: agentws.TypeHello, Hello: &agentws.Hello{Version: "0.0.1", Arch: "amd64", SelfUpdate: true}}
	b, _ := json.Marshal(hello)
	if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for h.Info().Arch != "amd64" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if h.Info().Arch != "amd64" {
		t.Fatal("hub did not record amd64 arch")
	}

	// Auto-update should already have fired from hello (0.0.1 << 9.9.9).
	select {
	case u := <-updates:
		if !strings.HasSuffix(u.URL, "/agent/fireproxy-agent?arch=amd64") {
			t.Fatalf("url = %q, want amd64-suffixed", u.URL)
		}
		if u.SHA256 != wantBin.SHA256 {
			t.Fatalf("sha = %s, want amd64 sha %s", u.SHA256, wantBin.SHA256)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no auto agent.update received")
	}

	if err := h.SendUpdate(); err != nil {
		t.Fatal(err)
	}
	select {
	case u := <-updates:
		if !strings.HasSuffix(u.URL, "/agent/fireproxy-agent?arch=amd64") {
			t.Fatalf("url = %q, want amd64-suffixed", u.URL)
		}
		if u.SHA256 != wantBin.SHA256 {
			t.Fatalf("sha = %s, want amd64 sha %s", u.SHA256, wantBin.SHA256)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no agent.update received")
	}
}

func TestAutoUpdateSHAMismatchSameVersion(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"fireproxy-agent-arm64": "arm-bytes",
		"fireproxy-agent-amd64": "x86-bytes",
		"VERSION":               "dev\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := agentpkg.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	wantBin, _ := pkg.Bin("arm64")

	h := &Hub{Auth: auth.StaticToken("secret"), Package: pkg, PublicBase: func() string { return "http://fp.local" }}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/agent/ws", h.ServeWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/agent/ws"
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer secret")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	updates := make(chan *agentws.AgentUpdate, 1)
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var env agentws.Envelope
			if err := json.Unmarshal(data, &env); err != nil {
				continue
			}
			if env.Type == agentws.TypeAgentUpdate {
				updates <- env.AgentUpdate
			}
		}
	}()

	hello := agentws.Envelope{Type: agentws.TypeHello, Hello: &agentws.Hello{
		Version:    "dev",
		Arch:       "arm64",
		SelfUpdate: true,
		SHA256:     "deadbeef",
	}}
	b, _ := json.Marshal(hello)
	if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatal(err)
	}

	select {
	case u := <-updates:
		if u.SHA256 != wantBin.SHA256 {
			t.Fatalf("sha = %s, want %s", u.SHA256, wantBin.SHA256)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected auto-update on sha mismatch with equal version")
	}
}

func TestAutoUpdateSkipsMatchingSHA(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"fireproxy-agent-arm64": "arm-bytes",
		"VERSION":               "dev\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := agentpkg.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	wantBin, _ := pkg.Bin("arm64")

	h := &Hub{Auth: auth.StaticToken("secret"), Package: pkg, PublicBase: func() string { return "http://fp.local" }}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/agent/ws", h.ServeWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/agent/ws"
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer secret")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	updates := make(chan *agentws.AgentUpdate, 1)
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var env agentws.Envelope
			if err := json.Unmarshal(data, &env); err != nil {
				continue
			}
			if env.Type == agentws.TypeAgentUpdate {
				updates <- env.AgentUpdate
			}
		}
	}()

	hello := agentws.Envelope{Type: agentws.TypeHello, Hello: &agentws.Hello{
		Version:    "dev",
		Arch:       "arm64",
		SelfUpdate: true,
		SHA256:     wantBin.SHA256,
	}}
	b, _ := json.Marshal(hello)
	if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatal(err)
	}

	select {
	case <-updates:
		t.Fatal("should not auto-update when sha matches")
	case <-time.After(300 * time.Millisecond):
	}
}
