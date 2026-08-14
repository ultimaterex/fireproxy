package unifi

import "testing"

func TestBuildReportNamesMatchDiff(t *testing.T) {
	rep := BuildReport(ReportInput{
		NameEnabled: true,
		NameRows:    []NameRow{{MAC: "02:00:00:00:00:02", Firewalla: "NAS", Status: "conflict"}},
		VLAN:        []VLANRow{{MAC: "02:00:00:00:00:03", Name: "cam", FWVID: 20, UniFiVID: 1}},
		STP:         []STPRow{{MAC: "02:00:00:00:00:AA", Kind: "off"}},
		Unknown:     []UnknownRow{{MAC: "02:00:00:00:00:04", Side: "fw_only"}},
		Offline:     []OfflineRow{{MAC: "02:00:00:00:00:11", Kind: "ap"}},
		Pending:     []PendingDevice{{MAC: "02:00:00:00:00:BB", Model: "U6"}},
	})
	if rep.Names.Count != 1 || rep.VLAN.Count != 1 || rep.STP.Count != 1 {
		t.Fatalf("%+v", rep)
	}
	if rep.Unknown.Count != 1 || rep.Offline.Count != 1 || rep.Pending.Count != 1 {
		t.Fatalf("%+v", rep)
	}
}

func TestBuildReportEmptySlices(t *testing.T) {
	rep := BuildReport(ReportInput{})
	if rep.Names.Rows == nil || rep.VLAN.Rows == nil || rep.STP.Rows == nil {
		t.Fatal("nil rows")
	}
	if rep.Unknown.Rows == nil || rep.Offline.Rows == nil || rep.Pending.Rows == nil {
		t.Fatal("nil v2 rows")
	}
	if rep.Names.Count != 0 {
		t.Fatal("names")
	}
}

func TestSetAuditCounts(t *testing.T) {
	prefs := &PrefsStore{}
	prefs.Set(Prefs{Enabled: true})
	prefs.SetAuditCounts(AuditCountSet{Names: 4, VLAN: 2, STP: 3, Unknown: 5, Offline: 1, Pending: 6})
	c := prefs.AuditCounts()
	if c.Names != 4 || c.VLAN != 2 || c.STP != 3 || c.Unknown != 5 || c.Offline != 1 || c.Pending != 6 {
		t.Fatalf("counts=%+v", c)
	}
	if prefs.Pending() != 4 {
		t.Fatalf("pending=%d", prefs.Pending())
	}

	prefs.Set(Prefs{Enabled: false})
	prefs.SetAuditCounts(AuditCountSet{Names: 9, VLAN: 2, STP: 3, Unknown: 1, Offline: 1, Pending: 1})
	c = prefs.AuditCounts()
	if c.Names != 0 || c.VLAN != 2 || c.STP != 3 || c.Unknown != 1 {
		t.Fatalf("disabled counts=%+v", c)
	}
	if prefs.Pending() != 0 {
		t.Fatalf("disabled pending=%d", prefs.Pending())
	}

	prefs.Set(Prefs{Enabled: true})
	prefs.SetAuditCounts(AuditCountSet{Names: -1, VLAN: -2, STP: -3, Unknown: -1, Offline: -1, Pending: -1})
	c = prefs.AuditCounts()
	if c.Names != 0 || c.VLAN != 0 || c.STP != 0 || c.Unknown != 0 || c.Offline != 0 || c.Pending != 0 {
		t.Fatalf("clamped=%+v", c)
	}
}
