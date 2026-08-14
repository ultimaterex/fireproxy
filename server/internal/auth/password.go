package auth

import (
	"crypto/subtle"
	"fmt"

	"fireproxy/server/internal/store"
)

// PasswordFingerprintKey is the kv key for the AUTH_PASSWORD fingerprint.
const PasswordFingerprintKey = "auth.password_fp"

// VerifyPassword compares candidate to AUTH_PASSWORD in constant time
// via equal-length SHA-256 fingerprints (avoids length early-out).
func VerifyPassword(cfg Config, candidate string) bool {
	a := []byte(PasswordFingerprint(candidate))
	b := []byte(PasswordFingerprint(cfg.Password))
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}

// PasswordFingerprint returns hex(sha256(password)).
func PasswordFingerprint(password string) string {
	return HashToken(password)
}

// SyncPasswordFingerprint stores sha256(AUTH_PASSWORD) in kv.
// If the fingerprint differs from the stored value (or is missing), all sessions are deleted.
func SyncPasswordFingerprint(p *store.Persist, password string) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	fp := PasswordFingerprint(password)
	stored, ok, err := p.GetKV(PasswordFingerprintKey)
	if err != nil {
		return err
	}
	if ok && string(stored) == fp {
		return nil
	}
	if err := p.DeleteAllSessions(); err != nil {
		return err
	}
	return p.PutKV(PasswordFingerprintKey, []byte(fp))
}
