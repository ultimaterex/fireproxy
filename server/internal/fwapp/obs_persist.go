package fwapp

import (
	"encoding/json"
	"log"
	"time"
)

const (
	kvObsCache        = "fw_app_obs_cache_v1"
	InitPersistMaxAge = 24 * time.Hour
)

type persistedObsCache struct {
	RefreshedAt time.Time           `json:"refreshedAt"`
	Snapshot    ObservatorySnapshot `json:"snapshot"`
}

func obsSnapshotEmpty(snap ObservatorySnapshot) bool {
	return snap.Box == nil && len(snap.Devices) == 0 && snap.AlarmCount == 0
}

func (s *Service) persistObservatoryCache(snap ObservatorySnapshot, at time.Time) {
	if s == nil || s.vault == nil || s.vault.Store == nil {
		return
	}
	raw, err := json.Marshal(persistedObsCache{RefreshedAt: at.UTC(), Snapshot: snap})
	if err != nil {
		log.Printf("fwapp: marshal observatory cache: %v", err)
		return
	}
	if err := s.vault.Store.PutKV(kvObsCache, raw); err != nil {
		log.Printf("fwapp: persist observatory cache: %v", err)
	}
}

func (s *Service) clearPersistedObservatoryCache() {
	if s == nil || s.vault == nil || s.vault.Store == nil {
		return
	}
	_ = s.vault.Store.PutKV(kvObsCache, []byte{})
}

func (s *Service) hydrateObservatoryCacheFromStore() (ObservatorySnapshot, time.Time, bool) {
	if s == nil || s.vault == nil || s.vault.Store == nil {
		return ObservatorySnapshot{}, time.Time{}, false
	}
	raw, ok, err := s.vault.Store.GetKV(kvObsCache)
	if err != nil || !ok || len(raw) == 0 {
		return ObservatorySnapshot{}, time.Time{}, false
	}
	var p persistedObsCache
	if err := json.Unmarshal(raw, &p); err != nil {
		log.Printf("fwapp: load observatory cache: %v", err)
		return ObservatorySnapshot{}, time.Time{}, false
	}
	if obsSnapshotEmpty(p.Snapshot) {
		return ObservatorySnapshot{}, time.Time{}, false
	}
	at := p.RefreshedAt
	if at.IsZero() {
		return ObservatorySnapshot{}, time.Time{}, false
	}
	if time.Since(at) > InitPersistMaxAge {
		s.clearPersistedObservatoryCache()
		return ObservatorySnapshot{}, time.Time{}, false
	}
	s.obsCache.Set(p.Snapshot, at)
	return s.obsCache.Get()
}
