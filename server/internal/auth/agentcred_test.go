package auth_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/store"
)

func TestDevAgentTokenOnlyWhenAuthDisabled(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer devtok")

	disabled := &auth.AgentCreds{Cfg: auth.Config{Disabled: true, DevAgentToken: "devtok"}}
	if !disabled.AuthorizeAgent(req) {
		t.Fatal("want accept when AUTH_DISABLED + DevAgentToken")
	}

	enabled := &auth.AgentCreds{Cfg: auth.Config{Disabled: false, DevAgentToken: "devtok"}}
	if enabled.AuthorizeAgent(req) {
		t.Fatal("want reject DevAgentToken when auth enabled")
	}
}

func TestMintAndAuthorizeAgentCredential(t *testing.T) {
	p, err := store.OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 7)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })

	plain, err := auth.MintAgentCredential(p)
	if err != nil {
		t.Fatal(err)
	}
	if plain == "" || len(plain) < 10 {
		t.Fatalf("token %q", plain)
	}

	v := &auth.AgentCreds{Persist: p, Cfg: auth.Config{}}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer "+plain)
	if !v.AuthorizeAgent(req) {
		t.Fatal("want accept minted token")
	}

	req2 := httptest.NewRequest(http.MethodPost, "/", nil)
	req2.Header.Set("X-API-Key", plain)
	if !v.AuthorizeAgent(req2) {
		t.Fatal("want accept X-API-Key")
	}

	bad := httptest.NewRequest(http.MethodPost, "/", nil)
	bad.Header.Set("Authorization", "Bearer wrong")
	if v.AuthorizeAgent(bad) {
		t.Fatal("want reject wrong token")
	}

	creds, err := p.ListAgentCredentials()
	if err != nil || len(creds) != 1 {
		t.Fatalf("creds %+v err=%v", creds, err)
	}
	if creds[0].LastUsedAt == nil || *creds[0].LastUsedAt < time.Now().Unix()-5 {
		t.Fatalf("last_used not touched: %+v", creds[0])
	}
}

func TestMintSupersedesPreviousCredential(t *testing.T) {
	p, err := store.OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 7)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })

	first, err := auth.MintAgentCredential(p)
	if err != nil {
		t.Fatal(err)
	}
	second, err := auth.MintAgentCredential(p)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("want distinct tokens")
	}

	v := &auth.AgentCreds{Persist: p}
	reqOld := httptest.NewRequest(http.MethodPost, "/", nil)
	reqOld.Header.Set("Authorization", "Bearer "+first)
	if v.AuthorizeAgent(reqOld) {
		t.Fatal("superseded token must fail")
	}
	reqNew := httptest.NewRequest(http.MethodPost, "/", nil)
	reqNew.Header.Set("Authorization", "Bearer "+second)
	if !v.AuthorizeAgent(reqNew) {
		t.Fatal("new token must pass")
	}
	creds, _ := p.ListAgentCredentials()
	if len(creds) != 1 {
		t.Fatalf("want one active cred, got %d", len(creds))
	}
}

func TestMintAgentCredentialConcurrentSingleActive(t *testing.T) {
	p, err := store.OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 7)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })

	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := auth.MintAgentCredential(p)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	creds, err := p.ListAgentCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if len(creds) != 1 {
		t.Fatalf("want exactly one agent credential after concurrent mint, got %d", len(creds))
	}
}
