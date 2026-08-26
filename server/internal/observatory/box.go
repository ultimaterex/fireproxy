package observatory

import (
	"context"
	"time"

	"fireproxy/pkg/inventory"
	"fireproxy/server/internal/fwapp"
)

// BoxView is the /v1/box payload (without provenance).
type BoxView struct {
	TS   int64         `json:"ts"`
	Host string        `json:"host"`
	Box  inventory.Box `json:"box"`
}

// Box resolves on-box identity via agent catalog or fw-app init.
func Box(ctx context.Context, deps Deps) (BoxView, Provenance, bool) {
	now := deps.now()
	agentAge, haveAgent, cat := boxCatalogAge(deps, now, CatalogTTL)

	if deps.PreferInit {
		snap, at, prov, ok := takeInit(ctx, deps)
		if ok && snap.Box != nil {
			return boxFromInit(snap, at), prov, true
		}
		return BoxView{}, Provenance{Source: SourceEmpty}, false
	}

	if deps.AgentOnline && haveAgent && agentAge < CatalogTTL {
		return BoxView{TS: cat.TS, Host: cat.Host, Box: *cat.Box}, Provenance{Source: SourceAgent}, true
	}

	snap, at, initOK := peekInit(deps)
	initHasBox := initOK && snap.Box != nil
	prov, _ := Pick(deps.AgentOnline, agentAge, CatalogTTL, initHasBox, at)

	if prov.Source == SourceFWAppInit && initHasBox {
		return boxFromInit(snap, at), prov, true
	}

	if ensureInitOnce(ctx, deps) {
		snap, at, initOK = peekInit(deps)
		initHasBox = initOK && snap.Box != nil
		prov, _ = Pick(deps.AgentOnline, agentAge, CatalogTTL, initHasBox, at)
		if prov.Source == SourceFWAppInit && initHasBox {
			return boxFromInit(snap, at), prov, true
		}
	}

	if snap, at, ok := peekInitStale(deps); ok && snap.Box != nil {
		return boxFromInit(snap, at), staleInitProvenance(at), true
	}

	return BoxView{}, Provenance{Source: SourceEmpty}, false
}

func boxCatalogAge(deps Deps, now time.Time, ttl time.Duration) (age time.Duration, have bool, cat inventory.Catalog) {
	age = ttl
	if deps.Catalog == nil {
		return age, false, inventory.Catalog{}
	}
	c, ok := deps.Catalog()
	if !ok || c.Box == nil {
		return age, false, c
	}
	age = now.Sub(time.Unix(c.TS, 0))
	if age < 0 {
		age = 0
	}
	return age, true, c
}

func boxFromInit(snap fwapp.ObservatorySnapshot, at time.Time) BoxView {
	b := *snap.Box
	var ts int64
	if !at.IsZero() {
		ts = at.Unix()
	}
	return BoxView{TS: ts, Host: b.Name, Box: b}
}
