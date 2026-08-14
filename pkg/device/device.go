// Package device defines the FireProxy inventory (host list) contract.
package device

// Device is a condensed Firewalla host record for the UI.
type Device struct {
	MAC          string   `json:"mac"`
	Name         string   `json:"name"`
	IP           string   `json:"ip,omitempty"`
	IPv6         []string `json:"ipv6,omitempty"`
	Vendor       string   `json:"vendor,omitempty"`
	Type         string   `json:"type,omitempty"` // Firewalla detect.type (phone, nas, …)
	LocalDomain  string   `json:"local_domain,omitempty"`
	LastActiveTS float64  `json:"last_active_ts,omitempty"`
	// Active is membership in Firewalla host:active:mac (≈7d). Nil = legacy agent.
	Active *bool `json:"active,omitempty"`
}

// Inventory is a full device list push from the agent.
type Inventory struct {
	TS      int64    `json:"ts"`
	Host    string   `json:"host"`
	Devices []Device `json:"devices"`
}
