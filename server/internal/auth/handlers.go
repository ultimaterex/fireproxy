package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"fireproxy/server/internal/store"
)

// Handlers serves auth HTTP endpoints.
type Handlers struct {
	Persist *store.Persist
	Cfg     Config

	// Optional test hooks for OIDC (nil = real discovery/exchange).
	OIDCAuthURL  func(ctx context.Context, settings store.OIDCSettings, clientSecret, state, nonce, codeVerifier string) (string, error)
	OIDCExchange func(ctx context.Context, code, codeVerifier string, settings store.OIDCSettings, clientSecret string) (OIDCIdentity, error)
}

// Register mounts auth routes on mux.
func (h *Handlers) Register(mux *http.ServeMux) {
	if h == nil || mux == nil {
		return
	}
	mux.HandleFunc("POST /v1/auth/login", h.Login)
	mux.HandleFunc("POST /v1/auth/logout", h.Logout)
	mux.HandleFunc("GET /v1/auth/me", h.Me)
	mux.HandleFunc("GET /v1/auth/login-options", h.LoginOptions)
	mux.HandleFunc("GET /v1/auth/oidc/start", h.OIDCStart)
	mux.HandleFunc("GET /v1/auth/oidc/callback", h.OIDCCallback)
}

// Healthz is a data-free liveness probe: plain "ok".
func Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login authenticates admin + AUTH_PASSWORD and sets a session cookie.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body loginRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	userOK := secureStringEqual(strings.TrimSpace(body.Username), "admin")
	passOK := VerifyPassword(h.Cfg, body.Password)
	if !userOK || !passOK {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.Persist == nil {
		http.Error(w, "persist unavailable", http.StatusServiceUnavailable)
		return
	}
	id, err := CreateSession(h.Persist, h.Cfg, "password")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	SetSessionCookie(w, r, h.Cfg, id, h.Cfg.SessionAbs)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"method":        "password",
	})
}

// Logout clears the session cookie and deletes the session row.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		ClearSessionCookie(w, r, Config{})
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if c, err := r.Cookie(SessionCookie); err == nil && c.Value != "" && h.Persist != nil {
		_ = h.Persist.DeleteSession(c.Value)
	}
	ClearSessionCookie(w, r, h.Cfg)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Me returns the current session principal.
// When auth is disabled, returns authenticated without a session.
// When auth is enabled and anonymous, returns 401.
func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.Cfg.Disabled {
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": true,
			"method":        "disabled",
		})
		return
	}
	sess, ok := h.sessionFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"method":        sess.AuthMethod,
	})
}

func (h *Handlers) sessionFromRequest(r *http.Request) (store.AuthSession, bool) {
	c, err := r.Cookie(SessionCookie)
	if err != nil || c.Value == "" {
		return store.AuthSession{}, false
	}
	s, ok, err := LookupSession(h.Persist, c.Value)
	if err != nil || !ok {
		return store.AuthSession{}, false
	}
	return s, true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
