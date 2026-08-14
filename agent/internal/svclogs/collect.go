package svclogs

import (
	"fmt"
	"sort"

	"fireproxy/pkg/agentws"
)

const (
	MaxLinesPerBatch        = 200
	MaxBytesPerBatch        = 64 * 1024
	MaxBatchesPerInvocation = 5
	ACLTailBytes            = 256 * 1024
)

// Collector gathers quiet journal + ACL lines into capped batches.
type Collector struct {
	Journal   *JournalReader
	ACL       *ACLFileReader
	Store     *CursorStore
	Units     map[string]string
	SinceDays int // seed window; default DefaultSeedDays
}

// Collect reads new lines, returns batches (Done on last). Does not advance Store;
// caller advances after ack using ApplyCursors.
func (c *Collector) Collect(maxBatches int) (batches []agentws.LogsBatch, pending *PendingCursors, err error) {
	if maxBatches <= 0 {
		maxBatches = MaxBatchesPerInvocation
	}
	units := c.Units
	if units == nil {
		units = DefaultUnits
	}
	sinceDays := c.SinceDays
	if sinceDays <= 0 {
		sinceDays = DefaultSeedDays
	}

	pending = &PendingCursors{Journal: map[string]string{}}
	if c.Store != nil {
		pending.ACLOffset = c.Store.ACLOffset
		for k, v := range c.Store.Journal {
			// drop obsolete unit keys (e.g. dnsmasq.service)
			if _, ok := units[k]; ok {
				pending.Journal[k] = v
			}
		}
	}

	var all []agentws.LogLine

	if c.Journal != nil {
		names := make([]string, 0, len(units))
		for u := range units {
			names = append(names, u)
		}
		sort.Strings(names)
		for _, unit := range names {
			after := ""
			if c.Store != nil {
				after = c.Store.Journal[unit]
			}
			lines, cur, rerr := c.Journal.ReadUnit(unit, after, sinceDays)
			if rerr != nil {
				continue
			}
			all = append(all, lines...)
			if cur != "" && cur != CursorNow {
				pending.Journal[unit] = cur
			} else if after != "" && after != CursorNow {
				pending.Journal[unit] = after
			}
		}
	}

	if c.ACL != nil {
		off := int64(0)
		if c.Store != nil {
			off = c.Store.ACLOffset
		}
		c.ACL.Offset = off
		lines, newOff, rerr := c.ACL.ReadNotable(MaxLinesPerBatch)
		if rerr == nil {
			all = append(all, lines...)
			pending.ACLOffset = newOff
		}
	}

	if c.Store != nil {
		c.Store.inited = true
	}

	sort.SliceStable(all, func(i, j int) bool { return all[i].TS < all[j].TS })

	batches, dropped := splitBatches(all, maxBatches)
	if len(batches) == 0 {
		batches = []agentws.LogsBatch{{ID: "b0", Done: true}}
	} else {
		for i := range batches {
			batches[i].ID = fmt.Sprintf("b%d", i)
			batches[i].Done = i == len(batches)-1
		}
		batches[len(batches)-1].Dropped = dropped
	}
	return batches, pending, nil
}

// PendingCursors is applied to the store after all acks for an invocation.
type PendingCursors struct {
	Journal   map[string]string
	ACLOffset int64
}

func (c *Collector) ApplyCursors(p *PendingCursors) error {
	if c == nil || c.Store == nil || p == nil {
		return nil
	}
	if c.Store.Journal == nil {
		c.Store.Journal = map[string]string{}
	}
	// replace with pending (drops obsolete keys)
	c.Store.Journal = copyMap(p.Journal)
	c.Store.ACLOffset = p.ACLOffset
	c.Store.inited = true
	return c.Store.Save()
}

func splitBatches(lines []agentws.LogLine, maxBatches int) ([]agentws.LogsBatch, int) {
	var batches []agentws.LogsBatch
	var cur []agentws.LogLine
	bytes := 0
	dropped := 0
	flush := func() {
		if len(cur) == 0 {
			return
		}
		batches = append(batches, agentws.LogsBatch{Lines: cur})
		cur = nil
		bytes = 0
	}
	for _, ln := range lines {
		cost := len(ln.Line) + 32
		if len(cur) >= MaxLinesPerBatch || (len(cur) > 0 && bytes+cost > MaxBytesPerBatch) {
			if len(batches)+1 >= maxBatches {
				dropped++
				continue
			}
			flush()
		}
		if len(batches) >= maxBatches {
			dropped++
			continue
		}
		if len(cur) >= MaxLinesPerBatch || bytes+cost > MaxBytesPerBatch {
			if len(batches)+1 >= maxBatches {
				dropped++
				continue
			}
			flush()
		}
		cur = append(cur, ln)
		bytes += cost
	}
	if len(batches) < maxBatches {
		flush()
	} else if len(cur) > 0 {
		dropped += len(cur)
	}
	return batches, dropped
}

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
