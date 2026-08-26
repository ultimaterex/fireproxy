package api

import "fireproxy/server/internal/fwapp"

// multiWANNetworkExtras returns omitempty-friendly map keys for GET /v1/network.
func multiWANNetworkExtras(snap fwapp.ObservatorySnapshot, ok bool) map[string]any {
	out := map[string]any{
		"capabilities": map[string]any{"writes": false},
	}
	if !ok {
		return out
	}
	if snap.WanFeatures != nil {
		out["features"] = snap.WanFeatures
	}
	if snap.WanTest != nil {
		out["wan_test"] = snap.WanTest
	}
	if len(snap.VirtWANs) > 0 {
		out["virt_wans"] = snap.VirtWANs
	}
	return out
}
