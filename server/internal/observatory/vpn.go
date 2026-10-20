package observatory

import (
	"context"
	"time"

	"fireproxy/server/internal/fwapp"
)

// VPNView is a read-only VPN / VIP / virt-WAN inventory from init (no agent equivalent yet).
type VPNView struct {
	TS        int64                `json:"ts"`
	Host      string               `json:"host,omitempty"`
	WGPeers   []fwapp.InitWGPeer   `json:"wg_peers,omitempty"`
	WGClients []fwapp.InitWGClient `json:"wg_clients,omitempty"`
	VIPs      []fwapp.InitVIP      `json:"vips,omitempty"`
	VirtWANs  []fwapp.InitVirtWAN  `json:"virt_wans,omitempty"`
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
			return vpnFromInit(snap, at), Provenance{Source: SourceFWAppInit, FetchedAt: at, Reason: ReasonFallback}, true
		}
	}
	if snap, at, ok := peekInitStale(deps); ok {
		return vpnFromInit(snap, at), staleInitProvenance(at), true
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
	return VPNView{
		TS:        ts,
		Host:      host,
		WGPeers:   snap.WGPeers,
		WGClients: snap.WGClients,
		VIPs:      snap.VIPs,
		VirtWANs:  snap.VirtWANs,
	}
}
