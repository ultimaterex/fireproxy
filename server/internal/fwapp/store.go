package fwapp

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"fireproxy/server/internal/tplink"
)

const kvKeyCreds = "fw_app_creds_v1"

// Creds are persisted encrypted under FIREPROXY_SECRETS_KEY.
type Creds struct {
	Version    int       `json:"version"`
	PairedAt   time.Time `json:"pairedAt"`
	BoxIP      string    `json:"boxIP"`
	Gid        string    `json:"gid"`
	Eid        string    `json:"eid"`
	Aid        string    `json:"aid,omitempty"`
	SymKey     string    `json:"symKey"`
	RKeyTS     int64     `json:"rkeyts,omitempty"` // rotation key timestamp for local posts
	PrivatePEM string    `json:"privatePEM"`
	PublicPEM  string    `json:"publicPEM"`
	Email      string    `json:"email,omitempty"`
	DeviceName string    `json:"deviceName,omitempty"`
}

// Status is the safe, non-secret view for the UI.
type Status struct {
	Paired       bool   `json:"paired"`
	State        string `json:"state"` // unpaired | ready | lan-ok | lan-down | error
	BoxIP        string `json:"box_ip,omitempty"`
	GidHint      string `json:"gid_hint,omitempty"`
	Email        string `json:"email,omitempty"`
	DeviceName   string `json:"device_name,omitempty"`
	PairedAt     string `json:"paired_at,omitempty"`
	LastPingOK   bool   `json:"last_ping_ok,omitempty"`
	LastPingAt   string `json:"last_ping_at,omitempty"`
	LastError    string `json:"last_error,omitempty"`
	SecretsReady bool   `json:"secrets_ready"`
}

// Store persists encrypted credentials.
type Store interface {
	PutKV(k string, v []byte) error
	GetKV(k string) (v []byte, ok bool, err error)
}

type memStore struct {
	mu sync.Mutex
	m  map[string][]byte
}

func NewMemStore() Store {
	return &memStore{m: map[string][]byte{}}
}

func (s *memStore) PutKV(k string, v []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(v))
	copy(cp, v)
	s.m[k] = cp
	return nil
}

func (s *memStore) GetKV(k string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[k]
	if !ok {
		return nil, false, nil
	}
	cp := make([]byte, len(v))
	copy(cp, v)
	return cp, true, nil
}

// CredentialVault encrypts Creds at rest.
type CredentialVault struct {
	Store Store
	Key   []byte // 32-byte AES key from FIREPROXY_SECRETS_KEY
}

func (v *CredentialVault) Save(c Creds) error {
	if len(v.Key) == 0 {
		return fmt.Errorf("FIREPROXY_SECRETS_KEY not set")
	}
	c.Version = 1
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	enc, err := tplink.Encrypt(v.Key, string(raw))
	if err != nil {
		return err
	}
	return v.Store.PutKV(kvKeyCreds, []byte(enc))
}

func (v *CredentialVault) Load() (Creds, bool, error) {
	var zero Creds
	if len(v.Key) == 0 {
		return zero, false, fmt.Errorf("FIREPROXY_SECRETS_KEY not set")
	}
	raw, ok, err := v.Store.GetKV(kvKeyCreds)
	if err != nil || !ok || len(raw) == 0 {
		return zero, false, err
	}
	plain, err := tplink.Decrypt(v.Key, string(raw))
	if err != nil {
		return zero, false, err
	}
	var c Creds
	if err := json.Unmarshal([]byte(plain), &c); err != nil {
		return zero, false, err
	}
	return c, true, nil
}

func (v *CredentialVault) Clear() error {
	return v.Store.PutKV(kvKeyCreds, []byte{})
}

func gidHint(gid string) string {
	if len(gid) <= 8 {
		return gid
	}
	return gid[:4] + "…" + gid[len(gid)-4:]
}
