package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/store"
	"fireproxy/server/internal/tplink"
)

const testSecretsKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestOIDCEnabled(t *testing.T) {
	if auth.OIDCEnabled(store.OIDCSettings{Issuer: "https://idp", ClientID: "c", Allowlist: nil}) {
		t.Fatal("empty allowlist should disable")
	}
	if auth.OIDCEnabled(store.OIDCSettings{Issuer: "https://idp", ClientID: "c", Allowlist: []string{"", "  "}}) {
		t.Fatal("blank-only allowlist should disable")
	}
	if auth.OIDCEnabled(store.OIDCSettings{Issuer: "", ClientID: "c", Allowlist: []string{"a@b.co"}}) {
		t.Fatal("missing issuer")
	}
	if auth.OIDCEnabled(store.OIDCSettings{Issuer: "https://idp", ClientID: "", Allowlist: []string{"a@b.co"}}) {
		t.Fatal("missing client")
	}
	if !auth.OIDCEnabled(store.OIDCSettings{Issuer: "https://idp", ClientID: "c", Allowlist: []string{"a@b.co"}}) {
		t.Fatal("want enabled")
	}
}

func TestAllowlistAllows(t *testing.T) {
	allow := []string{"admin@example.com", "sub:user-1", "email:Other@Example.com"}
	cases := []struct {
		email, sub string
		want       bool
	}{
		{"admin@example.com", "", true},
		{"ADMIN@example.com", "", true},
		{"", "user-1", true},
		{"other@example.com", "", true},
		{"nope@example.com", "other", false},
		{"", "", false},
	}
	for _, tc := range cases {
		if got := auth.AllowlistAllows(allow, tc.email, tc.sub); got != tc.want {
			t.Fatalf("email=%q sub=%q got=%v want=%v", tc.email, tc.sub, got, tc.want)
		}
	}
	// Bare entry can match subject too.
	if !auth.AllowlistAllows([]string{"abc-sub"}, "", "abc-sub") {
		t.Fatal("bare sub match")
	}
}

