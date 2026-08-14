package svclogs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestJournalReadUnitFilters(t *testing.T) {
	stdout := "" +
		`{"__CURSOR":"c1","MESSAGE":"query A example.com","PRIORITY":"6","__REALTIME_TIMESTAMP":"1000000"}` + "\n" +
		`{"__CURSOR":"c2","MESSAGE":"fatal: network fail","PRIORITY":"3","__REALTIME_TIMESTAMP":"2000000"}` + "\n"
	j := &JournalReader{
		Run: func(unit, after string, sinceDays, limit int) (string, string, error) {
			if unit != "unbound.service" {
				t.Fatalf("unit %s", unit)
			}
			if after != CursorNow || sinceDays != 7 {
				t.Fatalf("after=%q since=%d", after, sinceDays)
			}
			return stdout, "c2", nil
		},
	}
	lines, cur, err := j.ReadUnit("unbound.service", CursorNow, 7)
	if err != nil {
		t.Fatal(err)
	}
	if cur != "c2" {
		t.Fatalf("cursor %q", cur)
	}
	if len(lines) != 1 || lines[0].Source != "unbound" || lines[0].Severity != "err" {
		t.Fatalf("%+v", lines)
	}
}

func TestJournalSkipUnknownUnit(t *testing.T) {
	j := &JournalReader{Run: func(string, string, int, int) (string, string, error) {
		t.Fatal("should not run")
		return "", "", nil
	}}
	lines, cur, err := j.ReadUnit("other.service", "x", 7)
	if err != nil || len(lines) != 0 || cur != "x" {
		t.Fatalf("%v %v %q", err, lines, cur)
	}
}

func TestCursorStoreFirstBootAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cursors.json")
	st, err := LoadCursorStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsFirstBoot() {
		t.Fatal("expected first boot")
	}
	st.InitNow(DefaultUnits)
	if st.Journal["unbound.service"] != CursorNow {
		t.Fatalf("%+v", st.Journal)
	}
	st.Journal["unbound.service"] = "c9"
	st.ACLOffset = 42
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	st2, err := LoadCursorStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if st2.IsFirstBoot() {
		t.Fatal("not first boot after save")
	}
	if st2.Journal["unbound.service"] != "c9" || st2.ACLOffset != 42 {
		t.Fatalf("%+v", st2)
	}
	raw, _ := os.ReadFile(path)
	var probe map[string]any
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
}
