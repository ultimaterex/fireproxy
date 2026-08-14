package geo

import "sync"

// Holder is a swappable Lookup so MMDB reloads do not restart the API.
type Holder struct {
	mu   sync.RWMutex
	look Lookup
}

func NewHolder(look Lookup) *Holder {
	return &Holder{look: look}
}

func (h *Holder) Country(ip string) string {
	if h == nil {
		return ""
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.look == nil {
		return ""
	}
	return h.look.Country(ip)
}

func (h *Holder) Ready() bool {
	if h == nil {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.look != nil
}

func (h *Holder) Swap(look Lookup) {
	if h == nil {
		return
	}
	h.mu.Lock()
	old := h.look
	h.look = look
	h.mu.Unlock()
	closeLookup(old)
}

type closer interface {
	Close() error
}

func closeLookup(look Lookup) {
	if c, ok := look.(closer); ok {
		_ = c.Close()
	}
}
