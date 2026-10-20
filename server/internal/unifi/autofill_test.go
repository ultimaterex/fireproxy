package unifi

import "testing"

func TestAutoFillEmptyOnly(t *testing.T) {
	prefs := &PrefsStore{}
	prefs.Set(Prefs{Enabled: true, Auto: true})
	var applied []NameRow
	AutoFillEmpty(prefs, func() ([]User, []FWHost, []string, error) {
		return []User{
			{ID: "u2", MAC: "02:00:00:00:00:02", Name: "old-nas"},
			{ID: "u3", MAC: "02:00:00:00:00:03", Hostname: "android-xx"},
		}, []FWHost{
			{MAC: "02:00:00:00:00:02", Name: "NAS"},
			{MAC: "02:00:00:00:00:03", Name: "phone"},
		}, nil, nil
	}, func(rows []NameRow) []ApplyResult {
		applied = rows
		out := make([]ApplyResult, len(rows))
		for i, r := range rows {
			out[i] = ApplyResult{MAC: r.MAC, OK: true}
		}
		return out
	})
	if len(applied) != 1 || applied[0].Status != "empty" || applied[0].MAC != "02:00:00:00:00:03" {
		t.Fatalf("%+v", applied)
	}
}

func TestAutoFillSkipsWhenBusy(t *testing.T) {
	prefs := &PrefsStore{}
	prefs.Set(Prefs{Enabled: true, Auto: true})
	unlock, ok := prefs.TryApply()
	if !ok {
		t.Fatal("lock")
	}
	defer unlock()
	called := false
	AutoFillEmpty(prefs, func() ([]User, []FWHost, []string, error) {
		t.Fatal("fetch")
		return nil, nil, nil, nil
	}, func(rows []NameRow) []ApplyResult {
		called = true
		return nil
	})
	if called {
		t.Fatal("apply should not run")
	}
}

func TestRefreshPendingCountsActive(t *testing.T) {
	prefs := &PrefsStore{}
	prefs.Set(Prefs{Enabled: true, Excluded: []string{"02:00:00:00:00:02"}})
	RefreshPending(prefs, func() ([]User, []FWHost, []string, error) {
		return []User{
			{ID: "u2", MAC: "02:00:00:00:00:02", Name: "old-nas"},
			{ID: "u3", MAC: "02:00:00:00:00:03", Hostname: "android-xx"},
		}, []FWHost{
			{MAC: "02:00:00:00:00:02", Name: "NAS"},
			{MAC: "02:00:00:00:00:03", Name: "phone"},
		}, nil, nil
	})
	if prefs.Pending() != 1 {
		t.Fatalf("pending=%d", prefs.Pending())
	}
}

func TestRefreshPendingClearsWhenDisabled(t *testing.T) {
	prefs := &PrefsStore{}
	prefs.Set(Prefs{Enabled: false})
	prefs.SetPending(9)
	RefreshPending(prefs, func() ([]User, []FWHost, []string, error) {
		t.Fatal("fetch")
		return nil, nil, nil, nil
	})
	if prefs.Pending() != 0 {
		t.Fatalf("pending=%d", prefs.Pending())
	}
}

func TestRefreshAudit(t *testing.T) {
	prefs := &PrefsStore{}
	prefs.Set(Prefs{Enabled: true})
	rep := BuildReport(ReportInput{
		VLAN:    []VLANRow{{MAC: "02:00:00:00:00:01"}, {MAC: "02:00:00:00:00:02"}},
		STP:     []STPRow{{MAC: "aa"}, {MAC: "bb"}, {MAC: "cc"}},
		Unknown: []UnknownRow{{MAC: "u1"}},
		Offline: []OfflineRow{{MAC: "o1"}, {MAC: "o2"}},
		Pending: []PendingDevice{{MAC: "p1"}},
	})
	RefreshAudit(prefs, func() ([]User, []FWHost, []string, error) {
		return []User{
			{ID: "u3", MAC: "02:00:00:00:00:03", Hostname: "android-xx"},
		}, []FWHost{
			{MAC: "02:00:00:00:00:03", Name: "phone"},
		}, nil, nil
	}, rep)
	c := prefs.AuditCounts()
	if c.Names != 1 || c.VLAN != 2 || c.STP != 3 || c.Unknown != 1 || c.Offline != 2 || c.Pending != 1 {
		t.Fatalf("counts=%+v", c)
	}
}
