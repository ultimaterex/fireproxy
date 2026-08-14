package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

const (
	kvOIDCName             = "auth.oidc.name"
	kvOIDCIssuer           = "auth.oidc.issuer"
	kvOIDCClientID         = "auth.oidc.client_id"
	kvOIDCRedirectURI      = "auth.oidc.redirect_uri"
	kvOIDCAllowlist        = "auth.oidc.allowlist"
	kvOIDCSecretCiphertext = "auth.oidc.secret_ciphertext"
)

// AuthSession is a server-backed browser session row.
type AuthSession struct {
	ID           string
	CreatedAt    int64
	ExpiresAt    int64
	IdleDeadline int64
	AuthMethod   string
}

// AuthAPIKey is a hashed API key with CSV scopes.
type AuthAPIKey struct {
	ID         string
	Name       string
	Hash       string
	Scopes     string // CSV, e.g. "read,write"
	CreatedAt  int64
	LastUsedAt *int64
}

// AuthAgentCredential is a hashed agent enroll token.
type AuthAgentCredential struct {
	ID         string
	Hash       string
	CreatedAt  int64
	LastUsedAt *int64
}

// OIDCSettings are OIDC client settings stored in kv.
type OIDCSettings struct {
	Name             string // display label (e.g. Pocket ID); not used for discovery
	Issuer           string // issuer URL for OIDC discovery
	ClientID         string
	RedirectURI      string
	Allowlist        []string
	SecretCiphertext []byte
}

func (p *Persist) migrateAuth() error {
	_, err := p.db.Exec(`
CREATE TABLE IF NOT EXISTS auth_sessions (
  id TEXT PRIMARY KEY,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  idle_deadline INTEGER NOT NULL,
  auth_method TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS auth_api_keys (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  hash TEXT NOT NULL,
  scopes TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  last_used_at INTEGER
);
CREATE TABLE IF NOT EXISTS auth_agent_credentials (
  id TEXT PRIMARY KEY,
  hash TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  last_used_at INTEGER
);
CREATE TABLE IF NOT EXISTS auth_oidc_pending (
  state TEXT PRIMARY KEY,
  nonce TEXT NOT NULL,
  code_verifier TEXT NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS auth_api_keys_hash ON auth_api_keys(hash);
CREATE INDEX IF NOT EXISTS auth_agent_credentials_hash ON auth_agent_credentials(hash);
CREATE INDEX IF NOT EXISTS auth_oidc_pending_expires ON auth_oidc_pending(expires_at);
`)
	return err
}

// OIDCPending is a short-lived OIDC login attempt (PKCE + state + nonce).
type OIDCPending struct {
	State        string
	Nonce        string
	CodeVerifier string
	ExpiresAt    int64
}

// PutOIDCPending stores a pending OIDC login row.
func (p *Persist) PutOIDCPending(row OIDCPending) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	_, err := p.db.Exec(
		`INSERT OR REPLACE INTO auth_oidc_pending(state, nonce, code_verifier, expires_at) VALUES(?,?,?,?)`,
		row.State, row.Nonce, row.CodeVerifier, row.ExpiresAt,
	)
	return err
}

// TakeOIDCPending loads and deletes a pending row by state. ok=false when missing.
func (p *Persist) TakeOIDCPending(state string) (OIDCPending, bool, error) {
	if p == nil || state == "" {
		return OIDCPending{}, false, nil
	}
	tx, err := p.db.Begin()
	if err != nil {
		return OIDCPending{}, false, err
	}
	var row OIDCPending
	err = tx.QueryRow(
		`SELECT state, nonce, code_verifier, expires_at FROM auth_oidc_pending WHERE state=?`,
		state,
	).Scan(&row.State, &row.Nonce, &row.CodeVerifier, &row.ExpiresAt)
	if err == sql.ErrNoRows {
		_ = tx.Rollback()
		return OIDCPending{}, false, nil
	}
	if err != nil {
		_ = tx.Rollback()
		return OIDCPending{}, false, err
	}
	if _, err := tx.Exec(`DELETE FROM auth_oidc_pending WHERE state=?`, state); err != nil {
		_ = tx.Rollback()
		return OIDCPending{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return OIDCPending{}, false, err
	}
	return row, true, nil
}

// DeleteExpiredOIDCPending removes pending rows past expires_at.
func (p *Persist) DeleteExpiredOIDCPending(nowUnix int64) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	_, err := p.db.Exec(`DELETE FROM auth_oidc_pending WHERE expires_at < ?`, nowUnix)
	return err
}

// InsertSession stores a new session row.
func (p *Persist) InsertSession(s AuthSession) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	_, err := p.db.Exec(
		`INSERT INTO auth_sessions(id, created_at, expires_at, idle_deadline, auth_method) VALUES(?,?,?,?,?)`,
		s.ID, s.CreatedAt, s.ExpiresAt, s.IdleDeadline, s.AuthMethod,
	)
	return err
}

