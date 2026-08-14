package agentevents

import (
	"path/filepath"
	"testing"
)

func TestAppendPendingDrop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-events.jsonl")
	s := &Store{Path: path}
	if err := s.Append("update_failed", "sha256 mismatch"); err != nil {
		t.Fatal(err)
	}
	if err := s.Append("update_failed", "download status 404"); err != nil {
		t.Fatal(err)
	}
	lines, err := s.Pending()
	if err != nil || len(lines) != 2 {
		t.Fatalf("%+v err=%v", lines, err)
	}
	if lines[0].Kind != "update_failed" || lines[0].Detail == "" {
		t.Fatalf("%+v", lines[0])
	}
	if err := s.Drop(1); err != nil {
		t.Fatal(err)
	}
	lines, err = s.Pending()
	if err != nil || len(lines) != 1 || lines[0].Detail != "download status 404" {
		t.Fatalf("%+v err=%v", lines, err)
	}
	if err := s.Drop(5); err != nil {
		t.Fatal(err)
	}
	lines, err = s.Pending()
	if err != nil || len(lines) != 0 {
		t.Fatalf("%+v", lines)
	}
}
