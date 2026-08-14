package agenthub

import (
	"strings"

	"fireproxy/server/internal/store"
)

const offlineGapMinSec int64 = 5

// PlannedEvent is a lifecycle row to insert (no side effects).
type PlannedEvent struct {
	Kind, Detail, FromVer, ToVer string
}

// DecideHelloEvents plans offline/updated/restarted from durable state + hello.
// offlineTS == 0 means no offline/restarted handling (first hello / unset / server restart).
// Empty lastVer seeds on first hello with no updated event (caller sets KV).
// Version changes always emit updated, even when offline gap is missing or <5s
// (fast self-update / server recreate), without inventing an offline row.
func DecideHelloEvents(lastVer string, offlineTS int64, helloVer string, now int64) []PlannedEvent {
	helloVer = strings.TrimSpace(helloVer)
	lastVer = strings.TrimSpace(lastVer)

	versionChanged := lastVer != "" && helloVer != "" && helloVer != lastVer
	updated := PlannedEvent{
		Kind:    "updated",
		Detail:  lastVer + " → " + helloVer,
		FromVer: lastVer,
		ToVer:   helloVer,
	}

	if offlineTS <= 0 {
		if versionChanged {
			return []PlannedEvent{updated}
		}
		return nil
	}
	gap := now - offlineTS
	if gap < offlineGapMinSec {
		if versionChanged {
			return []PlannedEvent{updated}
		}
		return nil
	}

	out := []PlannedEvent{{
		Kind:   "offline",
		Detail: store.FormatOfflineDuration(gap),
	}}
	if lastVer == "" {
		return out
	}
	if versionChanged {
		out = append(out, updated)
		return out
	}
	out = append(out, PlannedEvent{Kind: "restarted"})
	return out
}
