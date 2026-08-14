package unifi

import "log"

// AutoFillEmpty applies empty-status name rows when prefs allow. Skips if apply is in flight.
func AutoFillEmpty(prefs *PrefsStore, fetch func() ([]User, []FWHost, []string, error), apply func([]NameRow) []ApplyResult) {
	if prefs == nil {
		return
	}
	p := prefs.Get()
	if !p.Enabled || !p.Auto {
		return
	}
	unlock, ok := prefs.TryApply()
	if !ok {
		return
	}
	defer unlock()
	users, fw, hw, err := fetch()
	if err != nil {
		log.Printf("unifi name-sync auto: %v", err)
		return
	}
	all := Diff(DiffInput{Firewalla: fw, Users: users, Hardware: hw})
	active, _ := PartitionExcluded(all, p.Excluded)
	rows := SelectRows(active, nil, true)
	if len(rows) == 0 {
		return
	}
	for _, r := range apply(rows) {
		if !r.OK {
			log.Printf("unifi name-sync auto %s: %s", r.MAC, r.Error)
		}
	}
}

// RefreshPending recounts actionable name-sync rows into prefs.
func RefreshPending(prefs *PrefsStore, fetch func() ([]User, []FWHost, []string, error)) {
	if prefs == nil {
		return
	}
	p := prefs.Get()
	if !p.Enabled {
		prefs.SetPending(0)
		return
	}
	users, fw, hw, err := fetch()
	if err != nil {
		return
	}
	active, _ := PartitionExcluded(Diff(DiffInput{Firewalla: fw, Users: users, Hardware: hw}), p.Excluded)
	prefs.SetPending(len(active))
}

// RefreshAudit recounts name-sync pending, then caches all audit finding counts.
func RefreshAudit(prefs *PrefsStore, fetch func() ([]User, []FWHost, []string, error), rep Report) {
	RefreshPending(prefs, fetch)
	if prefs == nil {
		return
	}
	prefs.SetAuditCounts(AuditCountSet{
		Names:   prefs.Pending(),
		VLAN:    rep.VLAN.Count,
		STP:     rep.STP.Count,
		Unknown: rep.Unknown.Count,
		Offline: rep.Offline.Count,
		Pending: rep.Pending.Count,
	})
}
