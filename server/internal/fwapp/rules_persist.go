package fwapp

import (
	"encoding/json"
	"log"
	"time"
)

const kvRulesCache = "fw_app_rules_cache_v1"

type persistedRulesCache struct {
	RefreshedAt time.Time     `json:"refreshedAt"`
	Snapshot    RulesSnapshot `json:"snapshot"`
}

func (s *Service) persistRulesCache(snap RulesSnapshot, at time.Time) {
	if s == nil || s.vault == nil || s.vault.Store == nil {
		return
	}
	raw, err := json.Marshal(persistedRulesCache{RefreshedAt: at.UTC(), Snapshot: snap})
	if err != nil {
		log.Printf("fwapp: marshal rules cache: %v", err)
		return
	}
	if err := s.vault.Store.PutKV(kvRulesCache, raw); err != nil {
		log.Printf("fwapp: persist rules cache: %v", err)
	}
}

func (s *Service) clearPersistedRulesCache() {
	if s == nil || s.vault == nil || s.vault.Store == nil {
		return
	}
	_ = s.vault.Store.PutKV(kvRulesCache, []byte{})
}

func (s *Service) hydrateRulesCacheFromStore() (RulesSnapshot, time.Time, bool) {
	if s == nil || s.vault == nil || s.vault.Store == nil {
		return RulesSnapshot{}, time.Time{}, false
	}
	raw, ok, err := s.vault.Store.GetKV(kvRulesCache)
	if err != nil || !ok || len(raw) == 0 {
		return RulesSnapshot{}, time.Time{}, false
	}
	var p persistedRulesCache
	if err := json.Unmarshal(raw, &p); err != nil {
		log.Printf("fwapp: load rules cache: %v", err)
		return RulesSnapshot{}, time.Time{}, false
	}
	if len(p.Snapshot.Rules) == 0 && len(p.Snapshot.Exceptions) == 0 {
		return RulesSnapshot{}, time.Time{}, false
	}
	at := p.RefreshedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}
	s.rules.Set(p.Snapshot, at)
	return s.rules.Get()
}