// GetSession loads a session by id. ok=false when missing.
func (p *Persist) GetSession(id string) (AuthSession, bool, error) {
	if p == nil {
		return AuthSession{}, false, nil
	}
	var s AuthSession
	err := p.db.QueryRow(
		`SELECT id, created_at, expires_at, idle_deadline, auth_method FROM auth_sessions WHERE id=?`,
		id,
	).Scan(&s.ID, &s.CreatedAt, &s.ExpiresAt, &s.IdleDeadline, &s.AuthMethod)
	if err == sql.ErrNoRows {
		return AuthSession{}, false, nil
	}
	if err != nil {
		return AuthSession{}, false, err
	}
	return s, true, nil
}

// TouchSession updates idle_deadline for sliding idle expiry.
func (p *Persist) TouchSession(id string, idleDeadline int64) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	_, err := p.db.Exec(`UPDATE auth_sessions SET idle_deadline=? WHERE id=?`, idleDeadline, id)
	return err
}

// DeleteSession removes one session.
func (p *Persist) DeleteSession(id string) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	_, err := p.db.Exec(`DELETE FROM auth_sessions WHERE id=?`, id)
	return err
}

// DeleteAllSessions removes every session row.
func (p *Persist) DeleteAllSessions() error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	_, err := p.db.Exec(`DELETE FROM auth_sessions`)
	return err
}

// InsertAPIKey stores a hashed API key.
func (p *Persist) InsertAPIKey(k AuthAPIKey) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	var last any
	if k.LastUsedAt != nil {
		last = *k.LastUsedAt
	}
	_, err := p.db.Exec(
		`INSERT INTO auth_api_keys(id, name, hash, scopes, created_at, last_used_at) VALUES(?,?,?,?,?,?)`,
		k.ID, k.Name, k.Hash, k.Scopes, k.CreatedAt, last,
	)
	return err
}