func putOIDCWithSecret(t *testing.T, p *store.Persist, allow []string) {
	t.Helper()
	t.Setenv("FIREPROXY_SECRETS_KEY", testSecretsKey)
	key, err := tplink.KeyFromEnv(os.Getenv("FIREPROXY_SECRETS_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	enc, err := tplink.Encrypt(key, "client-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.PutOIDCSettings(store.OIDCSettings{
		Issuer:           "https://idp.example",
		ClientID:         "cid",
		RedirectURI:      "https://app.example/v1/auth/oidc/callback",
		Allowlist:        allow,
		SecretCiphertext: []byte(enc),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestOIDCStartEmptyAllowlist400(t *testing.T) {
	p := openAuthPersist(t)
	putOIDCWithSecret(t, p, []string{})
	h := &auth.Handlers{
		Persist: p,
		Cfg:     auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour},
		OIDCAuthURL: func(ctx context.Context, settings store.OIDCSettings, clientSecret, state, nonce, codeVerifier string) (string, error) {
			t.Fatal("should not build auth URL when allowlist empty")
			return "", nil
		},
	}
	mux := http.NewServeMux()
	h.Register(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/auth/oidc/start", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}

func TestOIDCStartRedirectsWithPKCEHooks(t *testing.T) {
	p := openAuthPersist(t)
	putOIDCWithSecret(t, p, []string{"admin@example.com"})
	var sawState, sawNonce, sawVerifier string
	h := &auth.Handlers{
		Persist: p,
		Cfg:     auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour},
		OIDCAuthURL: func(ctx context.Context, settings store.OIDCSettings, clientSecret, state, nonce, codeVerifier string) (string, error) {
			if clientSecret != "client-secret" {
				t.Fatalf("secret %q", clientSecret)
			}
			sawState, sawNonce, sawVerifier = state, nonce, codeVerifier
			return "https://idp.example/auth?state=" + state, nil
		},
	}
	mux := http.NewServeMux()
	h.Register(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/auth/oidc/start", nil))
	if rr.Code != http.StatusFound {
		t.Fatalf("status %d %s", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	if !strings.Contains(loc, "https://idp.example/auth") {
		t.Fatalf("location %s", loc)
	}
	if sawState == "" || sawNonce == "" || sawVerifier == "" {
		t.Fatal("missing pkce/state/nonce")
	}
	// Pending consumed only on callback — still present after start.
	pending, ok, err := p.TakeOIDCPending(sawState)
	if err != nil || !ok {
		t.Fatalf("pending ok=%v err=%v", ok, err)
	}
	if pending.Nonce != sawNonce || pending.CodeVerifier != sawVerifier {
		t.Fatalf("pending %+v", pending)
	}
}

func TestOIDCCallbackBadState(t *testing.T) {
	p := openAuthPersist(t)
	putOIDCWithSecret(t, p, []string{"admin@example.com"})
	h := &auth.Handlers{Persist: p, Cfg: auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour}}
	mux := http.NewServeMux()
	h.Register(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/auth/oidc/callback?state=nope&code=abc", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "invalid state") {
		t.Fatalf("body %s", rr.Body.String())
	}
}

func TestOIDCCallbackBadNonce(t *testing.T) {
	p := openAuthPersist(t)
	putOIDCWithSecret(t, p, []string{"admin@example.com"})
	state := "state-1"
	if err := p.PutOIDCPending(store.OIDCPending{
		State: state, Nonce: "nonce-expected", CodeVerifier: "verifier", ExpiresAt: time.Now().Add(time.Minute).Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	h := &auth.Handlers{
		Persist: p,
		Cfg:     auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour},
		OIDCExchange: func(ctx context.Context, code, codeVerifier string, settings store.OIDCSettings, clientSecret string) (auth.OIDCIdentity, error) {
			return auth.OIDCIdentity{Subject: "sub1", Email: "admin@example.com", Nonce: "nonce-WRONG"}, nil
		},
	}
	mux := http.NewServeMux()
	h.Register(mux)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/oidc/callback?state="+state+"&code=abc", nil)
	req.AddCookie(&http.Cookie{Name: "fp_oidc_state", Value: state})
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "invalid nonce") {
		t.Fatalf("body %s", rr.Body.String())
	}
	_, ok, _ := p.GetSession("anything")
	_ = ok
	// No session cookie
	if sc := rr.Header().Get("Set-Cookie"); strings.Contains(sc, "fp_session=") && !strings.Contains(sc, "Max-Age=0") {
		t.Fatalf("should not set session: %s", sc)
	}
}

func TestOIDCCallbackAllowlistReject(t *testing.T) {
	p := openAuthPersist(t)
	putOIDCWithSecret(t, p, []string{"admin@example.com"})
	state := "state-2"
	if err := p.PutOIDCPending(store.OIDCPending{
		State: state, Nonce: "n1", CodeVerifier: "v1", ExpiresAt: time.Now().Add(time.Minute).Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	h := &auth.Handlers{
		Persist: p,
		Cfg:     auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour},
		OIDCExchange: func(ctx context.Context, code, codeVerifier string, settings store.OIDCSettings, clientSecret string) (auth.OIDCIdentity, error) {
			return auth.OIDCIdentity{Subject: "stranger", Email: "evil@example.com", Nonce: "n1"}, nil
		},
	}
	mux := http.NewServeMux()
	h.Register(mux)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/oidc/callback?state="+state+"&code=abc", nil)
	req.AddCookie(&http.Cookie{Name: "fp_oidc_state", Value: state})
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status %d %s", rr.Code, rr.Body.String())
	}
}

func TestOIDCCallbackSuccessCreatesSession(t *testing.T) {
	p := openAuthPersist(t)
	putOIDCWithSecret(t, p, []string{"admin@example.com"})
	state := "state-ok"
	if err := p.PutOIDCPending(store.OIDCPending{
		State: state, Nonce: "n-ok", CodeVerifier: "v-ok", ExpiresAt: time.Now().Add(time.Minute).Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	h := &auth.Handlers{
		Persist: p,
		Cfg:     auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour},
		OIDCExchange: func(ctx context.Context, code, codeVerifier string, settings store.OIDCSettings, clientSecret string) (auth.OIDCIdentity, error) {
			if code != "good-code" || codeVerifier != "v-ok" {
				t.Fatalf("code=%q verifier=%q", code, codeVerifier)
			}
			return auth.OIDCIdentity{Subject: "sub-ok", Email: "admin@example.com", Nonce: "n-ok"}, nil
		},
	}
	mux := http.NewServeMux()
	h.Register(mux)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/oidc/callback?state="+state+"&code=good-code", nil)
	req.AddCookie(&http.Cookie{Name: "fp_oidc_state", Value: state})
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status %d %s", rr.Code, rr.Body.String())
	}
	var sessID string
	for _, c := range rr.Result().Cookies() {
		if c.Name == "fp_session" {
			sessID = c.Value
		}
	}
	if sessID == "" {
		t.Fatal("no session cookie")
	}
	sess, ok, err := p.GetSession(sessID)
	if err != nil || !ok {
		t.Fatalf("session ok=%v err=%v", ok, err)
	}
	if sess.AuthMethod != "oidc" {
		t.Fatalf("method %q", sess.AuthMethod)
	}
	// State one-time use
	_, ok, _ = p.TakeOIDCPending(state)
	if ok {
		t.Fatal("pending should be consumed")
	}
}

func TestOIDCCallbackRequiresStateCookie(t *testing.T) {
	p := openAuthPersist(t)
	putOIDCWithSecret(t, p, []string{"admin@example.com"})
	state := "state-csrf"
	if err := p.PutOIDCPending(store.OIDCPending{
		State: state, Nonce: "n", CodeVerifier: "v", ExpiresAt: time.Now().Add(time.Minute).Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	h := &auth.Handlers{
		Persist: p,
		Cfg:     auth.Config{Password: "x", SessionAbs: time.Hour, SessionIdle: time.Hour},
		OIDCExchange: func(ctx context.Context, code, codeVerifier string, settings store.OIDCSettings, clientSecret string) (auth.OIDCIdentity, error) {
			t.Fatal("exchange must not run without a matching state cookie")
			return auth.OIDCIdentity{}, nil
		},
	}
	mux := http.NewServeMux()
	h.Register(mux)

	// No cookie at all → rejected before any token exchange.
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/auth/oidc/callback?state="+state+"&code=abc", nil))
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "invalid state") {
		t.Fatalf("no-cookie: status %d body %s", rr.Code, rr.Body.String())
	}

	// Mismatched cookie (attacker's own flow) → rejected, pending untouched.
	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/oidc/callback?state="+state+"&code=abc", nil)
	req.AddCookie(&http.Cookie{Name: "fp_oidc_state", Value: "attacker-state"})
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad-cookie: status %d body %s", rr.Code, rr.Body.String())
	}
	if _, ok, _ := p.TakeOIDCPending(state); !ok {
		t.Fatal("pending should not be consumed on state-cookie mismatch")
	}
}

func TestLoginOptionsOIDCEnabled(t *testing.T) {
	p := openAuthPersist(t)
	h := &auth.Handlers{Persist: p, Cfg: auth.Config{Password: "x"}}
	mux := http.NewServeMux()
	h.Register(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/auth/login-options", nil))
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if out["oidc_enabled"] != false {
		t.Fatalf("want false without settings: %#v", out)
	}

	putOIDCWithSecret(t, p, []string{"a@b.co"})
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/auth/login-options", nil))
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if out["oidc_enabled"] != true {
		t.Fatalf("want true: %#v", out)
	}

	putOIDCWithSecret(t, p, []string{})
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/auth/login-options", nil))
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if out["oidc_enabled"] != false {
		t.Fatalf("empty allowlist disables: %#v", out)
	}
}

func TestOIDCPendingStoreRoundTrip(t *testing.T) {
	p := openAuthPersist(t)
	row := store.OIDCPending{
		State: "s1", Nonce: "n1", CodeVerifier: "v1", ExpiresAt: time.Now().Add(time.Minute).Unix(),
	}
	if err := p.PutOIDCPending(row); err != nil {
		t.Fatal(err)
	}
	got, ok, err := p.TakeOIDCPending("s1")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if got.Nonce != "n1" || got.CodeVerifier != "v1" {
		t.Fatalf("%+v", got)
	}
	_, ok, _ = p.TakeOIDCPending("s1")
	if ok {
		t.Fatal("second take should miss")
	}
}
