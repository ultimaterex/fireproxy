package observatory

import (
	"context"
	"time"

	"fireproxy/pkg/inventory"
	"fireproxy/server/internal/fwapp"
)

// DevicesView is the /v1/devices payload (without provenance).
type DevicesView struct {
	TS      int64              `json:"ts"`
	Host    string             `json:"host"`
	Devices []inventory.Device `json:"devices"`
}

// Devices resolves the device list via agent catalog or fw-app init.
// UniFi overlay is applied by the API only on the agent/catalog path.
func Devices(ctx context.Context, deps Deps) (DevicesView, Provenance, bool) {
	now := deps.now()
	agentAge, haveAgent, cat := devicesCatalogAge(deps, now, CatalogTTL)

	if deps.PreferInit {
		snap, at, prov, ok := takeInit(ctx, deps)
		if ok {
			return devicesFromInit(snap, at), prov, true
		}
		return DevicesView{}, Provenance{Source: SourceEmpty}, false
	}

	if deps.AgentOnline && haveAgent && agentAge < CatalogTTL {
		return DevicesView{TS: cat.TS, Host: cat.Host, Devices: cat.Devices}, Provenance{Source: SourceAgent}, true
	}

	snap, at, initOK := peekInit(deps)
	prov, _ := Pick(deps.AgentOnline, agentAge, CatalogTTL, initOK, at)

	if prov.Source == SourceFWAppInit && initOK {
		return devicesFromInit(snap, at), prov, true
	}

	if ensureInitOnce(ctx, deps) {
		snap, at, initOK = peekInit(deps)
		prov, _ = Pick(deps.AgentOnline, agentAge, CatalogTTL, initOK, at)
		if prov.Source == SourceFWAppInit && initOK {
			return devicesFromInit(snap, at), prov, true
		}
	}

	return DevicesView{}, Provenance{Source: SourceEmpty}, false
}

func devicesCatalogAge(deps Deps, now time.Time, ttl time.Duration) (age time.Duration, have bool, cat inventory.Catalog) {
	age = ttl
	if deps.Catalog == nil {
		return age, false, inventory.Catalog{}
	}
	c, ok := deps.Catalog()
	if !ok {
		return age, false, c
	}
	age = now.Sub(time.Unix(c.TS, 0))
	if age < 0 {
		age = 0
	}
	return age, true, c
}

func devicesFromInit(snap fwapp.ObservatorySnapshot, at time.Time) DevicesView {
	host := ""
	if snap.Box != nil {
		host = snap.Box.Name
	}
	var ts int64
	if !at.IsZero() {
		ts = at.Unix()
	}
	devs := snap.Devices
	if devs == nil {
		devs = []inventory.Device{}
	}
	return DevicesView{TS: ts, Host: host, Devices: devs}
}
