package tplink

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"fireproxy/server/internal/store"
)

const kvKey = "tplink_switches"

// Config is one configured Easy Smart switch (persisted).
type Config struct {
	ID                 string    `json:"id"`
	IP                 string    `json:"ip"`
	MAC                string    `json:"mac,omitempty"`
	Username           string    `json:"username"`
	PasswordCiphertext string    `json:"password_ciphertext,omitempty"`
	Enabled            bool      `json:"enabled"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	LastOKAt           time.Time `json:"last_ok_at,omitempty"`
	LastError          string    `json:"last_error,omitempty"`
	LastModel          string    `json:"last_model,omitempty"`
	LastFirmware       string    `json:"last_firmware,omitempty"`
	LastStatus         string    `json:"last_status,omitempty"` // ok|auth|down
	UplinkPort         string    `json:"uplink_port,omitempty"` // manual local jack facing upstream
}

// Public is the API view (never includes ciphertext).
type Public struct {
	ID           string    `json:"id"`
	IP           string    `json:"ip"`
	MAC          string    `json:"mac,omitempty"`
	Username     string    `json:"username"`
	HasPassword  bool      `json:"has_password"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	LastOKAt     time.Time `json:"last_ok_at,omitempty"`
	LastError    string    `json:"last_error,omitempty"`
	LastModel    string    `json:"last_model,omitempty"`
	LastFirmware string    `json:"last_firmware,omitempty"`
	LastStatus   string    `json:"last_status,omitempty"`
	UplinkPort   string    `json:"uplink_port,omitempty"`
}

func (c Config) PublicView() Public {
	return Public{
		ID: c.ID, IP: c.IP, MAC: c.MAC, Username: c.Username,
		HasPassword: c.PasswordCiphertext != "", Enabled: c.Enabled,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
		LastOKAt: c.LastOKAt, LastError: c.LastError,
		LastModel: c.LastModel, LastFirmware: c.LastFirmware, LastStatus: c.LastStatus,
		UplinkPort: c.UplinkPort,
	}
}

// Store persists switch configs in SQLite kv.
type Store struct {
	mu     sync.Mutex
	p      *store.Persist
	key    []byte // may be nil until set
	keyErr error
}

func NewStore(p *store.Persist, secretsEnv string) *Store {
	s := &Store{p: p}
	if k, err := KeyFromEnv(secretsEnv); err != nil {
		s.keyErr = err
	} else {
		s.key = k
	}
	return s
}

func (s *Store) SetSecretsKey(env string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, err := KeyFromEnv(env)
	s.key = k
	s.keyErr = err
}

func (s *Store) HasSecretsKey() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.key) == 32
}

// Persist returns the backing SQLite store (may be nil).
func (s *Store) Persist() *store.Persist {
	if s == nil {
		return nil
	}
	return s.p
}

func (s *Store) load() ([]Config, error) {
	if s.p == nil {
		return nil, nil
	}
	raw, ok, err := s.p.GetKV(kvKey)
	if err != nil || !ok {
		return nil, err
	}
	var list []Config
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Store) save(list []Config) error {
	if s.p == nil {
		return fmt.Errorf("no persist")
	}
	raw, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return s.p.PutKV(kvKey, raw)
}

func (s *Store) List() ([]Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Store) Get(id string) (Config, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.load()
	if err != nil {
		return Config{}, false, err
	}
	for _, c := range list {
		if c.ID == id {
			return c, true, nil
		}
	}
	return Config{}, false, nil
}

type UpsertInput struct {
	IP         string
	MAC        string
	Username   string
	Password   *string // nil = leave unchanged; non-nil set/clear
	Enabled    *bool
	UplinkPort *string // nil = leave unchanged; non-nil set ("" clears)
}

func (s *Store) Create(in UpsertInput) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(in.IP) == "" {
		return Config{}, fmt.Errorf("ip required")
	}
	user := in.Username
	if user == "" {
		user = "admin"
	}
	now := time.Now().UTC()
	c := Config{
		ID:        newID(),
		IP:        strings.TrimSpace(in.IP),
		MAC:       strings.ToUpper(strings.TrimSpace(in.MAC)),
		Username:  user,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if in.Enabled != nil {
		c.Enabled = *in.Enabled
	}
	if in.UplinkPort != nil {
		c.UplinkPort = strings.TrimSpace(*in.UplinkPort)
	}
	if in.Password != nil && *in.Password != "" {
		if len(s.key) != 32 {
			return Config{}, ErrNoSecretsKey
		}
		ct, err := Encrypt(s.key, *in.Password)
		if err != nil {
			return Config{}, err
		}
		c.PasswordCiphertext = ct
	}
	list, err := s.load()
	if err != nil {
		return Config{}, err
	}
	list = append(list, c)
	if err := s.save(list); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (s *Store) Update(id string, in UpsertInput) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.load()
	if err != nil {
		return Config{}, err
	}
	for i := range list {
		if list[i].ID != id {
			continue
		}
		if in.IP != "" {
			list[i].IP = strings.TrimSpace(in.IP)
		}
		if in.MAC != "" {
			list[i].MAC = strings.ToUpper(strings.TrimSpace(in.MAC))
		}
		if in.Username != "" {
			list[i].Username = in.Username
		}
		if in.Enabled != nil {
			list[i].Enabled = *in.Enabled
		}
		if in.UplinkPort != nil {
			list[i].UplinkPort = strings.TrimSpace(*in.UplinkPort)
		}
		if in.Password != nil {
			if *in.Password == "" {
				list[i].PasswordCiphertext = ""
			} else {
				if len(s.key) != 32 {
					return Config{}, ErrNoSecretsKey
				}
				ct, err := Encrypt(s.key, *in.Password)
				if err != nil {
					return Config{}, err
				}
				list[i].PasswordCiphertext = ct
			}
		}
		list[i].UpdatedAt = time.Now().UTC()
		if err := s.save(list); err != nil {
			return Config{}, err
		}
		return list[i], nil
	}
	return Config{}, fmt.Errorf("not found")
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.load()
	if err != nil {
		return err
	}
	out := list[:0]
	found := false
	for _, c := range list {
		if c.ID == id {
			found = true
			continue
		}
		out = append(out, c)
	}
	if !found {
		return fmt.Errorf("not found")
	}
	return s.save(out)
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (s *Store) DecryptPassword(c Config) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.PasswordCiphertext == "" {
		return "", fmt.Errorf("no password")
	}
	if len(s.key) != 32 {
		return "", ErrNoSecretsKey
	}
	return Decrypt(s.key, c.PasswordCiphertext)
}

func (s *Store) SaveStatus(id string, st Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.load()
	if err != nil {
		return err
	}
	for i := range list {
		if list[i].ID != id {
			continue
		}
		list[i].MAC = st.MAC
		list[i].LastOKAt = st.LastOKAt
		list[i].LastError = st.LastError
		list[i].LastModel = st.LastModel
		list[i].LastFirmware = st.LastFirmware
		list[i].LastStatus = st.LastStatus
		list[i].UpdatedAt = time.Now().UTC()
		return s.save(list)
	}
	return fmt.Errorf("not found")
}
