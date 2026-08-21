package observatory

import "time"

const (
	SourceAgent     = "agent"
	SourceFWAppInit = "fw-app-init"
	SourceEmpty     = "empty"
)

// Provenance describes which backend filled an observatory DTO.
type Provenance struct {
	Source    string    `json:"source"`
	FetchedAt time.Time `json:"fetched_at,omitempty"`
	Stale     bool      `json:"stale,omitempty"`
}
