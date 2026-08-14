package svclogs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"fireproxy/pkg/agentws"
)

// DefaultUnits maps journal unit → source label (Firewalla-real names).
var DefaultUnits = map[string]string{
	"unbound.service":        "unbound",
	"firerouter.service":     "firerouter",
	"firerouter_dns.service": "dnsmasq",
}

// CursorNow is a legacy sentinel meaning “not seeded yet”.
const CursorNow = "now"

// DefaultSeedDays is used when Collect.SinceDays is unset.
const DefaultSeedDays = 7

// JournalRunner runs one journalctl-style read.
// afterCursor empty or CursorNow → use sinceDays (seed) via --since; else --after-cursor.
type JournalRunner func(unit, afterCursor string, sinceDays, limit int) (stdout string, newCursor string, err error)

// JournalReader pulls quiet lines from pinned units via an injectable runner.
type JournalReader struct {
	Units map[string]string // unit → source
	Run   JournalRunner
	Limit int // per-unit line fetch before filter; default 200
}

type journalJSON struct {
	Cursor   string `json:"__CURSOR"`
	Message  string `json:"MESSAGE"`
	Priority string `json:"PRIORITY"`
	Realtime string `json:"__REALTIME_TIMESTAMP"`
}

// ReadUnit returns filtered lines and the latest cursor seen (or old cursor if none).
// sinceDays is used when afterCursor is empty or CursorNow (seed / first read).
func (j *JournalReader) ReadUnit(unit, afterCursor string, sinceDays int) (lines []agentws.LogLine, newCursor string, err error) {
	if j == nil || j.Run == nil {
		return nil, afterCursor, fmt.Errorf("journal runner unset")
	}
	units := j.Units
	if units == nil {
		units = DefaultUnits
	}
	source, ok := units[unit]
	if !ok {
		return nil, afterCursor, nil // skip unknown
	}
	limit := j.Limit
	if limit <= 0 {
		limit = 200
	}
	if sinceDays <= 0 {
		sinceDays = DefaultSeedDays
	}
	cur := afterCursor
	stdout, gotCursor, err := j.Run(unit, cur, sinceDays, limit)
	if err != nil {
		return nil, afterCursor, err
	}
	newCursor = afterCursor
	if gotCursor != "" {
		newCursor = gotCursor
	}
	sc := bufio.NewScanner(strings.NewReader(stdout))
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var row journalJSON
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		if row.Cursor != "" {
			newCursor = row.Cursor
		}
		sev := priorityToSev(row.Priority)
		msg := row.Message
		if msg == "" {
			continue
		}
		if !QuietKeep(sev, msg) {
			continue
		}
		ts := time.Now().Unix()
		if row.Realtime != "" {
			// journal realtime is µs since epoch
			var us int64
			if _, err := fmt.Sscan(row.Realtime, &us); err == nil && us > 0 {
				ts = us / 1_000_000
			}
		}
		lines = append(lines, agentws.LogLine{
			TS:       ts,
			Source:   source,
			Severity: sev,
			Line:     msg,
		})
	}
	return lines, newCursor, sc.Err()
}

func priorityToSev(p string) string {
	switch strings.TrimSpace(p) {
	case "0", "1", "2", "3":
		return "err"
	case "4":
		return "warning"
	default:
		return "info"
	}
}
