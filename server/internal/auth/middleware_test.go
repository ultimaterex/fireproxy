package auth_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/store"
)

func middlewarePersist(t *testing.T) *store.Persist {
	t.Helper()
	p, err := store.OpenPersist(filepath.Join(t.TempDir(), "mw.db"), 7)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func wrapMux(t *testing.T, cfg auth.Config, p *store.Persist, agents auth.AgentVerifier) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("POST /v1/agent/enroll/code", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":"x"}`))
	})
	mux.HandleFunc("POST /v1/ingest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("GET /v1/auth/login-options", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"oidc_enabled":false}`))
	})
	mux.HandleFunc("POST /v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mw := &auth.Middleware{
		Cfg:     cfg,
		Persist: p,
		Agents:  agents,
	}
	return mw.Handler(mux)
}

func mintKey(t *testing.T, p *store.Persist, name string, scopes []auth.Scope) string {
	t.Helper()
	plain, _, err := auth.MintAPIKey(p, name, scopes)
	if err != nil {
		t.Fatal(err)
	}
	return plain
}

func mintSession(t *testing.T, p *store.Persist, cfg auth.Config) string {
	t.Helper()
	id, err := auth.CreateSession(p, cfg, "password")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestMiddlewareUnauthHealth401(t *testing.T) {
	p := middlewarePersist(t)
	cfg := auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour}
	h := wrapMux(t, cfg, p, &auth.AgentCreds{Persist: p, Cfg: cfg})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d want 401", rr.Code)
	}
}

func TestMiddlewareReadWriteCannotMintEnrollCodeAdminCan(t *testing.T) {
	p := middlewarePersist(t)
	cfg := auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour}
	h := wrapMux(t, cfg, p, &auth.AgentCreds{Persist: p, Cfg: cfg})

	readKey := mintKey(t, p, "read", []auth.Scope{auth.ScopeRead})
	writeKey := mintKey(t, p, "write", []auth.Scope{auth.ScopeWrite})
	adminKey := mintKey(t, p, "admin", []auth.Scope{auth.ScopeAdmin})

	for _, tc := range []struct {
		name string
		key  string
		want int
	}{
		{"read", readKey, http.StatusForbidden},
		{"write", writeKey, http.StatusForbidden},
		{"admin", adminKey, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/agent/enroll/code", strings.NewReader(`{}`))
			req.Header.Set("Authorization", "Bearer "+tc.key)
			req.Header.Set("Content-Type", "application/json")
			h.ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("status %d want %d body=%s", rr.Code, tc.want, rr.Body.String())
			}
		})
	}
}

func TestMiddlewareSessionCSRFWithoutOrigin403(t *testing.T) {
	p := middlewarePersist(t)
	cfg := auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour}
	h := wrapMux(t, cfg, p, &auth.AgentCreds{Persist: p, Cfg: cfg})
	sid := mintSession(t, p, cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/enroll/code", strings.NewReader(`{}`))
	req.Host = "fireproxy.local"
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sid})
	req.Header.Set("Content-Type", "application/json")
	// no Origin / Referer
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status %d want 403", rr.Code)
	}
}

func TestMiddlewareSessionSameOriginPOSTAllowed(t *testing.T) {
	p := middlewarePersist(t)
	cfg := auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour}
	h := wrapMux(t, cfg, p, &auth.AgentCreds{Persist: p, Cfg: cfg})
	sid := mintSession(t, p, cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/enroll/code", strings.NewReader(`{}`))
	req.Host = "fireproxy.local"
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sid})
	req.Header.Set("Origin", "http://fireproxy.local")
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d want 200 body=%s", rr.Code, rr.Body.String())
	}
}

// nginx $host strips the port; Origin keeps it. CSRF must still allow the POST.
func TestMiddlewareSessionCSRFOriginPortVsHostWithoutPort(t *testing.T) {
	p := middlewarePersist(t)
	cfg := auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour}
	h := wrapMux(t, cfg, p, &auth.AgentCreds{Persist: p, Cfg: cfg})
	sid := mintSession(t, p, cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/enroll/code", strings.NewReader(`{}`))
	req.Host = "localhost" // as forwarded by nginx $host
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sid})
	req.Header.Set("Origin", "http://localhost:3080")
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d want 200 body=%s", rr.Code, rr.Body.String())
	}
}

func TestMiddlewareAgentTokenOnHealth401OnIngestOK(t *testing.T) {
	p := middlewarePersist(t)
	cfg := auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour}
	token, err := auth.MintAgentCredential(p)
	if err != nil {
		t.Fatal(err)
	}
	h := wrapMux(t, cfg, p, &auth.AgentCreds{Persist: p, Cfg: cfg})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("health status %d want 401", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/ingest", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("ingest status %d want 200", rr.Code)
	}
}

func TestMiddlewareSessionOnIngest401(t *testing.T) {
	p := middlewarePersist(t)
	cfg := auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour}
	h := wrapMux(t, cfg, p, &auth.AgentCreds{Persist: p, Cfg: cfg})
	sid := mintSession(t, p, cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest", strings.NewReader(`{}`))
	req.Host = "fireproxy.local"
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sid})
	req.Header.Set("Origin", "http://fireproxy.local")
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d want 401", rr.Code)
	}
}

func TestMiddlewareDevAgentTokenIgnoredWhenAuthOn(t *testing.T) {
	p := middlewarePersist(t)
	cfg := auth.Config{
		Password:      "x",
		DevAgentToken: "devtok",
		SessionAbs:    time.Hour,
		SessionIdle:   time.Hour,
	}
	// AgentCreds must ignore DevAgentToken when auth enabled (Cfg.Disabled=false).
	agents := &auth.AgentCreds{Persist: p, Cfg: cfg}
	h := wrapMux(t, cfg, p, agents)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer devtok")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d want 401", rr.Code)
	}
}

func TestMiddlewareLoginOptionsPublicWhenAuthOn(t *testing.T) {
	p := middlewarePersist(t)
	cfg := auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour}
	h := wrapMux(t, cfg, p, &auth.AgentCreds{Persist: p, Cfg: cfg})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/login-options", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d want 200", rr.Code)
	}
}

func TestMiddlewareAuthDisabledSyntheticAdmin(t *testing.T) {
	p := middlewarePersist(t)
	cfg := auth.Config{Disabled: true, SessionAbs: time.Hour, SessionIdle: time.Hour}
	h := wrapMux(t, cfg, p, &auth.AgentCreds{Persist: p, Cfg: cfg})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d want 200", rr.Code)
	}
}

func TestMiddlewareCORSNoStarWhenAuthOn(t *testing.T) {
	p := middlewarePersist(t)
	cfg := auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour}
	h := wrapMux(t, cfg, p, &auth.AgentCreds{Persist: p, Cfg: cfg})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/login-options", nil)
	h.ServeHTTP(rr, req)
	if acao := rr.Header().Get("Access-Control-Allow-Origin"); acao == "*" {
		t.Fatal("must not set ACAO=* when auth enabled")
	}
}
