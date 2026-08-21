package observatory

import (
	"context"
	"time"

	"fireproxy/pkg/inventory"
	"fireproxy/server/internal/fwapp"
)

// AlarmsView is the /v1/alarms bootstrap payload (without provenance).
type AlarmsView struct {
	ActiveAlarmCount int64               `json:"active_alarm_count"`
	NewAlarms        []fwapp.AlarmSample `json:"new_alarms"`
}

// Alarms resolves alarm count + newAlarms sample via agent catalog or fw-app init.
// Prefer agent path for active_alarm_count when catalog dashboard is fresh and agent online.
// Read-only bootstrap — no ignore/archive cmds.
func Alarms(ctx context.Context, deps Deps) (AlarmsView, Provenance, bool) {
	now := deps.now()
	agentAge, haveAgent, cat := alarmsCatalogAge(deps, now, CatalogTTL)

	if deps.AgentOnline && haveAgent && agentAge < CatalogTTL {
		return AlarmsView{
			ActiveAlarmCount: cat.Dashboard.AlarmCount,
			NewAlarms:        []fwapp.AlarmSample{},
		}, Provenance{Source: SourceAgent}, true
	}

	snap, at, initOK := peekInit(deps)
	prov, _ := Pick(deps.AgentOnline, agentAge, CatalogTTL, initOK, at)

	if prov.Source == SourceFWAppInit && initOK {
		return alarmsFromInit(snap), prov, true
	}

	if ensureInitOnce(ctx, deps) {
		snap, at, initOK = peekInit(deps)
		prov, _ = Pick(deps.AgentOnline, agentAge, CatalogTTL, initOK, at)
		if prov.Source == SourceFWAppInit && initOK {
			return alarmsFromInit(snap), prov, true
		}
	}

	return AlarmsView{}, Provenance{Source: SourceEmpty}, false
}

func alarmsCatalogAge(deps Deps, now time.Time, ttl time.Duration) (age time.Duration, have bool, cat inventory.Catalog) {
	age = ttl
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

func alarmsFromInit(snap fwapp.ObservatorySnapshot) AlarmsView {
	alarms := snap.NewAlarms
	if alarms == nil {
		alarms = []fwapp.AlarmSample{}
	}
	return AlarmsView{
		ActiveAlarmCount: snap.AlarmCount,
		NewAlarms:        alarms,
	}
}
