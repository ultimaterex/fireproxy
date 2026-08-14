package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/store"
	"fireproxy/server/internal/tplink"
)

func (s *Server) registerIAMRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/auth/api-keys", s.listAPIKeys)
	mux.HandleFunc("POST /v1/auth/api-keys", s.createAPIKey)
	mux.HandleFunc("DELETE /v1/auth/api-keys/{id}", s.deleteAPIKey)
	mux.HandleFunc("GET /v1/auth/oidc", s.getOIDCSettings)
	mux.HandleFunc("PUT /v1/auth/oidc", s.putOIDCSettings)
	mux.HandleFunc("GET /v1/auth/agent-credentials", s.listAgentCredentials)
	mux.HandleFunc("DELETE /v1/auth/agent-credentials", s.deleteAllAgentCredentials)
	mux.HandleFunc("DELETE /v1/auth/agent-credentials/{id}", s.deleteAgentCredential)
}

func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	p := s.persist()
	if p == nil {
		http.Error(w, "persist disabled", http.StatusServiceUnavailable)
		return
	}
	keys, err := auth.ListAPIKeys(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if keys == nil {
		keys = []auth.APIKeyInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	p := s.persist()
	if p == nil {
		http.Error(w, "persist disabled", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	scopes := make([]auth.Scope, 0, len(body.Scopes))
	for _, sc := range body.Scopes {
		scopes = append(scopes, auth.Scope(strings.TrimSpace(sc)))
	}
	plain, info, err := auth.MintAPIKey(p, body.Name, scopes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         info.ID,
		"name":       info.Name,
		"scopes":     info.Scopes,
		"created_at": info.CreatedAt,
		"token":      plain,
	})
}

func (s *Server) deleteAPIKey(w http.ResponseWriter, r *http.Request) {
	p := s.persist()
	if p == nil {
		http.Error(w, "persist disabled", http.StatusServiceUnavailable)
		return
	}
	id := r.PathValue("id")
	if err := auth.RevokeAPIKey(p, id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type oidcPublic struct {
	Name        string   `json:"name"`
	Issuer      string   `json:"issuer"`
	ClientID    string   `json:"client_id"`
	RedirectURI string   `json:"redirect_uri"`
	Allowlist   []string `json:"allowlist"`
	SecretSet   bool     `json:"secret_set"`
}

func (s *Server) getOIDCSettings(w http.ResponseWriter, r *http.Request) {
	p := s.persist()
	if p == nil {
		http.Error(w, "persist disabled", http.StatusServiceUnavailable)
		return
	}
	cfg, ok, err := p.GetOIDCSettings()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := oidcPublic{Allowlist: []string{}}
	if ok {
		out = oidcPublic{
			Name:        cfg.Name,
			Issuer:      cfg.Issuer,
			ClientID:    cfg.ClientID,
			RedirectURI: cfg.RedirectURI,
			Allowlist:   cfg.Allowlist,
			SecretSet:   len(cfg.SecretCiphertext) > 0,
		}
		if out.Allowlist == nil {
			out.Allowlist = []string{}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) putOIDCSettings(w http.ResponseWriter, r *http.Request) {
	p := s.persist()
	if p == nil {
		http.Error(w, "persist disabled", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Name         string   `json:"name"`
		Issuer       string   `json:"issuer"`
		ClientID     string   `json:"client_id"`
		RedirectURI  string   `json:"redirect_uri"`
		Allowlist    []string `json:"allowlist"`
		ClientSecret string   `json:"client_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.Allowlist == nil {
		body.Allowlist = []string{}
	}

	existing, _, err := p.GetOIDCSettings()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	secret := existing.SecretCiphertext
	if body.ClientSecret != "" {
		key, kerr := tplink.KeyFromEnv(os.Getenv("FIREPROXY_SECRETS_KEY"))
		if kerr != nil {
			http.Error(w, "FIREPROXY_SECRETS_KEY required to store client secret", http.StatusBadRequest)
			return
		}
		enc, eerr := tplink.Encrypt(key, body.ClientSecret)
		if eerr != nil {
			http.Error(w, eerr.Error(), http.StatusInternalServerError)
			return
		}
		secret = []byte(enc)
	}

	cfg := store.OIDCSettings{
		Name:             strings.TrimSpace(body.Name),
		Issuer:           strings.TrimSpace(body.Issuer),
		ClientID:         strings.TrimSpace(body.ClientID),
		RedirectURI:      strings.TrimSpace(body.RedirectURI),
		Allowlist:        body.Allowlist,
		SecretCiphertext: secret,
	}
	if err := p.PutOIDCSettings(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Changing OIDC config invalidates all browser sessions.
	if err := p.DeleteAllSessions(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, oidcPublic{
		Name:        cfg.Name,
		Issuer:      cfg.Issuer,
		ClientID:    cfg.ClientID,
		RedirectURI: cfg.RedirectURI,
		Allowlist:   cfg.Allowlist,
		SecretSet:   len(cfg.SecretCiphertext) > 0,
	})
}

func (s *Server) listAgentCredentials(w http.ResponseWriter, r *http.Request) {
	p := s.persist()
	if p == nil {
		http.Error(w, "persist disabled", http.StatusServiceUnavailable)
		return
	}
	creds, err := p.ListAgentCredentials()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type view struct {
		ID         string `json:"id"`
		CreatedAt  int64  `json:"created_at"`
		LastUsedAt *int64 `json:"last_used_at,omitempty"`
	}
	out := make([]view, 0, len(creds))
	for _, c := range creds {
		out = append(out, view{ID: c.ID, CreatedAt: c.CreatedAt, LastUsedAt: c.LastUsedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"credentials": out})
}

func (s *Server) deleteAllAgentCredentials(w http.ResponseWriter, r *http.Request) {
	p := s.persist()
	if p == nil {
		http.Error(w, "persist disabled", http.StatusServiceUnavailable)
		return
	}
	if err := p.DeleteAllAgentCredentials(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteAgentCredential(w http.ResponseWriter, r *http.Request) {
	p := s.persist()
	if p == nil {
		http.Error(w, "persist disabled", http.StatusServiceUnavailable)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if err := p.DeleteAgentCredential(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
