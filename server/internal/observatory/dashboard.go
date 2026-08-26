package observatory

import (
	"context"
	"time"

	"fireproxy/pkg/inventory"
	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/store"
)

// CatalogTTL is the agent catalog freshness window for dashboard Pick.
const CatalogTTL = time.Hour

// Deps wires ranked backends for observatory facades.
type Deps struct {
	Now         time.Time // zero → time.Now()
	AgentOnline bool
	// PreferInit forces fw-app init when warm (or after one EnsureInit), even if
	// the agent catalog/ingest is still within TTL. Set after an explicit control refresh.
	PreferInit bool

	Catalog func() (inventory.Catalog, bool)
	Latest  func() (store.LatestView, bool)

	ObservatorySnapshot func() (fwapp.ObservatorySnapshot, time.Time, bool)
	EnsureInit          func(ctx context.Context) error
}

func (d Deps) now() time.Time {
	if d.Now.IsZero() {
		return time.Now()
	}
	return d.Now
}

// DashboardView is the /v1/dashboard payload (without provenance).
type DashboardView struct {
	TS              int64                    `json:"ts"`
	Host            string                   `json:"host"`
	Devices         int                      `json:"devices"`
	Rules           int                      `json:"rules"`
	AlarmCount      int64                    `json:"alarm_count"`
	Transfer24h     inventory.Transfer       `json:"transfer_24h"`
	Transfer30d     inventory.Transfer       `json:"transfer_30d,omitempty"`
	Transfer60      inventory.Transfer       `json:"transfer_60,omitempty"`
	Transfer12m     inventory.Transfer       `json:"transfer_12m,omitempty"`
	MonthlyWANs     []inventory.WANUsage     `json:"monthly_wans"`
	MonthlyBeginTS  int64                    `json:"monthly_begin_ts,omitempty"`
	MonthlyEndTS    int64                    `json:"monthly_end_ts,omitempty"`
	Blocked         inventory.BlockedMix     `json:"blocked"`
	TopUpload       []inventory.RankedFlow   `json:"top_upload"`
	TopDownload     []inventory.RankedFlow   `json:"top_download"`
	TopDestUpload   []inventory.RankedFlow   `json:"top_dest_upload"`
	TopDestDownload []inventory.RankedFlow   `json:"top_dest_download"`
	TopRegions      []inventory.RankedFlow   `json:"top_regions"`
	Speedtest       []inventory.SpeedtestWAN `json:"speedtest"`
	DNS             *inventory.DNSHealth     `json:"dns"`
}

// Dashboard resolves MSP dashboard rollups via agent catalog or fw-app init.
func Dashboard(ctx context.Context, deps Deps) (DashboardView, Provenance, bool) {
	now := deps.now()
	agentAge, haveAgent, cat := catalogAge(deps, now, CatalogTTL)

	if deps.PreferInit {
		snap, at, prov, ok := takeInit(ctx, deps)
		if ok {
			view := dashboardFromInit(snap, at)
			view, filled := gapFillDashboard(view, deps)
			return view, markEnriched(prov, filled), true
		}
		return DashboardView{}, Provenance{Source: SourceEmpty}, false
	}

	// Prefer agent without touching InitCache when online + fresh.
	if deps.AgentOnline && haveAgent && agentAge < CatalogTTL {
		return dashboardFromCatalog(cat), Provenance{Source: SourceAgent}, true
	}

	snap, at, initOK := peekInit(deps)
	prov, _ := Pick(deps.AgentOnline, agentAge, CatalogTTL, initOK, at)

	if prov.Source == SourceFWAppInit && initOK {
		view := dashboardFromInit(snap, at)
		view, filled := gapFillDashboard(view, deps)
		return view, markEnriched(prov, filled), true
	}

	// Cold init: one bounded on-demand fetch when paired/LAN-OK, then re-resolve.
	if ensureInitOnce(ctx, deps) {
		snap, at, initOK = peekInit(deps)
		prov, _ = Pick(deps.AgentOnline, agentAge, CatalogTTL, initOK, at)
		if prov.Source == SourceFWAppInit && initOK {
			view := dashboardFromInit(snap, at)
			view, filled := gapFillDashboard(view, deps)
			return view, markEnriched(prov, filled), true
		}
	}

	return DashboardView{}, Provenance{Source: SourceEmpty}, false
}

