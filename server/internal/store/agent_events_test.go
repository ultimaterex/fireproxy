package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAgentEventsInsertQueryPrune(t *testing.T) {
	dir := t.TempDir()
	p, err := OpenPersist(filepath.Join(dir, "t.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	old := time.Now().Unix() - 400*24*3600
	if err := p.InsertAgentEvent(AgentEvent{TS: old, Kind: "restarted"}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	if err := p.InsertAgentEvent(AgentEvent{TS: now, Kind: "updated", FromVer: "0.1.6", ToVer: "0.1.7", Detail: "0.1.6 → 0.1.7"}); err != nil {
		t.Fatal(err)
	}
	if err := p.PruneAgentEvents(); err != nil {
		t.Fatal(err)
	}
	rows, err := p.QueryAgentEvents(0, 10)
	if err != nil || len(rows) != 1 || rows[0].Kind != "updated" {
		t.Fatalf("%+v err=%v", rows, err)
	}
	page, err := p.QueryAgentEvents(rows[0].ID, 10)
	if err != nil || len(page) != 0 {
		t.Fatalf("before_id page %+v", page)
	}
}

func TestAgentEventKV(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "t.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if v, _ := p.AgentLastVersion(); v != "" {
		t.Fatalf("%q", v)
	}
	_ = p.SetAgentLastVersion("0.1.6")
	_ = p.SetAgentLastOfflineTS(100)
	v, _ := p.AgentLastVersion()
	ts, _ := p.AgentLastOfflineTS()
	if v != "0.1.6" || ts != 100 {
		t.Fatalf("%q %d", v, ts)
	}
	_ = p.ClearAgentLastOfflineTS()
	ts, _ = p.AgentLastOfflineTS()
	if ts != 0 {
		t.Fatalf("%d", ts)
	}
}

func TestFormatOfflineDuration(t *testing.T) {
	cases := []struct {
		sec  int64
		want string
	}{
		{30, "1m"},
		{60, "1m"},
		{119, "2m"},
		{3600, "1h"},
		{7200, "2h"},
		{86400, "1d"},
		{90000, "1d"},
	}
	for _, tc := range cases {
		if got := FormatOfflineDuration(tc.sec); got != tc.want {
			t.Fatalf("sec=%d got %q want %q", tc.sec, got, tc.want)
		}
	}
}
