package observatory

import (
	"context"
	"time"

	"fireproxy/server/internal/fwapp"
)

// VPNView is a read-only VPN / VIP / virt-WAN inventory from init (no agent equivalent yet).
type VPNView struct {
	TS             int64                       `json:"ts"`
	Host           string                      `json:"host,omitempty"`
	WGPeers        []fwapp.InitWGPeer          `json:"wg_peers,omitempty"`
	AWGPeers       []fwapp.InitWGPeer          `json:"awg_peers,omitempty"`
	WGClients      []fwapp.InitWGClient        `json:"wg_clients,omitempty"`
	ClientProfiles []fwapp.InitVPNClientFamily `json:"client_profiles,omitempty"`
	VIPs           []fwapp.InitVIP             `json:"vips,omitempty"`
	VirtWANs       []fwapp.InitVirtWAN         `json:"virt_wans,omitempty"`
}

// VPN resolves WireGuard / VIP / virt-WAN inventory from fw-app init.
// Agent path returns empty (no catalog fields yet) so PreferInit / offline init still works.
func VPN(ctx context.Context, deps Deps) (VPNView, Provenance, bool) {
	if deps.PreferInit {
		snap, at, prov, ok := takeInit(ctx, deps)
		if ok {
			return vpnFromInit(snap, at), prov, true
		}
		return VPNView{}, Provenance{Source: SourceEmpty}, false
	}

	// Prefer warm init whenever present — VPN inventory is init-primary.
	snap, at, initOK := peekInit(deps)
	if initOK {
		return vpnFromInit(snap, at), Provenance{Source: SourceFWAppInit, FetchedAt: at}, true
	}
	if ensureInitOnce(ctx, deps) {
		snap, at, initOK = peekInit(deps)
		if initOK {
			return vpnFromInit(snap, at), Provenance{Source: SourceFWAppInit, FetchedAt: at}, true
		}
	}
	return VPNView{}, Provenance{Source: SourceEmpty}, false
}

func vpnFromInit(snap fwapp.ObservatorySnapshot, at time.Time) VPNView {
	host := ""
	if snap.Box != nil {
		host = snap.Box.Name
	}
	var ts int64
	if !at.IsZero() {
		ts = at.Unix()
	}
	awg := snap.AWGPeers
	if awg == nil {
		awg = []fwapp.InitWGPeer{}
	}
	families := snap.ClientProfiles
	if families == nil {
		families = []fwapp.InitVPNClientFamily{}
	}
	return VPNView{
		TS:             ts,
		Host:           host,
		WGPeers:        snap.WGPeers,
		AWGPeers:       awg,
		WGClients:      snap.WGClients,
		ClientProfiles: families,
		VIPs:           snap.VIPs,
		VirtWANs:       snap.VirtWANs,
	}
}
