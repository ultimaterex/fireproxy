package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"fireproxy/server/internal/store"
	"fireproxy/server/internal/tplink"
)

const oidcPendingTTL = 10 * time.Minute

// oidcStateCookie binds the authorization-code flow to the browser that started
// it. Without it, state lives only in a global server table, so any browser
// presenting a valid state+code completes the login — a login-CSRF / session
// fixation vector. The callback requires this cookie to match the query state.
const oidcStateCookie = "fp_oidc_state"

func setOIDCStateCookie(w http.ResponseWriter, r *http.Request, cfg Config, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookie,
		Value:    state,
		Path:     "/v1/auth/oidc",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode, // Lax so it survives the IdP redirect back
		Secure:   CookieSecure(r, cfg),
		MaxAge:   int(oidcPendingTTL / time.Second),
	})
}

func clearOIDCStateCookie(w http.ResponseWriter, r *http.Request, cfg Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookie,
		Value:    "",
		Path:     "/v1/auth/oidc",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   CookieSecure(r, cfg),
		MaxAge:   -1,
	})
}

func oidcStateFromRequest(r *http.Request) string {
	c, err := r.Cookie(oidcStateCookie)
	if err != nil || c == nil {
		return ""
	}
	return strings.TrimSpace(c.Value)
}

// OIDCIdentity is the verified identity from an IdP after code exchange.
type OIDCIdentity struct {
	Subject string
	Email   string
	Nonce   string
}

// OIDCEnabled reports whether OIDC login is available: issuer + client_id set and allowlist non-empty.
func OIDCEnabled(s store.OIDCSettings) bool {
	if strings.TrimSpace(s.Issuer) == "" || strings.TrimSpace(s.ClientID) == "" {
		return false
	}
	return allowlistNonEmpty(s.Allowlist)
}

func allowlistNonEmpty(allow []string) bool {
	for _, e := range allow {
		if strings.TrimSpace(e) != "" {
			return true
		}
	}
	return false
}

// AllowlistAllows reports whether email or subject matches the allowlist.
// Entries may be bare emails/subs, or prefixed "email:" / "sub:".
func AllowlistAllows(allowlist []string, email, sub string) bool {
	email = strings.TrimSpace(strings.ToLower(email))
	sub = strings.TrimSpace(sub)
	for _, raw := range allowlist {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		lower := strings.ToLower(entry)
		switch {
		case strings.HasPrefix(lower, "email:"):
			want := strings.TrimSpace(entry[len("email:"):])
			if email != "" && strings.EqualFold(email, want) {
				return true
			}
		case strings.HasPrefix(lower, "sub:"):
			want := strings.TrimSpace(entry[len("sub:"):])
			if sub != "" && subtleEqual(sub, want) {
				return true
			}
		default:
			if email != "" && strings.EqualFold(email, entry) {
				return true
			}
			if sub != "" && subtleEqual(sub, entry) {
				return true
			}
		}
	}
	return false
}

func subtleEqual(a, b string) bool {
	return strings.TrimSpace(a) == strings.TrimSpace(b)
}

// LoginOptions is public; oidc_enabled when issuer+client configured and allowlist non-empty.
func (h *Handlers) LoginOptions(w http.ResponseWriter, r *http.Request) {
	enabled := false
	name := ""
	if h != nil && h.Persist != nil {
		if s, ok, err := h.Persist.GetOIDCSettings(); err == nil && ok {
			enabled = OIDCEnabled(s)
			name = strings.TrimSpace(s.Name)
		}
	}
	out := map[string]any{"oidc_enabled": enabled}
	if name != "" {
		out["oidc_name"] = name
	}
	writeJSON(w, http.StatusOK, out)
}

