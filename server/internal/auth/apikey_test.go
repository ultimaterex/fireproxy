package auth_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/store"
)

func TestMintListRevokeAPIKey(t *testing.T) {
	p, err := store.OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 7)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })

	plain, info, err := auth.MintAPIKey(p, "ci", []auth.Scope{auth.ScopeRead, auth.ScopeWrite})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(plain, "fp_") {
		t.Fatalf("plaintext prefix: %q", plain)
	}
	if info.ID == "" || info.Name != "ci" {
		t.Fatalf("info %+v", info)
	}
	if !auth.HasScope(info.Scopes, auth.ScopeRead) || !auth.HasScope(info.Scopes, auth.ScopeWrite) {
		t.Fatalf("scopes %+v", info.Scopes)
	}

	found, ok, err := auth.LookupAPIKey(p, plain)
	if err != nil || !ok {
		t.Fatalf("lookup ok=%v err=%v", ok, err)
	}
	if found.Hash != auth.HashToken(plain) {
		t.Fatalf("stored hash mismatch")
	}
	if found.Hash == plain || strings.Contains(found.Hash, plain) {
		t.Fatal("hash must not equal plaintext")
	}

	listed, err := auth.ListAPIKeys(p)
	if err != nil || len(listed) != 1 {
		t.Fatalf("list %+v err=%v", listed, err)
	}
	raw, err := json.Marshal(listed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), plain) {
		t.Fatalf("list leaked plaintext: %s", raw)
	}
	if listed[0].ID != info.ID || listed[0].Name != "ci" {
		t.Fatalf("list entry %+v", listed[0])
	}

	if err := auth.RevokeAPIKey(p, info.ID); err != nil {
		t.Fatal(err)
	}
	_, ok, err = auth.LookupAPIKey(p, plain)
	if err != nil || ok {
		t.Fatalf("after revoke want miss ok=%v err=%v", ok, err)
	}
	listed, err = auth.ListAPIKeys(p)
	if err != nil || len(listed) != 0 {
		t.Fatalf("after revoke list %+v err=%v", listed, err)
	}
}

func TestAdminScopeImpliesWriteRead(t *testing.T) {
	have := []auth.Scope{auth.ScopeAdmin}
	if !auth.HasScope(have, auth.ScopeWrite) || !auth.HasScope(have, auth.ScopeRead) {
		t.Fatal("admin must imply write and read")
	}
}
