package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAuthSessionsAPIKeysAgentOIDC(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "auth.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	now := time.Now().Unix()
	sess := AuthSession{
		ID:           "sess-1",
		CreatedAt:    now,
		ExpiresAt:    now + 7*24*3600,
		IdleDeadline: now + 24*3600,
		AuthMethod:   "password",
	}
	if err := p.InsertSession(sess); err != nil {
		t.Fatal(err)
	}
	got, ok, err := p.GetSession("sess-1")
	if err != nil || !ok {
		t.Fatalf("GetSession: ok=%v err=%v", ok, err)
	}
	if got.ID != sess.ID || got.AuthMethod != "password" || got.ExpiresAt != sess.ExpiresAt {
		t.Fatalf("session %+v", got)
	}
	newIdle := now + 48*3600
	if err := p.TouchSession("sess-1", newIdle); err != nil {
		t.Fatal(err)
	}
	got, ok, err = p.GetSession("sess-1")
	if err != nil || !ok || got.IdleDeadline != newIdle {
		t.Fatalf("touch idle=%d ok=%v err=%v", got.IdleDeadline, ok, err)
	}
	if err := p.DeleteSession("sess-1"); err != nil {
		t.Fatal(err)
	}
	_, ok, err = p.GetSession("sess-1")
	if err != nil || ok {
		t.Fatalf("deleted session still present ok=%v err=%v", ok, err)
	}
	_ = p.InsertSession(sess)
	_ = p.InsertSession(AuthSession{
		ID: "sess-2", CreatedAt: now, ExpiresAt: now + 1, IdleDeadline: now + 1, AuthMethod: "oidc",
	})
	if err := p.DeleteAllSessions(); err != nil {
		t.Fatal(err)
	}
	_, ok, _ = p.GetSession("sess-2")
	if ok {
		t.Fatal("DeleteAllSessions left rows")
	}

	key := AuthAPIKey{
		ID:        "key-1",
		Name:      "ci",
		Hash:      "hash-abc",
		Scopes:    "read,write",
		CreatedAt: now,
	}
	if err := p.InsertAPIKey(key); err != nil {
		t.Fatal(err)
	}
	keys, err := p.ListAPIKeys()
	if err != nil || len(keys) != 1 || keys[0].Name != "ci" || keys[0].Scopes != "read,write" {
		t.Fatalf("ListAPIKeys %+v err=%v", keys, err)
	}
	found, ok, err := p.GetAPIKeyByHash("hash-abc")
	if err != nil || !ok || found.ID != "key-1" {
		t.Fatalf("GetAPIKeyByHash %+v ok=%v err=%v", found, ok, err)
	}
	usedAt := now + 10
	if err := p.TouchAPIKeyLastUsed("key-1", usedAt); err != nil {
		t.Fatal(err)
	}
	found, ok, _ = p.GetAPIKeyByHash("hash-abc")
	if !ok || found.LastUsedAt == nil || *found.LastUsedAt != usedAt {
		t.Fatalf("last_used %+v", found)
	}
	if err := p.DeleteAPIKey("key-1"); err != nil {
		t.Fatal(err)
	}
	keys, err = p.ListAPIKeys()
	if err != nil || len(keys) != 0 {
		t.Fatalf("after delete %+v err=%v", keys, err)
	}

	cred := AuthAgentCredential{
		ID:        "agent-1",
		Hash:      "agent-hash",
		CreatedAt: now,
	}
	if err := p.InsertAgentCredential(cred); err != nil {
		t.Fatal(err)
	}
	creds, err := p.ListAgentCredentials()
	if err != nil || len(creds) != 1 || creds[0].Hash != "agent-hash" {
		t.Fatalf("ListAgentCredentials %+v err=%v", creds, err)
	}
	ac, ok, err := p.LookupAgentCredentialByHash("agent-hash")
	if err != nil || !ok || ac.ID != "agent-1" {
		t.Fatalf("LookupAgentCredentialByHash %+v ok=%v err=%v", ac, ok, err)
	}
	agentUsed := now + 20
	if err := p.TouchAgentCredentialLastUsed("agent-1", agentUsed); err != nil {
		t.Fatal(err)
	}
	ac, ok, _ = p.LookupAgentCredentialByHash("agent-hash")
	if !ok || ac.LastUsedAt == nil || *ac.LastUsedAt != agentUsed {
		t.Fatalf("agent last_used %+v", ac)
	}
	if err := p.DeleteAllAgentCredentials(); err != nil {
		t.Fatal(err)
	}
	creds, err = p.ListAgentCredentials()
	if err != nil || len(creds) != 0 {
		t.Fatalf("after DeleteAllAgentCredentials %+v err=%v", creds, err)
	}

	if err := p.InsertAgentCredential(AuthAgentCredential{ID: "old", Hash: "h1", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := p.ReplaceAgentCredential(AuthAgentCredential{ID: "new", Hash: "h2", CreatedAt: now + 1}); err != nil {
		t.Fatal(err)
	}
	creds, err = p.ListAgentCredentials()
	if err != nil || len(creds) != 1 || creds[0].ID != "new" || creds[0].Hash != "h2" {
		t.Fatalf("after ReplaceAgentCredential %+v err=%v", creds, err)
	}

	settings := OIDCSettings{
		Issuer:           "https://idp.example/oidc",
		ClientID:         "fireproxy",
		RedirectURI:      "https://fp.example/v1/auth/oidc/callback",
		Allowlist:        []string{"admin@example.com", "sub:abc"},
		SecretCiphertext: []byte("cipher-bytes"),
	}
	if err := p.PutOIDCSettings(settings); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := p.GetOIDCSettings()
	if err != nil || !ok {
		t.Fatalf("GetOIDCSettings ok=%v err=%v", ok, err)
	}
	if loaded.Issuer != settings.Issuer || loaded.ClientID != settings.ClientID ||
		loaded.RedirectURI != settings.RedirectURI || string(loaded.SecretCiphertext) != "cipher-bytes" {
		t.Fatalf("oidc %+v", loaded)
	}
	if len(loaded.Allowlist) != 2 || loaded.Allowlist[0] != "admin@example.com" {
		t.Fatalf("allowlist %+v", loaded.Allowlist)
	}
}

func TestGetAPIKeyByID(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "auth.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	_, ok, err := p.GetAPIKeyByID("missing")
	if err != nil || ok {
		t.Fatalf("miss: ok=%v err=%v", ok, err)
	}

	now := time.Now().Unix()
	key := AuthAPIKey{
		ID:        "key-1",
		Name:      "ci",
		Hash:      "hash-abc",
		Scopes:    "read,write",
		CreatedAt: now,
	}
	if err := p.InsertAPIKey(key); err != nil {
		t.Fatal(err)
	}
	found, ok, err := p.GetAPIKeyByID("key-1")
	if err != nil || !ok || found.ID != "key-1" || found.Name != "ci" ||
		found.Hash != "hash-abc" || found.Scopes != "read,write" || found.CreatedAt != now {
		t.Fatalf("GetAPIKeyByID %+v ok=%v err=%v", found, ok, err)
	}
	usedAt := now + 10
	if err := p.TouchAPIKeyLastUsed("key-1", usedAt); err != nil {
		t.Fatal(err)
	}
	found, ok, _ = p.GetAPIKeyByID("key-1")
	if !ok || found.LastUsedAt == nil || *found.LastUsedAt != usedAt {
		t.Fatalf("last_used %+v", found)
	}
}

func TestAuthPutOIDCSettingsNilSecretAtomic(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "auth-oidc.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	prior := OIDCSettings{
		Issuer:           "https://old.example/oidc",
		ClientID:         "old-client",
		RedirectURI:      "https://old.example/callback",
		Allowlist:        []string{"old@example.com"},
		SecretCiphertext: []byte("old-secret"),
	}
	if err := p.PutOIDCSettings(prior); err != nil {
		t.Fatal(err)
	}

	// Nil SecretCiphertext must not fail kv NOT NULL mid-update.
	next := OIDCSettings{
		Issuer:           "https://new.example/oidc",
		ClientID:         "new-client",
		RedirectURI:      "https://new.example/callback",
		Allowlist:        []string{"new@example.com"},
		SecretCiphertext: nil,
	}
	if err := p.PutOIDCSettings(next); err != nil {
		t.Fatalf("PutOIDCSettings nil secret: %v", err)
	}
	loaded, ok, err := p.GetOIDCSettings()
	if err != nil || !ok {
		t.Fatalf("GetOIDCSettings ok=%v err=%v", ok, err)
	}
	if loaded.Issuer != next.Issuer || loaded.ClientID != next.ClientID ||
		loaded.RedirectURI != next.RedirectURI {
		t.Fatalf("partial or wrong oidc %+v", loaded)
	}
	if len(loaded.Allowlist) != 1 || loaded.Allowlist[0] != "new@example.com" {
		t.Fatalf("allowlist %+v", loaded.Allowlist)
	}
	if len(loaded.SecretCiphertext) != 0 {
		t.Fatalf("secret want empty, got %#v", loaded.SecretCiphertext)
	}
	// All five keys present together (no mid-update partial config).
	for _, k := range []string{
		kvOIDCIssuer, kvOIDCClientID, kvOIDCRedirectURI, kvOIDCAllowlist, kvOIDCSecretCiphertext,
	} {
		if _, ok, err := p.GetKV(k); err != nil || !ok {
			t.Fatalf("missing kv %s ok=%v err=%v", k, ok, err)
		}
	}
}