func catalogAge(deps Deps, now time.Time, ttl time.Duration) (age time.Duration, have bool, cat inventory.Catalog) {
	age = ttl // missing catalog → treat as stale for Pick
	if deps.Catalog == nil {
		return age, false, inventory.Catalog{}
	}
	c, ok := deps.Catalog()
	if !ok || c.Dashboard == nil {
		return age, false, c
	}
	age = now.Sub(time.Unix(c.TS, 0))
	if age < 0 {
		age = 0
	}
	return age, true, c
}

func peekInit(deps Deps) (fwapp.ObservatorySnapshot, time.Time, bool) {
	if deps.ObservatorySnapshot == nil {
		return fwapp.ObservatorySnapshot{}, time.Time{}, false
	}
	snap, at, ok := deps.ObservatorySnapshot()
	if !ok {
		return fwapp.ObservatorySnapshot{}, time.Time{}, false
	}
	// Enforce InitCacheTTL so facades refresh via EnsureInit instead of serving
	// an arbitrarily old snapshot after the first successful init.
	now := deps.now()
	if at.IsZero() || now.Sub(at) >= fwapp.InitCacheTTL {
		return snap, at, false
	}
	return snap, at, true
}

func ensureInitOnce(ctx context.Context, deps Deps) bool {
	if deps.EnsureInit == nil {
		return false
	}
	if err := deps.EnsureInit(ctx); err != nil {
		return false
	}
	return true
}

// takeInit returns a warm observatory snapshot, ensuring init once if needed.
func takeInit(ctx context.Context, deps Deps) (fwapp.ObservatorySnapshot, time.Time, Provenance, bool) {
	snap, at, initOK := peekInit(deps)
	if !initOK && ensureInitOnce(ctx, deps) {
		snap, at, initOK = peekInit(deps)
	}
	if !initOK {
		return fwapp.ObservatorySnapshot{}, time.Time{}, Provenance{Source: SourceEmpty}, false
	}
	return snap, at, Provenance{Source: SourceFWAppInit, FetchedAt: at}, true
}

func dashboardFromCatalog(cat inventory.Catalog) DashboardView {
	dash := *cat.Dashboard
	return DashboardView{
		TS:              cat.TS,
		Host:            cat.Host,
		Devices:         len(cat.Devices),
		Rules:           len(cat.Policies),
		AlarmCount:      dash.AlarmCount,
		Transfer24h:     dash.Transfer24h,
		MonthlyWANs:     dash.MonthlyWANs,
		Blocked:         dash.Blocked,
		TopUpload:       dash.TopUpload,
		TopDownload:     dash.TopDownload,
		TopDestUpload:   dash.TopDestUpload,
		TopDestDownload: dash.TopDestDownload,
		TopRegions:      dash.TopRegions,
		Speedtest:       dash.Speedtest,
		DNS:             dash.DNS,
	}
}

func dashboardFromInit(snap fwapp.ObservatorySnapshot, at time.Time) DashboardView {
	host := ""
	if snap.Box != nil {
		host = snap.Box.Name
	}
	var ts int64
	if !at.IsZero() {
		ts = at.Unix()
	}
	return DashboardView{
		TS:             ts,
		Host:           host,
		Devices:        len(snap.Devices),
		Rules:          snap.RuleCount,
		AlarmCount:     snap.AlarmCount,
		Transfer24h:    snap.Transfer24h,
		Transfer30d:    snap.Transfer30d,
		Transfer60:     snap.Transfer60,
		Transfer12m:    snap.Transfer12m,
		MonthlyWANs:    snap.MonthlyWANs,
		MonthlyBeginTS: snap.MonthlyBeginTS,
		MonthlyEndTS:   snap.MonthlyEndTS,
		Blocked:        snap.Blocked,
		DNS:            snap.DNS,
		// top_* intentionally empty — init has no ranked flows
		Speedtest: snap.Speedtest,
	}
}
