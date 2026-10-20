package api

import (
	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/observatory"
	"time"
)

// Keep extras' init age separate from the agent catalog timestamp.
func multiWANNetworkExtrasAt(snap fwapp.ObservatorySnapshot, at time.Time, ok bool, now time.Time) map[string]any {
	age := now.Sub(at)
	usable := ok && !at.IsZero() && age >= 0 && age <= fwapp.InitPersistMaxAge
	out := multiWANNetworkExtras(snap, usable)
	if !usable {
		return out
	}
	prov := observatory.Provenance{Source: observatory.SourceFWAppInit, FetchedAt: at, Reason: observatory.ReasonPrefer}
	if age >= fwapp.InitCacheTTL {
		prov.Stale = true
		prov.Reason = observatory.ReasonFallback
	}
	out["extras_provenance"] = prov
	return out
}

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
