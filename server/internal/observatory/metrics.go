package observatory

import (
	"context"
	"strconv"
	"time"

	"fireproxy/pkg/snapshot"
	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/store"
)

// IngestTTL is the agent metrics ingest freshness window for MetricsLatest Pick.
const IngestTTL = 3 * time.Minute

// MetricsLatest resolves /v1/metrics/latest via agent ingest or fw-app init.
func MetricsLatest(ctx context.Context, deps Deps) (store.LatestView, Provenance, bool) {
	now := deps.now()
	agentAge, haveAgent, view := ingestAge(deps, now, IngestTTL)

	if deps.PreferInit {
		snap, at, prov, ok := takeInit(ctx, deps)
		if ok {
			view := metricsFromInit(snap, at)
			view, filled := gapFillMetrics(view, deps)
			return view, markEnriched(prov, filled), true
		}
		return store.LatestView{}, Provenance{Source: SourceEmpty}, false
	}

	if deps.AgentOnline && haveAgent && agentAge < IngestTTL {
		return view, Provenance{Source: SourceAgent}, true
	}

	snap, at, initOK := peekInit(deps)
	prov, _ := Pick(deps.AgentOnline, agentAge, IngestTTL, initOK, at)

	if prov.Source == SourceFWAppInit && initOK {
		v := metricsFromInit(snap, at)
		v, filled := gapFillMetrics(v, deps)
		return v, markEnriched(prov, filled), true
	}

	if ensureInitOnce(ctx, deps) {
		snap, at, initOK = peekInit(deps)
		prov, _ = Pick(deps.AgentOnline, agentAge, IngestTTL, initOK, at)
		if prov.Source == SourceFWAppInit && initOK {
			v := metricsFromInit(snap, at)
			v, filled := gapFillMetrics(v, deps)
			return v, markEnriched(prov, filled), true
		}
	}

	return store.LatestView{}, Provenance{Source: SourceEmpty}, false
}

func ingestAge(deps Deps, now time.Time, ttl time.Duration) (age time.Duration, have bool, view store.LatestView) {
	age = ttl
	if deps.Latest == nil {
		return age, false, store.LatestView{}
	}
	v, ok := deps.Latest()
	if !ok {
		return age, false, store.LatestView{}
	}
	age = now.Sub(time.Unix(v.Snapshot.TS, 0))
	if age < 0 {
		age = 0
	}
	return age, true, v
}

func metricsFromInit(obs fwapp.ObservatorySnapshot, at time.Time) store.LatestView {
	var ts int64
	if !at.IsZero() {
		ts = at.Unix()
	}
	snap := snapshot.Snapshot{
		TS:     ts,
		Ifaces: map[string]snapshot.IfaceStats{},
		WAN:    map[string]snapshot.WANLink{},
	}
	if obs.SysMetrics != nil {
		snap.Load = snapshot.Load{
			M1:  obs.SysMetrics.Load1,
			M5:  obs.SysMetrics.Load5,
			M15: obs.SysMetrics.Load15,
		}
		snap.CPU = obs.SysMetrics.CPU
		for _, d := range obs.SysMetrics.Disks {
			snap.Disks = append(snap.Disks, snapshot.Disk{
				Mount:      d.Mount,
				Filesystem: d.Filesystem,
				Capacity:   d.Capacity,
				Size:       d.Size,
				Used:       d.Used,
				Available:  d.Available,
			})
		}
	}
	for _, n := range obs.NICStates {
		st := snapshot.IfaceStats{
			Carrier: n.Carrier == "1" || n.Carrier == "true",
		}
		if mbps, err := strconv.Atoi(n.Speed); err == nil && mbps > 0 {
			st.SpeedMbps = &mbps
		}
		// RxBytes/TxBytes stay zero — do not invent counters or rates.
		snap.Ifaces[n.Name] = st
	}
	for iface, link := range obs.WAN {
		snap.WAN[iface] = link
	}
	for _, m := range obs.NICMetrics {
		snap.NICMetrics = append(snap.NICMetrics, snapshot.NICMetric{
			Name:     m.Name,
			RxMedian: m.RxMedian,
			TxMedian: m.TxMedian,
			RxPt90:   m.RxPt90,
			TxPt90:   m.TxPt90,
		})
	}
	// Resolver probes stay on dashboard.dns; do not synthesize dns_svcs process chips
	// from upstream IPs (that regresses Unbound/Dnsmasq UI under control fallback).
	return store.LatestView{
		Snapshot: snap,
		Rates:    map[string]store.Rates{},
		HavePrev: false,
	}
}
