package svclogs

import (
	"path/filepath"
	"strings"
	"testing"

	"fireproxy/pkg/agentws"
)

func TestCollectCapsAndMultiBatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	st, err := LoadCursorStore(path)
	if err != nil {
		t.Fatal(err)
	}
	st.Journal["unbound.service"] = "c0"
	st.Journal["firerouter.service"] = "c0"
	st.Journal["firerouter_dns.service"] = "c0"
	st.inited = true
	_ = st.Save()

	var msgs []string
	for i := 0; i < 250; i++ {
		msgs = append(msgs, `{"__CURSOR":"cX","MESSAGE":"error line `+strings.Repeat("x", 20)+`","PRIORITY":"3"}`)
	}
	stdout := strings.Join(msgs, "\n") + "\n"

	c := &Collector{
		Store: st,
		Journal: &JournalReader{Run: func(unit, after string, sinceDays, limit int) (string, string, error) {
			if unit != "unbound.service" {
				return "", after, nil
			}
			return stdout, "cX", nil
		}},
	}
	batches, pending, err := c.Collect(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 {
		t.Fatalf("batches %d", len(batches))
	}
	if !batches[1].Done {
		t.Fatal("last should be Done")
	}
	if len(batches[0].Lines) != MaxLinesPerBatch {
		t.Fatalf("batch0 lines %d", len(batches[0].Lines))
	}
	if pending.Journal["unbound.service"] != "cX" {
		t.Fatalf("%+v", pending)
	}
	if st.Journal["unbound.service"] != "c0" {
		t.Fatal("store advanced early")
	}
	if err := c.ApplyCursors(pending); err != nil {
		t.Fatal(err)
	}
	if st.Journal["unbound.service"] != "cX" {
		t.Fatal("apply failed")
	}
}

func TestCollectFirstBootSeeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	st, _ := LoadCursorStore(path)
	sawSince := false
	c := &Collector{
		Store:     st,
		SinceDays: 7,
		Journal: &JournalReader{Run: func(unit, after string, sinceDays, limit int) (string, string, error) {
			if after != "" && after != CursorNow {
				t.Fatalf("want seed after empty/now got %q", after)
			}
			if sinceDays != 7 {
				t.Fatalf("sinceDays %d", sinceDays)
			}
			sawSince = true
			return `{"__CURSOR":"seed1","MESSAGE":"error boom","PRIORITY":"3","__REALTIME_TIMESTAMP":"1000000"}` + "\n", "seed1", nil
		}},
	}
	batches, pending, err := c.Collect(5)
	if err != nil {
		t.Fatal(err)
	}
	if !sawSince {
		t.Fatal("expected seed read")
	}
	if len(batches) < 1 || len(batches[0].Lines) < 1 {
		t.Fatalf("%+v", batches)
	}
	if pending.Journal["unbound.service"] != "seed1" {
		t.Fatalf("%+v", pending.Journal)
	}
}

func TestCollectDropsObsoleteUnitKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	st, _ := LoadCursorStore(path)
	st.Journal["dnsmasq.service"] = "old"
	st.Journal["unbound.service"] = "c1"
	st.inited = true
	c := &Collector{
		Store: st,
		Journal: &JournalReader{Run: func(unit, after string, sinceDays, limit int) (string, string, error) {
			return "", after, nil
		}},
	}
	_, pending, err := c.Collect(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := pending.Journal["dnsmasq.service"]; ok {
		t.Fatal("obsolete key kept")
	}
}

func TestSplitBatchByteCap(t *testing.T) {
	big := agentws.LogLine{TS: 1, Source: "unbound", Severity: "err", Line: strings.Repeat("y", MaxBytesPerBatch/2)}
	lines := []agentws.LogLine{big, big, big}
	batches, dropped := splitBatches(lines, 5)
	if len(batches) < 2 {
		t.Fatalf("want multi batch got %d dropped=%d", len(batches), dropped)
	}
}

func TestParseFollowJSON(t *testing.T) {
	raw := `{"MESSAGE":"fatal fail","PRIORITY":"3","_SYSTEMD_UNIT":"unbound.service","__REALTIME_TIMESTAMP":"2000000"}`
	ln := parseFollowJSON(raw, DefaultUnits)
	if ln == nil || ln.Source != "unbound" {
		t.Fatalf("%+v", ln)
	}
	spam := `{"MESSAGE":"zero TTL record, return NULL","PRIORITY":"4","_SYSTEMD_UNIT":"firerouter_dns.service"}`
	if parseFollowJSON(spam, DefaultUnits) != nil {
		t.Fatal("expected denylist drop")
	}
}
