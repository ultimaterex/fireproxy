package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"fireproxy/server/internal/store"
	"fireproxy/server/internal/tplink"
)

func (s *Server) tplinkStore() *tplink.Store {
	if s.TPLink != nil {
		return s.TPLink
	}
	if s.Persist != nil {
		s.TPLink = tplink.NewStore(s.Persist, "")
	}
	return s.TPLink
}

func (s *Server) listTPLinkSwitches(w http.ResponseWriter, r *http.Request) {
	st := s.tplinkStore()
	if st == nil {
		writeJSON(w, http.StatusOK, map[string]any{"switches": []tplink.Public{}})
		return
	}
	list, err := st.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]tplink.Public, 0, len(list))
	for _, c := range list {
		out = append(out, c.PublicView())
	}
	writeJSON(w, http.StatusOK, map[string]any{"switches": out, "secrets_ready": st.HasSecretsKey()})
}

func (s *Server) getTPLinkSwitch(w http.ResponseWriter, r *http.Request) {
	st := s.tplinkStore()
	if st == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	c, ok, err := st.Get(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, c.PublicView())
}

func (s *Server) createTPLinkSwitch(w http.ResponseWriter, r *http.Request) {
	st := s.tplinkStore()
	if st == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		IP         string  `json:"ip"`
		MAC        string  `json:"mac"`
		Username   string  `json:"username"`
		Password   string  `json:"password"`
		Enabled    *bool   `json:"enabled"`
		UplinkPort *string `json:"uplink_port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	in := tplink.UpsertInput{IP: body.IP, MAC: body.MAC, Username: body.Username, Enabled: body.Enabled, UplinkPort: body.UplinkPort}
	if body.Password != "" {
		p := body.Password
		in.Password = &p
	}
	c, err := st.Create(in)
	if errors.Is(err, tplink.ErrNoSecretsKey) {
		http.Error(w, "FIREPROXY_SECRETS_KEY required to store password", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, c.PublicView())
}

func (s *Server) patchTPLinkSwitch(w http.ResponseWriter, r *http.Request) {
	st := s.tplinkStore()
	if st == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		IP         string  `json:"ip"`
		MAC        string  `json:"mac"`
		Username   string  `json:"username"`
		Password   *string `json:"password"`
		Enabled    *bool   `json:"enabled"`
		UplinkPort *string `json:"uplink_port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	c, err := st.Update(r.PathValue("id"), tplink.UpsertInput{
		IP: body.IP, MAC: body.MAC, Username: body.Username, Password: body.Password, Enabled: body.Enabled,
		UplinkPort: body.UplinkPort,
	})
	if errors.Is(err, tplink.ErrNoSecretsKey) {
		http.Error(w, "FIREPROXY_SECRETS_KEY required to store password", http.StatusBadRequest)
		return
	}
	if err != nil {
		if err.Error() == "not found" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, c.PublicView())
}

func (s *Server) deleteTPLinkSwitch(w http.ResponseWriter, r *http.Request) {
	st := s.tplinkStore()
	if st == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := st.Delete(r.PathValue("id")); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) testTPLinkSwitch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID       string `json:"id"`
		IP       string `json:"ip"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	ip := strings.TrimSpace(body.IP)
	user := body.Username
	pass := body.Password
	if body.ID != "" {
		st := s.tplinkStore()
		if st == nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		c, ok, err := st.Get(body.ID)
		if err != nil || !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if ip == "" {
			ip = c.IP
		}
		if user == "" {
			user = c.Username
		}
		if pass == "" {
			pass, err = st.DecryptPassword(c)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
	}
	if ip == "" || pass == "" {
		http.Error(w, "ip and password required", http.StatusBadRequest)
		return
	}
	if user == "" {
		user = "admin"
	}
	mod := tplink.New(tplink.ModuleConfig{})
	sw, err := mod.TestOne(ip, user, pass)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "switch": sw})
}

func (s *Server) listTPLinkCandidates(w http.ResponseWriter, r *http.Request) {
	st := s.tplinkStore()
	var configured []tplink.Config
	if st != nil {
		configured, _ = st.List()
	}
	cat, ok := s.getCatalog()
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"candidates": []tplink.Candidate{}})
		return
	}
	var p *store.Persist
	if s.Persist != nil {
		p = s.Persist
	} else if st != nil {
		p = st.Persist()
	}
	cache := tplink.NewProbeCache(p)
	writeJSON(w, http.StatusOK, map[string]any{
		"candidates": tplink.CandidatesFromCatalog(cat, configured, cache),
	})
}

func (s *Server) getTPLinkSettings(w http.ResponseWriter, r *http.Request) {
	p := s.tplinkPrefs().Get()
	writeJSON(w, http.StatusOK, map[string]any{
		"poll_sec":     p.PollSec,
		"poll_notches": tplink.PollNotches,
	})
}

func (s *Server) putTPLinkSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PollSec *int `json:"poll_sec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	cur := s.tplinkPrefs().Get()
	if body.PollSec != nil {
		cur.PollSec = *body.PollSec
	}
	cur = s.tplinkPrefs().Set(cur)
	writeJSON(w, http.StatusOK, map[string]any{
		"poll_sec":     cur.PollSec,
		"poll_notches": tplink.PollNotches,
	})
}
