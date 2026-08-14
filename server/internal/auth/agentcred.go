package auth

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"sync"
	"time"

	"fireproxy/server/internal/store"
)

// AgentVerifier authorizes agent ingest / WebSocket requests.
type AgentVerifier interface {
	AuthorizeAgent(r *http.Request) bool
}

// AuthorizeFunc adapts a function to AgentVerifier.
type AuthorizeFunc func(*http.Request) bool

func (f AuthorizeFunc) AuthorizeAgent(r *http.Request) bool {
	if f == nil {
		return false
	}
	return f(r)
}

// StaticToken returns a verifier that accepts a single plaintext token
// (Bearer or X-API-Key) via constant-time compare. Useful in tests.
func StaticToken(token string) AgentVerifier {
	return AuthorizeFunc(func(r *http.Request) bool {
		got := extractAgentToken(r)
		if got == "" || token == "" {
			return false
		}
		return secureStringEqual(got, token)
	})
}

// AgentCreds verifies agent tokens against Persist hashes, or DevAgentToken when auth is disabled.
type AgentCreds struct {
	Persist *store.Persist
	Cfg     Config

	gateOnce sync.Once
	gate     *touchGate // coalesces last-used writes across bursty ingest
}

// AuthorizeAgent extracts Bearer or X-API-Key and verifies the credential.
func (a *AgentCreds) AuthorizeAgent(r *http.Request) bool {
	if a == nil {
		return false
	}
	token := extractAgentToken(r)
	if token == "" {
		return false
	}
	if a.Cfg.Disabled && a.Cfg.DevAgentToken != "" {
		if secureStringEqual(token, a.Cfg.DevAgentToken) {
			return true
		}
	}
	if a.Persist == nil {
		return false
	}
	hash := HashToken(token)
	cred, ok, err := a.Persist.LookupAgentCredentialByHash(hash)
	if err != nil || !ok {
		return false
	}
	if !VerifyToken(token, cred.Hash) {
		return false
	}
	// Coalesce the last-used write: bursty ingest would otherwise UPDATE on
	// every POST. The first authorization in an interval still writes, so
	// last_used is set promptly after enrollment.
	a.gateOnce.Do(func() { a.gate = newTouchGate(time.Minute) })
	if a.gate.due("a:"+cred.ID, time.Now()) {
		_ = a.Persist.TouchAgentCredentialLastUsed(cred.ID, time.Now().Unix())
	}
	return true
}

// MintAgentCredential generates a new agent token, deletes all prior credentials, and stores the hash.
func MintAgentCredential(p *store.Persist) (plaintext string, err error) {
	if p == nil {
		return "", fmt.Errorf("nil persist")
	}
	plaintext, err = NewToken("fp_ag_")
	if err != nil {
		return "", err
	}
	id, err := NewToken("ag_")
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	if err := p.ReplaceAgentCredential(store.AuthAgentCredential{
		ID:        id,
		Hash:      HashToken(plaintext),
		CreatedAt: now,
	}); err != nil {
		return "", err
	}
	return plaintext, nil
}

func extractAgentToken(r *http.Request) string {
	token, _ := extractBearerOrAPIKey(r)
	return token
}

func secureStringEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