// OIDCStart begins the authorization-code + PKCE flow.
func (h *Handlers) OIDCStart(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Persist == nil {
		http.Error(w, "oidc unavailable", http.StatusServiceUnavailable)
		return
	}
	settings, ok, err := h.Persist.GetOIDCSettings()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok || !OIDCEnabled(settings) {
		http.Error(w, "oidc disabled: empty allowlist or incomplete config", http.StatusBadRequest)
		return
	}
	secret, err := h.decryptOIDCSecret(settings)
	if err != nil {
		http.Error(w, "oidc secret unavailable", http.StatusBadRequest)
		return
	}

	_ = h.Persist.DeleteExpiredOIDCPending(time.Now().Unix())

	state, err := randomOIDCValue()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	nonce, err := randomOIDCValue()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	verifier := oauth2.GenerateVerifier()

	if err := h.Persist.PutOIDCPending(store.OIDCPending{
		State:        state,
		Nonce:        nonce,
		CodeVerifier: verifier,
		ExpiresAt:    time.Now().Add(oidcPendingTTL).Unix(),
	}); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	authURL, err := h.buildAuthCodeURL(r.Context(), settings, secret, state, nonce, verifier)
	if err != nil {
		log.Printf("oidc provider error: %v", err)
		_, _, _ = h.Persist.TakeOIDCPending(state) // best-effort cleanup
		http.Error(w, "oidc provider error", http.StatusBadGateway)
		return
	}
	setOIDCStateCookie(w, r, h.Cfg, state)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// OIDCCallback completes the OIDC login.
func (h *Handlers) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Persist == nil {
		http.Error(w, "oidc unavailable", http.StatusServiceUnavailable)
		return
	}
	if errParam := strings.TrimSpace(r.URL.Query().Get("error")); errParam != "" {
		http.Error(w, "oidc error: "+errParam, http.StatusBadRequest)
		return
	}
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if state == "" || code == "" {
		http.Error(w, "missing state or code", http.StatusBadRequest)
		return
	}

	// Bind the callback to the browser that started the flow. An attacker's
	// valid state+code is useless without the matching cookie in the victim's
	// browser.
	if cs := oidcStateFromRequest(r); cs == "" || !secureStringEqual(cs, state) {
		clearOIDCStateCookie(w, r, h.Cfg)
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	clearOIDCStateCookie(w, r, h.Cfg)

	pending, ok, err := h.Persist.TakeOIDCPending(state)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	if time.Now().Unix() > pending.ExpiresAt {
		http.Error(w, "expired state", http.StatusBadRequest)
		return
	}

	settings, ok, err := h.Persist.GetOIDCSettings()
	if err != nil || !ok || !OIDCEnabled(settings) {
		http.Error(w, "oidc disabled", http.StatusBadRequest)
		return
	}
	secret, err := h.decryptOIDCSecret(settings)
	if err != nil {
		http.Error(w, "oidc secret unavailable", http.StatusBadRequest)
		return
	}

	ident, err := h.exchangeOIDC(r.Context(), code, pending.CodeVerifier, settings, secret)
	if err != nil {
		log.Printf("oidc token exchange failed: %v", err)
		http.Error(w, "token exchange failed", http.StatusBadRequest)
		return
	}
	if ident.Nonce != pending.Nonce {
		http.Error(w, "invalid nonce", http.StatusBadRequest)
		return
	}
	if !AllowlistAllows(settings.Allowlist, ident.Email, ident.Subject) {
		http.Error(w, "not allowlisted", http.StatusForbidden)
		return
	}

	id, err := CreateSession(h.Persist, h.Cfg, "oidc")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	SetSessionCookie(w, r, h.Cfg, id, h.Cfg.SessionAbs)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handlers) decryptOIDCSecret(settings store.OIDCSettings) (string, error) {
	if len(settings.SecretCiphertext) == 0 {
		return "", fmt.Errorf("no client secret")
	}
	key, err := tplink.KeyFromEnv(os.Getenv("FIREPROXY_SECRETS_KEY"))
	if err != nil {
		return "", err
	}
	return tplink.Decrypt(key, string(settings.SecretCiphertext))
}

func (h *Handlers) buildAuthCodeURL(ctx context.Context, settings store.OIDCSettings, clientSecret, state, nonce, codeVerifier string) (string, error) {
	if h != nil && h.OIDCAuthURL != nil {
		return h.OIDCAuthURL(ctx, settings, clientSecret, state, nonce, codeVerifier)
	}
	provider, err := oidc.NewProvider(ctx, strings.TrimSpace(settings.Issuer))
	if err != nil {
		return "", err
	}
	cfg := oauth2Config(provider, settings, clientSecret)
	return cfg.AuthCodeURL(state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(codeVerifier),
	), nil
}

func (h *Handlers) exchangeOIDC(ctx context.Context, code, codeVerifier string, settings store.OIDCSettings, clientSecret string) (OIDCIdentity, error) {
	if h != nil && h.OIDCExchange != nil {
		return h.OIDCExchange(ctx, code, codeVerifier, settings, clientSecret)
	}
	provider, err := oidc.NewProvider(ctx, strings.TrimSpace(settings.Issuer))
	if err != nil {
		return OIDCIdentity{}, err
	}
	cfg := oauth2Config(provider, settings, clientSecret)
	tok, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return OIDCIdentity{}, err
	}
	rawIDToken, ok := tok.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return OIDCIdentity{}, fmt.Errorf("no id_token")
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: settings.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return OIDCIdentity{}, err
	}
	var claims struct {
		Email         string          `json:"email"`
		EmailVerified json.RawMessage `json:"email_verified"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return OIDCIdentity{}, err
	}
	// Only a verified email may satisfy an email allowlist entry. An attacker
	// can register any address at the IdP without proving ownership, so an
	// unverified email is dropped here and only the subject can match.
	email := claims.Email
	if !jsonBoolTrue(claims.EmailVerified) {
		email = ""
	}
	return OIDCIdentity{
		Subject: idToken.Subject,
		Email:   email,
		Nonce:   idToken.Nonce,
	}, nil
}

// jsonBoolTrue reports whether a JSON scalar is boolean true or the string
// "true". IdPs are inconsistent about encoding email_verified; absent or any
// other value is treated as false.
func jsonBoolTrue(raw json.RawMessage) bool {
	s := strings.TrimSpace(strings.Trim(strings.TrimSpace(string(raw)), `"`))
	return strings.EqualFold(s, "true")
}

func oauth2Config(provider *oidc.Provider, settings store.OIDCSettings, clientSecret string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     settings.ClientID,
		ClientSecret: clientSecret,
		RedirectURL:  settings.RedirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}
}

func randomOIDCValue() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	// URL-safe, no padding — fine for state/nonce query params.
	sum := sha256.Sum256(b[:])
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}
