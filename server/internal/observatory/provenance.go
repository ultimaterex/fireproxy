package observatory

import "time"

const (
	SourceAgent     = "agent"
	SourceFWAppInit = "fw-app-init"
	SourceEmpty     = "empty"

	// ReasonPrefer = user forced control (header refresh / PreferInit hold).
	ReasonPrefer = "prefer"
	// ReasonFallback = agent offline or past TTL; control is the degrade path.
	ReasonFallback = "fallback"
)

// Provenance describes which backend filled an observatory DTO.
type Provenance struct {
	Source       string    `json:"source"`
	FetchedAt    time.Time `json:"fetched_at,omitempty"`
	Stale        bool      `json:"stale,omitempty"`
	EnrichedFrom string    `json:"enriched_from,omitempty"` // e.g. "agent" when gap-filled
	Reason       string    `json:"reason,omitempty"`        // prefer | fallback when source is fw-app-init
}
