package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"fireproxy/server/internal/store"
)

// SessionCookie is the browser session cookie name.
const SessionCookie = "fp_session"

// CookieSecure reports whether the session cookie should set the Secure flag.
// Secure when the request is TLS, or when TrustedProxy and X-Forwarded-Proto is https.
func CookieSecure(r *http.Request, cfg Config) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	if cfg.TrustedProxy && strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}
	return false
}

// SetSessionCookie writes the fp_session cookie.
func SetSessionCookie(w http.ResponseWriter, r *http.Request, cfg Config, sessionID string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   CookieSecure(r, cfg),
		MaxAge:   int(maxAge.Seconds()),
	})
}

// ClearSessionCookie expires the fp_session cookie.
func ClearSessionCookie(w http.ResponseWriter, r *http.Request, cfg Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   CookieSecure(r, cfg),
		MaxAge:   -1,
	})
}

// CreateSession inserts a new server-backed session and returns its id (cookie value).
func CreateSession(p *store.Persist, cfg Config, method string) (id string, err error) {
	if p == nil {
		return "", fmt.Errorf("nil persist")
	}
	id, err = NewToken("fp_sess_")
	if err != nil {
		return "", err
	}
	now := time.Now()
	s := store.AuthSession{
		ID:           id,
		CreatedAt:    now.Unix(),
		ExpiresAt:    now.Add(cfg.SessionAbs).Unix(),
		IdleDeadline: now.Add(cfg.SessionIdle).Unix(),
		AuthMethod:   method,
	}
	if err := p.InsertSession(s); err != nil {
		return "", err
	}
	return id, nil
}

// LookupSession loads a non-expired session by id. ok=false when missing or expired.
func LookupSession(p *store.Persist, id string) (store.AuthSession, bool, error) {
	if p == nil || id == "" {
		return store.AuthSession{}, false, nil
	}
	s, ok, err := p.GetSession(id)
	if err != nil || !ok {
		return store.AuthSession{}, ok, err
	}
	now := time.Now().Unix()
	if now > s.ExpiresAt || now > s.IdleDeadline {
		_ = p.DeleteSession(id)
		return store.AuthSession{}, false, nil
	}
	return s, true, nil
}

// TouchSessionIdle extends idle_deadline using cfg.SessionIdle.
func TouchSessionIdle(p *store.Persist, cfg Config, id string) error {
	if p == nil || id == "" {
		return nil
	}
	return p.TouchSession(id, time.Now().Add(cfg.SessionIdle).Unix())
}
