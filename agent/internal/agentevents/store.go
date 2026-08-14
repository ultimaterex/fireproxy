package agentevents

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"fireproxy/pkg/agentws"
)

const DefaultPath = "/home/pi/.fireproxy/agent-events.jsonl"

// Store is a tiny append-only JSONL of agent lifecycle lines.
type Store struct {
	Path string
	mu   sync.Mutex
}

func (s *Store) path() string {
	if s != nil && s.Path != "" {
		return s.Path
	}
	return DefaultPath
}

// Append writes one line (e.g. update_failed).
func (s *Store) Append(kind, detail string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.path()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	ln := agentws.AgentEventLine{TS: time.Now().Unix(), Kind: kind, Detail: detail}
	b, err := json.Marshal(ln)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// Pending returns all lines currently on disk.
func (s *Store) Pending() ([]agentws.AgentEventLine, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readAll()
}

// Drop removes the first n lines (after ack).
func (s *Store) Drop(n int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n <= 0 {
		return nil
	}
	lines, err := s.readAll()
	if err != nil {
		return err
	}
	if n >= len(lines) {
		return os.Remove(s.path())
	}
	rest := lines[n:]
	path := s.path()
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	for _, ln := range rest {
		b, err := json.Marshal(ln)
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return err
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return err
		}
	}
	_ = f.Close()
	return os.Rename(tmp, path)
}

func (s *Store) readAll() ([]agentws.AgentEventLine, error) {
	path := s.path()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []agentws.AgentEventLine
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ln agentws.AgentEventLine
		if err := json.Unmarshal(line, &ln); err != nil {
			continue
		}
		out = append(out, ln)
	}
	return out, sc.Err()
}
