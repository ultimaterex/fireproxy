package observatory

import (
	"context"
	"time"

	"fireproxy/pkg/inventory"
	"fireproxy/server/internal/fwapp"
)

// TagsView is the /v1/tags payload (without provenance).
type TagsView struct {
	TS   int64           `json:"ts"`
	Host string          `json:"host"`
	Tags []inventory.Tag `json:"tags"`
}

// Tags resolves group/user/device tags via agent catalog or fw-app init.
func Tags(ctx context.Context, deps Deps) (TagsView, Provenance, bool) {
	now := deps.now()
	agentAge, haveAgent, cat := tagsCatalogAge(deps, now, CatalogTTL)

	if deps.PreferInit {
		snap, at, prov, ok := takeInit(ctx, deps)
		if ok {
			return tagsFromInit(snap, at), prov, true
		}
		return TagsView{}, Provenance{Source: SourceEmpty}, false
	}

	if deps.AgentOnline && haveAgent && agentAge < CatalogTTL {
		return TagsView{TS: cat.TS, Host: cat.Host, Tags: cat.Tags}, Provenance{Source: SourceAgent}, true
	}

	snap, at, initOK := peekInit(deps)
	prov, _ := Pick(deps.AgentOnline, agentAge, CatalogTTL, initOK, at)

	if prov.Source == SourceFWAppInit && initOK {
		return tagsFromInit(snap, at), prov, true
	}

	if ensureInitOnce(ctx, deps) {
		snap, at, initOK = peekInit(deps)
		prov, _ = Pick(deps.AgentOnline, agentAge, CatalogTTL, initOK, at)
		if prov.Source == SourceFWAppInit && initOK {
			return tagsFromInit(snap, at), prov, true
		}
	}

	return TagsView{}, Provenance{Source: SourceEmpty}, false
}

func tagsCatalogAge(deps Deps, now time.Time, ttl time.Duration) (age time.Duration, have bool, cat inventory.Catalog) {
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

func tagsFromInit(snap fwapp.ObservatorySnapshot, at time.Time) TagsView {
	host := ""
	if snap.Box != nil {
		host = snap.Box.Name
	}
	var ts int64
	if !at.IsZero() {
		ts = at.Unix()
	}
	tags := snap.Tags
	if tags == nil {
		tags = []inventory.Tag{}
	}
	return TagsView{TS: ts, Host: host, Tags: tags}
}
