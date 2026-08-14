package store

import (
	"log"
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
