package store

import (
	"path/filepath"
	"testing"
	"time"

	"fireproxy/pkg/agentws"
)

func TestServiceLogsInsertQueryPrune(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.db")
	p, err := OpenPersist(path, 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if p.LogRetentionDays() != 7 {
		t.Fatalf("default retention %d", p.LogRetentionDays())
	}
	now := time.Now().Unix()
	err = p.InsertLogLines([]agentws.LogLine{
		{TS: now - 10, Source: "unbound", Severity: "err", Line: "boom"},
		{TS: now - 5, Source: "dnsmasq", Severity: "warn", Line: "[ACL][Blocked] x"},
		{TS: now - 8*24*3600, Source: "firerouter", Severity: "err", Line: "old"},
	})
	if err != nil {
		t.Fatal(err)
	}
	all, err := p.QueryLogs(now-3600, now, "", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("got %d %+v", len(all), all)
	}
	only, err := p.QueryLogs(now-3600, now, "unbound", "boom", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(only) != 1 || only[0].Source != "unbound" {
		t.Fatalf("%+v", only)
	}
	if err := p.SetLogRetentionDays(1); err != nil {
		t.Fatal(err)
	}
	if p.LogRetentionDays() != 1 {
		t.Fatal("set failed")
	}
}
