package store

import (
	"log"
	"sort"
	"strings"
	"sync"

	"fireproxy/pkg/inventory"
)

// CatalogStore holds the latest full inventory catalog.
type CatalogStore struct {
	mu      sync.RWMutex
	cat     *inventory.Catalog
	persist *Persist
}

func NewCatalogStore() *CatalogStore {
	return &CatalogStore{}
}

func (s *CatalogStore) Set(cat inventory.Catalog) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := cat
	if cp.Devices == nil {
		cp.Devices = []inventory.Device{}
	}
	if cp.Network == nil {
		cp.Network = []inventory.NetworkIface{}
	}
	if cp.Policies == nil {
		cp.Policies = []inventory.Policy{}
	}
	if cp.Tags == nil {
		cp.Tags = []inventory.Tag{}
	}
	s.cat = &cp
	if s.persist != nil {
		if err := s.persist.SaveCatalog(cp); err != nil {
			log.Printf("persist catalog: %v", err)
		}
	}
}

// AttachPersist hydrates the last catalog from disk.
func (s *CatalogStore) AttachPersist(p *Persist) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.persist = p
	if p == nil {
		return
	}
	cat, ok, err := p.LoadCatalog()
	if err != nil {
		log.Printf("persist load catalog: %v", err)
		return
	}
	if ok {
		s.cat = &cat
	}
}

func (s *CatalogStore) Get() (inventory.Catalog, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cat == nil {
		return inventory.Catalog{}, false
	}
	return *s.cat, true
}

// MergeSpeedtest upserts LAN/App API speedtest points into the cached dashboard.
// Dedupes by (wan uuid, ts). Updates last Down/Up/Ping when a newer point arrives.
func (s *CatalogStore) MergeSpeedtest(points []inventory.SpeedtestPoint, wanUUID string) {
	wanUUID = strings.TrimSpace(wanUUID)
	if wanUUID == "" || len(points) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cat == nil {
		s.cat = &inventory.Catalog{}
	}
	dash := s.cat.Dashboard
	if dash == nil {
		dash = &inventory.Dashboard{}
		s.cat.Dashboard = dash
	}
	idx := -1
	for i := range dash.Speedtest {
		if dash.Speedtest[i].UUID == wanUUID {
			idx = i
			break
		}
	}
	if idx < 0 {
		dash.Speedtest = append(dash.Speedtest, inventory.SpeedtestWAN{
			UUID: wanUUID,
			Name: wanUUID,
		})
		idx = len(dash.Speedtest) - 1
	}
	row := &dash.Speedtest[idx]
	byTS := map[int64]inventory.SpeedtestPoint{}
	for _, p := range row.Points {
		byTS[p.TS] = p
	}
	for _, p := range points {
		if p.TS <= 0 || (p.Down == 0 && p.Up == 0) {
			continue
		}
		if prev, ok := byTS[p.TS]; ok {
			byTS[p.TS] = mergeSpeedPoint(prev, p)
		} else {
			byTS[p.TS] = p
		}
	}
	merged := make([]inventory.SpeedtestPoint, 0, len(byTS))
	for _, p := range byTS {
		merged = append(merged, p)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].TS < merged[j].TS })
	row.Points = merged
	if n := len(row.Points); n > 0 {
		last := row.Points[n-1]
		row.Down, row.Up, row.Ping = last.Down, last.Up, last.Ping
		row.ServerID, row.Server, row.Location = last.ServerID, last.Server, last.Location
	}
	if s.persist != nil {
		if err := s.persist.SaveCatalog(*s.cat); err != nil {
			log.Printf("persist catalog speedtest: %v", err)
		}
	}
}

func mergeSpeedPoint(prev, neu inventory.SpeedtestPoint) inventory.SpeedtestPoint {
	out := neu
	if out.ServerID == "" {
		out.ServerID = prev.ServerID
	}
	if out.Server == "" {
		out.Server = prev.Server
	}
	if out.Location == "" {
		out.Location = prev.Location
	}
	return out
}