// ListAPIKeys returns all API keys newest-first by created_at.
func (p *Persist) ListAPIKeys() ([]AuthAPIKey, error) {
	if p == nil {
		return nil, nil
	}
	rows, err := p.db.Query(
		`SELECT id, name, hash, scopes, created_at, last_used_at FROM auth_api_keys ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuthAPIKey
	for rows.Next() {
		var k AuthAPIKey
		var last sql.NullInt64
		if err := rows.Scan(&k.ID, &k.Name, &k.Hash, &k.Scopes, &k.CreatedAt, &last); err != nil {
			return nil, err
		}
		if last.Valid {
			v := last.Int64
			k.LastUsedAt = &v
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// GetAPIKeyByHash looks up an API key by its stored hash.
func (p *Persist) GetAPIKeyByHash(hash string) (AuthAPIKey, bool, error) {
	if p == nil {
		return AuthAPIKey{}, false, nil
	}
	var k AuthAPIKey
	var last sql.NullInt64
	err := p.db.QueryRow(
		`SELECT id, name, hash, scopes, created_at, last_used_at FROM auth_api_keys WHERE hash=?`,
		hash,
	).Scan(&k.ID, &k.Name, &k.Hash, &k.Scopes, &k.CreatedAt, &last)
	if err == sql.ErrNoRows {
		return AuthAPIKey{}, false, nil
	}
	if err != nil {
		return AuthAPIKey{}, false, err
	}
	if last.Valid {
		v := last.Int64
		k.LastUsedAt = &v
	}
	return k, true, nil
}

// DeleteAPIKey removes one API key by id.
func (p *Persist) DeleteAPIKey(id string) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	_, err := p.db.Exec(`DELETE FROM auth_api_keys WHERE id=?`, id)
	return err
}

// TouchAPIKeyLastUsed sets last_used_at for an API key.
func (p *Persist) TouchAPIKeyLastUsed(id string, ts int64) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	_, err := p.db.Exec(`UPDATE auth_api_keys SET last_used_at=? WHERE id=?`, ts, id)
	return err
}

// InsertAgentCredential stores a hashed agent token.
func (p *Persist) InsertAgentCredential(c AuthAgentCredential) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	var last any
	if c.LastUsedAt != nil {
		last = *c.LastUsedAt
	}
	_, err := p.db.Exec(
		`INSERT INTO auth_agent_credentials(id, hash, created_at, last_used_at) VALUES(?,?,?,?)`,
		c.ID, c.Hash, c.CreatedAt, last,
	)
	return err
}

// ListAgentCredentials returns all agent credential rows newest-first.
func (p *Persist) ListAgentCredentials() ([]AuthAgentCredential, error) {
	if p == nil {
		return nil, nil
	}
	rows, err := p.db.Query(
		`SELECT id, hash, created_at, last_used_at FROM auth_agent_credentials ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuthAgentCredential
	for rows.Next() {
		var c AuthAgentCredential
		var last sql.NullInt64
		if err := rows.Scan(&c.ID, &c.Hash, &c.CreatedAt, &last); err != nil {
			return nil, err
		}
		if last.Valid {
			v := last.Int64
			c.LastUsedAt = &v
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteAllAgentCredentials removes every agent credential (supersede / revoke).
func (p *Persist) DeleteAllAgentCredentials() error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	_, err := p.db.Exec(`DELETE FROM auth_agent_credentials`)
	return err
}

// ReplaceAgentCredential deletes all agent credentials and inserts c in one transaction.
func (p *Persist) ReplaceAgentCredential(c AuthAgentCredential) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM auth_agent_credentials`); err != nil {
		_ = tx.Rollback()
		return err
	}
	var last any
	if c.LastUsedAt != nil {
		last = *c.LastUsedAt
	}
	if _, err := tx.Exec(
		`INSERT INTO auth_agent_credentials(id, hash, created_at, last_used_at) VALUES(?,?,?,?)`,
		c.ID, c.Hash, c.CreatedAt, last,
	); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// DeleteAgentCredential removes one agent credential by id.
func (p *Persist) DeleteAgentCredential(id string) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	_, err := p.db.Exec(`DELETE FROM auth_agent_credentials WHERE id=?`, id)
	return err
}

// LookupAgentCredentialByHash finds an agent credential by hash.
func (p *Persist) LookupAgentCredentialByHash(hash string) (AuthAgentCredential, bool, error) {
	if p == nil {
		return AuthAgentCredential{}, false, nil
	}
	var c AuthAgentCredential
	var last sql.NullInt64
	err := p.db.QueryRow(
		`SELECT id, hash, created_at, last_used_at FROM auth_agent_credentials WHERE hash=?`,
		hash,
	).Scan(&c.ID, &c.Hash, &c.CreatedAt, &last)
	if err == sql.ErrNoRows {
		return AuthAgentCredential{}, false, nil
	}
	if err != nil {
		return AuthAgentCredential{}, false, err
	}
	if last.Valid {
		v := last.Int64
		c.LastUsedAt = &v
	}
	return c, true, nil
}

// TouchAgentCredentialLastUsed sets last_used_at for an agent credential.
func (p *Persist) TouchAgentCredentialLastUsed(id string, ts int64) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	_, err := p.db.Exec(`UPDATE auth_agent_credentials SET last_used_at=? WHERE id=?`, ts, id)
	return err
}

// PutOIDCSettings writes OIDC settings into kv in a single transaction.
func (p *Persist) PutOIDCSettings(s OIDCSettings) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	allowJSON, err := json.Marshal(s.Allowlist)
	if err != nil {
		return err
	}
	if allowJSON == nil {
		allowJSON = []byte("[]")
	}
	secret := s.SecretCiphertext
	if secret == nil {
		secret = []byte{}
	}
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	put := func(k string, v []byte) error {
		_, err := tx.Exec(`INSERT OR REPLACE INTO kv(k,v) VALUES(?, ?)`, k, v)
		return err
	}
	for _, pair := range []struct {
		k string
		v []byte
	}{
		{kvOIDCName, []byte(s.Name)},
		{kvOIDCIssuer, []byte(s.Issuer)},
		{kvOIDCClientID, []byte(s.ClientID)},
		{kvOIDCRedirectURI, []byte(s.RedirectURI)},
		{kvOIDCAllowlist, allowJSON},
		{kvOIDCSecretCiphertext, secret},
	} {
		if err := put(pair.k, pair.v); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// GetOIDCSettings loads OIDC settings from kv. ok=false when no issuer is stored.
func (p *Persist) GetOIDCSettings() (OIDCSettings, bool, error) {
	if p == nil {
		return OIDCSettings{}, false, nil
	}
	issuer, ok, err := p.GetKV(kvOIDCIssuer)
	if err != nil {
		return OIDCSettings{}, false, err
	}
	if !ok {
		return OIDCSettings{}, false, nil
	}
	name, _, err := p.GetKV(kvOIDCName)
	if err != nil {
		return OIDCSettings{}, false, err
	}
	clientID, _, err := p.GetKV(kvOIDCClientID)
	if err != nil {
		return OIDCSettings{}, false, err
	}
	redirect, _, err := p.GetKV(kvOIDCRedirectURI)
	if err != nil {
		return OIDCSettings{}, false, err
	}
	allowRaw, allowOK, err := p.GetKV(kvOIDCAllowlist)
	if err != nil {
		return OIDCSettings{}, false, err
	}
	secret, _, err := p.GetKV(kvOIDCSecretCiphertext)
	if err != nil {
		return OIDCSettings{}, false, err
	}
	var allow []string
	if allowOK && len(allowRaw) > 0 {
		if err := json.Unmarshal(allowRaw, &allow); err != nil {
			return OIDCSettings{}, false, err
		}
	}
	if allow == nil {
		allow = []string{}
	}
	return OIDCSettings{
		Name:             string(name),
		Issuer:           string(issuer),
		ClientID:         string(clientID),
		RedirectURI:      string(redirect),
		Allowlist:        allow,
		SecretCiphertext: secret,
	}, true, nil
}
