package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fireproxy/server/internal/api"
	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/store"
	"fireproxy/server/internal/tplink"
)

func TestIAMAPIKeysOIDCAgentCreds(t *testing.T) {
	p, err := store.OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 7)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })

	s := &api.Server{Persist: p}
	mux := http.NewServeMux()
	s.Routes(mux)

	// Create API key — plaintext once
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/api-keys", strings.NewReader(`{"name":"ci","scopes":["read","write"]}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		ID     string   `json:"id"`
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
		Token  string   `json:"token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || !strings.HasPrefix(created.Token, "fp_") {
		t.Fatalf("created %+v", created)
	}
	if _, ok, _ := auth.LookupAPIKey(p, created.Token); !ok {
		t.Fatal("hash not stored")
	}

	// List never returns plaintext
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/auth/api-keys", nil))
	if rr.Code != 200 {
		t.Fatalf("list %d %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), created.Token) {
		t.Fatalf("list leaked token: %s", rr.Body.String())
	}
	var listed struct {
		Keys []map[string]any `json:"keys"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &listed)
	if len(listed.Keys) != 1 {
		t.Fatalf("keys %+v", listed.Keys)
	}
	if _, hasToken := listed.Keys[0]["token"]; hasToken {
		t.Fatal("list must not include token field")
	}
	if _, hasHash := listed.Keys[0]["hash"]; hasHash {
		t.Fatal("list must not include hash field")
	}

	// Revoke → lookup fails
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/v1/auth/api-keys/"+created.ID, nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("revoke %d %s", rr.Code, rr.Body.String())
	}
	if _, ok, _ := auth.LookupAPIKey(p, created.Token); ok {
		t.Fatal("revoked key still looks up")
	}

	// OIDC GET never includes secret plaintext
	hexKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	t.Setenv("FIREPROXY_SECRETS_KEY", hexKey)
	key, err := tplink.KeyFromEnv(os.Getenv("FIREPROXY_SECRETS_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	secretPlain := "super-oidc-secret"
	enc, err := tplink.Encrypt(key, secretPlain)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.PutOIDCSettings(store.OIDCSettings{
		Issuer:           "https://idp.example",
		ClientID:         "cid",
		RedirectURI:      "https://app.example/cb",
		Allowlist:        []string{"a@b.co"},
		SecretCiphertext: []byte(enc),
	}); err != nil {
		t.Fatal(err)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/auth/oidc", nil))
	if rr.Code != 200 {
		t.Fatalf("oidc get %d %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, secretPlain) || strings.Contains(body, enc) {
		t.Fatalf("oidc leaked secret: %s", body)
	}
	var oidc map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &oidc)
	if oidc["secret_set"] != true {
		t.Fatalf("secret_set %+v", oidc)
	}
	if _, ok := oidc["client_secret"]; ok {
		t.Fatal("must not include client_secret")
	}
	if _, ok := oidc["secret_ciphertext"]; ok {
		t.Fatal("must not include secret_ciphertext")
	}

	// PUT OIDC without secret keeps existing; GET still safe; revokes all sessions
	sid, err := auth.CreateSession(p, auth.Config{SessionAbs: time.Hour, SessionIdle: time.Hour}, "password")
	if err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/v1/auth/oidc", strings.NewReader(`{
		"issuer":"https://idp.example","client_id":"cid2","redirect_uri":"https://app.example/cb","allowlist":["a@b.co"]
	}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("oidc put %d %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), secretPlain) {
		t.Fatal("put response leaked secret")
	}
	if _, ok, _ := p.GetSession(sid); ok {
		t.Fatal("OIDC PUT should DeleteAllSessions")
	}

	// Agent credentials list + revoke
	token, err := auth.MintAgentCredential(p)
	if err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/auth/agent-credentials", nil))
	if rr.Code != 200 {
		t.Fatalf("agent list %d %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), token) {
		t.Fatal("agent list leaked token")
	}
	var creds struct {
		Credentials []struct {
			ID string `json:"id"`
		} `json:"credentials"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &creds)
	if len(creds.Credentials) != 1 {
		t.Fatalf("creds %+v", creds)
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/v1/auth/agent-credentials/"+creds.Credentials[0].ID, nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("agent revoke one %d", rr.Code)
	}
	agentRows, _ := p.ListAgentCredentials()
	if len(agentRows) != 0 {
		t.Fatalf("want empty after revoke one, got %d", len(agentRows))
	}

	_, _ = auth.MintAgentCredential(p)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/v1/auth/agent-credentials", nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("agent revoke all %d", rr.Code)
	}
	agentRows, _ = p.ListAgentCredentials()
	if len(agentRows) != 0 {
		t.Fatalf("want empty after revoke all")
	}
}
