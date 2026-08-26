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

// Alarms resolves alarm count + samples via dual sources:
// 1) fw-app get item "alarms" when control is lan-ok (beats PreferInit),
// 2) else PreferInit / agent catalog / fw-app init Pick (existing semantics).
// On get failure, fall through to agent/init — do not empty if those are available.
func Alarms(ctx context.Context, deps Deps) (AlarmsView, Provenance, bool) {
	now := deps.now()

	if deps.ControlLANOK && deps.GetAlarms != nil {
		count, alarms, err := deps.GetAlarms(ctx)
		if err == nil {
			if alarms == nil {
				alarms = []fwapp.AlarmSample{}
			}
			return AlarmsView{
				ActiveAlarmCount: count,
				NewAlarms:        alarms,
			}, Provenance{Source: SourceFWAppGet, FetchedAt: now}, true
		}
		// fall through to PreferInit / agent / init
	}

	agentAge, haveAgent, cat := alarmsCatalogAge(deps, now, CatalogTTL)

	if deps.PreferInit {
		snap, _, prov, ok := takeInit(ctx, deps)
		if ok {
			return alarmsFromInit(snap), prov, true
		}
		return AlarmsView{}, Provenance{Source: SourceEmpty}, false
	}

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
