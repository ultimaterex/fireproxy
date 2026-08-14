package enroll

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

const DefaultTTL = 15 * time.Minute

// CodeStore holds at most one outstanding enroll code.
type CodeStore struct {
	mu      sync.Mutex
	current *ticket
}

type ticket struct {
	Code      string
	BaseURL   string
	ExpiresAt time.Time
	Used      bool
}

// Mint creates a new code bound to baseURL; invalidates any prior unused code.
func (s *CodeStore) Mint(baseURL string, now time.Time, ttl time.Duration) (code string, expires time.Time, err error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "", time.Time{}, fmt.Errorf("public base URL required")
	}
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	code = hex.EncodeToString(b)
	expires = now.Add(ttl)
	s.mu.Lock()
	s.current = &ticket{Code: code, BaseURL: baseURL, ExpiresAt: expires}
	s.mu.Unlock()
	return code, expires, nil
}

// Exchange consumes a valid unused code and returns its bound base URL.
func (s *CodeStore) Exchange(code string, now time.Time) (baseURL string, err error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", fmt.Errorf("code required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.current
	if t == nil || t.Code != code {
		return "", fmt.Errorf("invalid code")
	}
	if t.Used {
		return "", fmt.Errorf("code already used")
	}
	if !now.Before(t.ExpiresAt) {
		return "", fmt.Errorf("code expired")
	}
	t.Used = true
	return t.BaseURL, nil
}
