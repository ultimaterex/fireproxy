package svclogs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"fireproxy/pkg/agentws"
)

const (
	LiveRatePerSec   = 20
	LiveFlushEvery   = 500 * time.Millisecond
	LiveACLPollEvery = time.Second
)

// BatchSink receives live log batches (Done=false for stream chunks).
type BatchSink func(batch agentws.LogsBatch) error

// Follow runs journalctl -f + ACL polling until Stop or ctx cancel.
type Follow struct {
	Units      map[string]string
	ACL        *ACLFileReader
	Store      *CursorStore
	Sink       BatchSink
	RatePerSec int

	mu     sync.Mutex
	cancel context.CancelFunc
	active bool
}

// Start begins follow if not already running.
func (f *Follow) Start(parent context.Context) {
	f.mu.Lock()
	if f.active {
		f.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	f.cancel = cancel
	f.active = true
	f.mu.Unlock()
	go f.run(ctx)
}

// Stop ends follow.
func (f *Follow) Stop() {
	f.mu.Lock()
	cancel := f.cancel
	f.cancel = nil
	f.active = false
	f.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Active reports whether follow is running.
func (f *Follow) Active() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.active
}

func (f *Follow) run(ctx context.Context) {
	defer func() {
		f.mu.Lock()
		f.active = false
		f.mu.Unlock()
	}()

	units := f.Units
	if units == nil {
		units = DefaultUnits
	}
	rate := f.RatePerSec
	if rate <= 0 {
		rate = LiveRatePerSec
	}

	args := []string{"-f", "--no-pager", "-o", "json", "-n", "0"}
	for u := range units {
		args = append(args, "-u", u)
	}
	cmd := exec.CommandContext(ctx, "journalctl", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	st := &liveState{rate: rate}
	lineCh := make(chan agentws.LogLine, 256)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		sc := bufio.NewScanner(stdout)
		buf := make([]byte, 0, 64*1024)
		sc.Buffer(buf, 1024*1024)
		for sc.Scan() {
			if ln := parseFollowJSON(sc.Text(), units); ln != nil {
				select {
				case lineCh <- *ln:
				case <-ctx.Done():
					return
				}
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	ticker := time.NewTicker(LiveFlushEvery)
	defer ticker.Stop()
	aclTick := time.NewTicker(LiveACLPollEvery)
	defer aclTick.Stop()

	for {
		select {
		case <-ctx.Done():
			f.emit(st)
			return
		case <-readDone:
			f.emit(st)
			return
		case ln := <-lineCh:
			st.accept(ln)
			if len(st.buf) >= 50 {
				f.emit(st)
			}
		case <-ticker.C:
			f.emit(st)
		case <-aclTick.C:
			f.pollACL(st)
		}
	}
}

type liveState struct {
	buf     []agentws.LogLine
	dropped int
	seq     int
	rate    int
	window  time.Time
	count   int
	aclOff  int64
	hasACL  bool
}

func (s *liveState) accept(ln agentws.LogLine) {
	now := time.Now()
	if s.window.IsZero() || now.Sub(s.window) >= time.Second {
		s.window = now
		s.count = 0
	}
	if s.count >= s.rate {
		s.dropped++
		return
	}
	s.count++
	s.buf = append(s.buf, ln)
}

func (f *Follow) emit(st *liveState) {
	if len(st.buf) == 0 {
		return
	}
	st.seq++
	b := agentws.LogsBatch{
		ID:      fmt.Sprintf("live-%d", st.seq),
		Lines:   st.buf,
		Dropped: st.dropped,
		Done:    false,
	}
	st.buf = nil
	st.dropped = 0
	aclOff := st.aclOff
	hasACL := st.hasACL
	if f.Sink != nil {
		if err := f.Sink(b); err != nil {
			return
		}
	}
	if hasACL && f.Store != nil {
		f.Store.ACLOffset = aclOff
		_ = f.Store.Save()
	}
}

func (f *Follow) pollACL(st *liveState) {
	if f.ACL == nil {
		return
	}
	if f.Store != nil {
		f.ACL.Offset = f.Store.ACLOffset
	}
	lines, off, err := f.ACL.ReadNotable(50)
	if err != nil || len(lines) == 0 {
		return
	}
	for _, ln := range lines {
		st.accept(ln)
	}
	st.aclOff = off
	st.hasACL = true
}

func parseFollowJSON(raw string, units map[string]string) *agentws.LogLine {
	var row struct {
		Message  string `json:"MESSAGE"`
		Priority string `json:"PRIORITY"`
		Realtime string `json:"__REALTIME_TIMESTAMP"`
		Unit     string `json:"_SYSTEMD_UNIT"`
	}
	if json.Unmarshal([]byte(raw), &row) != nil || row.Message == "" {
		return nil
	}
	sev := priorityToSev(row.Priority)
	if !QuietKeep(sev, row.Message) {
		return nil
	}
	source, ok := units[row.Unit]
	if !ok || source == "" {
		return nil
	}
	ts := time.Now().Unix()
	if row.Realtime != "" {
		var us int64
		if _, err := fmt.Sscan(row.Realtime, &us); err == nil && us > 0 {
			ts = us / 1_000_000
		}
	}
	return &agentws.LogLine{TS: ts, Source: source, Severity: sev, Line: row.Message}
}
