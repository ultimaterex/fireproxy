package auth

import (
	"fmt"
	"strings"
	"time"

	"fireproxy/server/internal/store"
)

// APIKeyInfo is a public API key view (never includes plaintext or hash).
type APIKeyInfo struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Scopes     []Scope `json:"scopes"`
	CreatedAt  int64   `json:"created_at"`
	LastUsedAt *int64  `json:"last_used_at,omitempty"`
}

// MintAPIKey creates a new API key. Plaintext is returned once; only the hash is stored.
func MintAPIKey(p *store.Persist, name string, scopes []Scope) (plaintext string, info APIKeyInfo, err error) {
	if p == nil {
		return "", APIKeyInfo{}, fmt.Errorf("nil persist")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", APIKeyInfo{}, fmt.Errorf("name required")
	}
	scopes = NormalizeScopes(scopes)
	if len(scopes) == 0 {
		return "", APIKeyInfo{}, fmt.Errorf("scopes required")
	}
	plaintext, err = NewToken("fp_")
	if err != nil {
		return "", APIKeyInfo{}, err
	}
	id, err := NewToken("key_")
	if err != nil {
		return "", APIKeyInfo{}, err
	}
	now := time.Now().Unix()
	row := store.AuthAPIKey{
		ID:        id,
		Name:      name,
		Hash:      HashToken(plaintext),
		Scopes:    ScopesCSV(scopes),
		CreatedAt: now,
	}
	if err := p.InsertAPIKey(row); err != nil {
		return "", APIKeyInfo{}, err
	}
	return plaintext, apiKeyInfoFromRow(row), nil
}

// ListAPIKeys returns metadata only (no plaintext, no hash).
func ListAPIKeys(p *store.Persist) ([]APIKeyInfo, error) {
	if p == nil {
		return nil, nil
	}
	rows, err := p.ListAPIKeys()
	if err != nil {
		return nil, err
	}
	out := make([]APIKeyInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, apiKeyInfoFromRow(r))
	}
	return out, nil
}

// RevokeAPIKey deletes an API key by id. Lookup by hash fails immediately after.
func RevokeAPIKey(p *store.Persist, id string) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id required")
	}
	return p.DeleteAPIKey(id)
}

// LookupAPIKey finds a stored key by plaintext token.
func LookupAPIKey(p *store.Persist, plaintext string) (store.AuthAPIKey, bool, error) {
	if p == nil || plaintext == "" {
		return store.AuthAPIKey{}, false, nil
	}
	return p.GetAPIKeyByHash(HashToken(plaintext))
}

// ScopesCSV joins scopes as a CSV string for storage.
func ScopesCSV(scopes []Scope) string {
	if len(scopes) == 0 {
		return ""
	}
	parts := make([]string, 0, len(scopes))
	for _, s := range scopes {
		parts = append(parts, string(s))
	}
	return strings.Join(parts, ",")
}

// ParseScopesCSV parses a CSV scopes string into Scope values.
func ParseScopesCSV(s string) []Scope {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]Scope, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, Scope(p))
	}
	return NormalizeScopes(out)
}

// NormalizeScopes keeps known scopes in stable order (read, write, admin), deduped.
func NormalizeScopes(in []Scope) []Scope {
	seen := map[Scope]bool{}
	for _, s := range in {
		switch s {
		case ScopeRead, ScopeWrite, ScopeAdmin:
			seen[s] = true
		}
	}
	var out []Scope
	for _, s := range []Scope{ScopeRead, ScopeWrite, ScopeAdmin} {
		if seen[s] {
			out = append(out, s)
		}
	}
	return out
}

func apiKeyInfoFromRow(r store.AuthAPIKey) APIKeyInfo {
	return APIKeyInfo{
		ID:         r.ID,
		Name:       r.Name,
		Scopes:     ParseScopesCSV(r.Scopes),
		CreatedAt:  r.CreatedAt,
		LastUsedAt: r.LastUsedAt,
	}
}
