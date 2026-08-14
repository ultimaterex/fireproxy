package unifi

import "testing"

func TestNormalizeMAC(t *testing.T) {
	got := NormalizeMAC("30-52-53-0f-45-11")
	if got != "30:52:53:0F:45:11" {
		t.Fatalf("%q", got)
	}
	if NormalizeMAC("3052530f4511") != "30:52:53:0F:45:11" {
		t.Fatal("bare")
	}
}

func TestUsableName(t *testing.T) {
	mac := "30:52:53:0F:45:11"
	if usableName("", mac) || usableName(mac, mac) || usableName("3052530f4511", mac) {
		t.Fatal("mac-only must be unusable")
	}
	if !usableName("KVM02", mac) {
		t.Fatal("want usable")
	}
}

func TestDiffRows(t *testing.T) {
	rows := Diff(DiffInput{
		Firewalla: []FWHost{
			{MAC: "02:00:00:00:00:01", Name: "KVM02", IP: "203.0.113.20"},
			{MAC: "02:00:00:00:00:02", Name: "NAS", IP: "203.0.113.21"},
			{MAC: "02:00:00:00:00:03", Name: "phone"},
			{MAC: "02:00:00:00:00:04"},                // no usable Firewalla name
			{MAC: "02:00:00:00:00:aa", Name: "USW-1"}, // hardware
			{MAC: "02:00:00:00:00:99", Name: "ghost"}, // UniFi-unknown
		},
		Users: []User{
			{ID: "u1", MAC: "02:00:00:00:00:01", Name: "kvm02"}, // equal, case
			{ID: "u2", MAC: "02:00:00:00:00:02", Name: "old-nas"},
			{ID: "u3", MAC: "02:00:00:00:00:03", Hostname: "android-xx"},
			{ID: "u4", MAC: "02:00:00:00:00:aa", Name: "USW-1"},
		},
		Hardware: []string{"02:00:00:00:00:aa"},
	})
	if len(rows) != 2 {
		t.Fatalf("rows: %+v", rows)
	}
	if rows[0].MAC != "02:00:00:00:00:02" || rows[0].Status != "conflict" || rows[0].Firewalla != "NAS" || rows[0].UniFi != "old-nas" {
		t.Fatalf("conflict: %+v", rows[0])
	}
	if rows[1].MAC != "02:00:00:00:00:03" || rows[1].Status != "empty" || rows[1].UniFi != "android-xx" {
		t.Fatalf("empty: %+v", rows[1])
	}
}

func TestFirewallaPrefersLocalDomain(t *testing.T) {
	rows := Diff(DiffInput{
		Firewalla: []FWHost{{MAC: "02:00:00:00:00:05", LocalDomain: "cam", IP: "203.0.113.30"}},
		Users:     []User{{ID: "u5", MAC: "02:00:00:00:00:05"}},
	})
	if len(rows) != 1 || rows[0].Status != "empty" || rows[0].Firewalla != "cam" || rows[0].IP != "203.0.113.30" {
		t.Fatalf("%+v", rows)
	}
}

func TestDiffIPFallback(t *testing.T) {
	rows := Diff(DiffInput{
		Firewalla: []FWHost{{MAC: "02:00:00:00:00:06", Name: "tv"}},
		Users:     []User{{ID: "u6", MAC: "02:00:00:00:00:06", IP: "203.0.113.40"}},
	})
	if len(rows) != 1 || rows[0].IP != "203.0.113.40" {
		t.Fatalf("%+v", rows)
	}
}

func TestPartitionExcluded(t *testing.T) {
	rows := []NameRow{
		{MAC: "02:00:00:00:00:02", Status: "conflict", Firewalla: "NAS"},
		{MAC: "02:00:00:00:00:03", Status: "empty", Firewalla: "phone"},
	}
	active, ignored := PartitionExcluded(rows, []string{"02-00-00-00-00-02"})
	if len(active) != 1 || active[0].MAC != "02:00:00:00:00:03" {
		t.Fatalf("active %+v", active)
	}
	if len(ignored) != 1 || ignored[0].MAC != "02:00:00:00:00:02" {
		t.Fatalf("ignored %+v", ignored)
	}
}
