package svclogs

import (
	"bufio"
	"os"
	"strings"
	"time"

	"fireproxy/pkg/agentws"
)

// ACLFileReader reads notable ACL lines via byte offset (like ACLTracker).
type ACLFileReader struct {
	Path   string
	Offset int64
}

// ReadNotable returns new notable lines and the offset after a successful read.
// Caller should persist newOffset only after server ack.
func (r *ACLFileReader) ReadNotable(max int) (lines []agentws.LogLine, newOffset int64, err error) {
	if r == nil || r.Path == "" {
		return nil, 0, nil
	}
	fi, err := os.Stat(r.Path)
	if err != nil {
		return nil, r.Offset, err
	}
	size := fi.Size()
	off := r.Offset
	if size < off {
		off = 0 // rotated
	}
	// First read / offset 0 on a large file: start near the end (Blocked-only still applies).
	if off == 0 && size > ACLTailBytes {
		off = size - ACLTailBytes
	}
	newOffset = off
	if size == off {
		return nil, off, nil
	}
	f, err := os.Open(r.Path)
	if err != nil {
		return nil, off, err
	}
	defer f.Close()
	if _, err := f.Seek(off, 0); err != nil {
		return nil, off, err
	}
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	now := time.Now().Unix()
	for sc.Scan() {
		text := sc.Text()
		if !notableACL(text) {
			continue
		}
		if !QuietKeep("info", text) {
			continue
		}
		lines = append(lines, agentws.LogLine{
			TS:       now,
			Source:   "dnsmasq",
			Severity: "warn",
			Line:     text,
		})
		if max > 0 && len(lines) >= max {
			break
		}
	}
	if err := sc.Err(); err != nil {
		return lines, off, err
	}
	return lines, size, nil
}

func notableACL(line string) bool {
	return strings.Contains(line, "[ACL][Blocked]")
}
