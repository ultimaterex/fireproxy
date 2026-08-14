package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/store"
)

func openAuthPersist(t *testing.T) *store.Persist {
	t.Helper()
	p, err := store.OpenPersist(filepath.Join(t.TempDir(), "auth.db"), 7)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func TestLoginSetsHttpOnlySameSiteCookie(t *testing.T) {
	p := openAuthPersist(t)
	cfg := auth.Config{
		Password:    "s3cret",
		SessionAbs:  7 * 24 * time.Hour,
		SessionIdle: 24 * time.Hour,
	}
	h := &auth.Handlers{Persist: p, Cfg: cfg}
	mux := http.NewServeMux()
	h.Register(mux)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"username":"admin","password":"s3cret"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login status %d %s", rr.Code, rr.Body.String())
	}
	sc := rr.Header().Get("Set-Cookie")
	if sc == "" {
		t.Fatal("missing Set-Cookie")
	}
	if !strings.Contains(sc, "fp_session=") {
		t.Fatalf("cookie name: %s", sc)
	}
	if !strings.Contains(sc, "HttpOnly") {
		t.Fatalf("want HttpOnly: %s", sc)
	}
	if !strings.Contains(sc, "SameSite=Lax") {
		t.Fatalf("want SameSite=Lax: %s", sc)
	}
	c := rr.Result().Cookies()
	if len(c) == 0 || c[0].Value == "" {
		t.Fatal("empty session cookie value")
	}
	sess, ok, err := p.GetSession(c[0].Value)
	if err != nil || !ok {
		t.Fatalf("session row: ok=%v err=%v", ok, err)
	}
	if sess.AuthMethod != "password" {
		t.Fatalf("auth_method=%q", sess.AuthMethod)
	}
}

func TestLoginWrongPassword401(t *testing.T) {
	p := openAuthPersist(t)
	h := &auth.Handlers{Persist: p, Cfg: auth.Config{Password: "s3cret", SessionAbs: time.Hour, SessionIdle: time.Hour}}
	mux := http.NewServeMux()
	h.Register(mux)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"username":"admin","password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rr.Code)
	}
	if rr.Header().Get("Set-Cookie") != "" {
		t.Fatal("should not set cookie on failed login")
	}
}

func TestLogoutClearsSession(t *testing.T) {
	p := openAuthPersist(t)
	cfg := auth.Config{Password: "s3cret", SessionAbs: time.Hour, SessionIdle: time.Hour}
	h := &auth.Handlers{Persist: p, Cfg: cfg}
	mux := http.NewServeMux()
	h.Register(mux)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"username":"admin","password":"s3cret"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookie")
	}
	sid := cookies[0].Value

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sid})
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("logout %d", rr.Code)
	}
	sc := rr.Header().Get("Set-Cookie")
	if !strings.Contains(sc, "fp_session=") || !strings.Contains(sc, "Max-Age=0") && !strings.Contains(strings.ToLower(sc), "max-age=-1") {
		// Go encodes MaxAge < 0 as Max-Age=0
		if !strings.Contains(sc, "Max-Age=0") {
			t.Fatalf("want cleared cookie: %s", sc)
		}
	}
	_, ok, err := p.GetSession(sid)
	if err != nil || ok {
		t.Fatalf("session should be deleted ok=%v err=%v", ok, err)
	}
}

func TestHealthzOK(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", auth.Healthz)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	if body := strings.TrimSpace(rr.Body.String()); body != "ok" {
		t.Fatalf("body %q", body)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("content-type %q", ct)
	}
}

func TestLoginOptionsPublic(t *testing.T) {
	h := &auth.Handlers{Cfg: auth.Config{Password: "x"}}
	mux := http.NewServeMux()
	h.Register(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/auth/login-options", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["oidc_enabled"] != false {
		t.Fatalf("%#v", out)
	}
}

func TestMeUnauthenticated401(t *testing.T) {
	p := openAuthPersist(t)
	h := &auth.Handlers{Persist: p, Cfg: auth.Config{Password: "x"}}
	mux := http.NewServeMux()
	h.Register(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestMeWhenAuthDisabled(t *testing.T) {
	h := &auth.Handlers{Cfg: auth.Config{Disabled: true}}
	mux := http.NewServeMux()
	h.Register(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if out["authenticated"] != true {
		t.Fatalf("%#v", out)
	}
}
