package observatory

import (
	"fireproxy/pkg/snapshot"
	"fireproxy/server/internal/store"
)

// gapFillDashboard copies agent-only ranked fields into an init-backed view when
// the catalog is still within CatalogTTL. Shared fields (transfer, monthly,
// alarms, blocked, DNS queries) stay from init.
func gapFillDashboard(view DashboardView, deps Deps) (DashboardView, bool) {
	now := deps.now()
	age, have, cat := catalogAge(deps, now, CatalogTTL)
	if !have || age >= CatalogTTL || cat.Dashboard == nil {
		return view, false
	}
	dash := cat.Dashboard
	filled := false
	if len(view.TopUpload) == 0 && len(dash.TopUpload) > 0 {
		view.TopUpload = dash.TopUpload
		filled = true
	}
	if len(view.TopDownload) == 0 && len(dash.TopDownload) > 0 {
		view.TopDownload = dash.TopDownload
		filled = true
	}
	if len(view.TopDestUpload) == 0 && len(dash.TopDestUpload) > 0 {
		view.TopDestUpload = dash.TopDestUpload
		filled = true
	}
	if len(view.TopDestDownload) == 0 && len(dash.TopDestDownload) > 0 {
		view.TopDestDownload = dash.TopDestDownload
		filled = true
	}
	if len(view.TopRegions) == 0 && len(dash.TopRegions) > 0 {
		view.TopRegions = dash.TopRegions
		filled = true
	}
	if len(view.Speedtest) == 0 && len(dash.Speedtest) > 0 {
		view.Speedtest = dash.Speedtest
		filled = true
	}
	// DNS resolvers only if init had none (queries stay from init when present).
	if view.DNS == nil && dash.DNS != nil {
		view.DNS = dash.DNS
		filled = true
	} else if view.DNS != nil && dash.DNS != nil && len(view.DNS.Resolvers) == 0 && len(dash.DNS.Resolvers) > 0 {
		cp := *view.DNS
		cp.Resolvers = dash.DNS.Resolvers
		view.DNS = &cp
		filled = true
	}
	return view, filled
}

// gapFillMetrics copies agent-only live fields (rates, dns_svcs, unbound, iface
// byte counters) into an init-backed metrics view when ingest is within IngestTTL.
func gapFillMetrics(view store.LatestView, deps Deps) (store.LatestView, bool) {
	now := deps.now()
	age, have, agent := ingestAge(deps, now, IngestTTL)
	if !have || age >= IngestTTL {
		return view, false
	}
	filled := false
	if len(view.Rates) == 0 && len(agent.Rates) > 0 {
		view.Rates = agent.Rates
		view.HavePrev = agent.HavePrev
		filled = true
	}
	if view.UnboundHit == nil && agent.UnboundHit != nil {
		view.UnboundHit = agent.UnboundHit
		filled = true
	}
	as := agent.Snapshot
	if len(view.Snapshot.DNSSvcs) == 0 && len(as.DNSSvcs) > 0 {
		view.Snapshot.DNSSvcs = as.DNSSvcs
		filled = true
	}
	if view.Snapshot.Unbound == nil && as.Unbound != nil {
		view.Snapshot.Unbound = as.Unbound
		filled = true
	}
	if view.Snapshot.DNSRestarts == 0 && as.DNSRestarts != 0 {
		view.Snapshot.DNSRestarts = as.DNSRestarts
		filled = true
	}
	// Merge iface byte counters from agent onto init carrier/speed rows.
	if view.Snapshot.Ifaces == nil {
		view.Snapshot.Ifaces = map[string]snapshot.IfaceStats{}
	}
	for name, ag := range as.Ifaces {
		cur, ok := view.Snapshot.Ifaces[name]
		if !ok {
			view.Snapshot.Ifaces[name] = ag
			filled = true
			continue
		}
		if cur.RxBytes == 0 && cur.TxBytes == 0 && (ag.RxBytes != 0 || ag.TxBytes != 0) {
			cur.RxBytes = ag.RxBytes
			cur.TxBytes = ag.TxBytes
			view.Snapshot.Ifaces[name] = cur
			filled = true
		}
	}
	// WAN: keep init names/ready; only add agent WANs missing from init.
	if view.Snapshot.WAN == nil {
		view.Snapshot.WAN = map[string]snapshot.WANLink{}
	}
	for name, ag := range as.WAN {
		if _, ok := view.Snapshot.WAN[name]; !ok {
			view.Snapshot.WAN[name] = ag
			filled = true
		}
	}
	return view, filled
}

func markEnriched(prov Provenance, filled bool) Provenance {
	if filled {
		prov.EnrichedFrom = SourceAgent
	}
	return prov
}
