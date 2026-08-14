package auth_test

import (
	"testing"
	"time"

	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/store"
)

func TestVerifyPassword(t *testing.T) {
	cfg := auth.Config{Password: "hunter2"}
	if !auth.VerifyPassword(cfg, "hunter2") {
		t.Fatal("expected match")
	}
	if auth.VerifyPassword(cfg, "Hunter2") {
		t.Fatal("expected mismatch")
	}
	if auth.VerifyPassword(cfg, "") {
		t.Fatal("empty should fail")
	}
}

func TestPasswordFingerprintChangeRevokesSessions(t *testing.T) {
	p := openAuthPersist(t)
	now := time.Now().Unix()
	if err := p.InsertSession(store.AuthSession{
		ID: "sess-old", CreatedAt: now, ExpiresAt: now + 3600, IdleDeadline: now + 3600, AuthMethod: "password",
	}); err != nil {
		t.Fatal(err)
	}

	if err := auth.SyncPasswordFingerprint(p, "password-v1"); err != nil {
		t.Fatal(err)
	}
	// First sync stores fp; sessions from before auth should be wiped.
	if _, ok, _ := p.GetSession("sess-old"); ok {
		t.Fatal("first fingerprint sync should revoke existing sessions")
	}

	if err := p.InsertSession(store.AuthSession{
		ID: "sess-keep", CreatedAt: now, ExpiresAt: now + 3600, IdleDeadline: now + 3600, AuthMethod: "password",
	}); err != nil {
		t.Fatal(err)
	}
	if err := auth.SyncPasswordFingerprint(p, "password-v1"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := p.GetSession("sess-keep"); !ok {
		t.Fatal("same password should keep sessions")
	}

	if err := auth.SyncPasswordFingerprint(p, "password-v2"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := p.GetSession("sess-keep"); ok {
		t.Fatal("password change should revoke sessions")
	}
	fp, ok, err := p.GetKV(auth.PasswordFingerprintKey)
	if err != nil || !ok {
		t.Fatalf("fp stored ok=%v err=%v", ok, err)
	}
	if string(fp) != auth.PasswordFingerprint("password-v2") {
		t.Fatalf("fp=%q", fp)
	}
}
