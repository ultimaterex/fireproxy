package svclogs

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// CursorStore persists journal cursors and ACL byte offset.
type CursorStore struct {
	Path      string            `json:"-"`
	Journal   map[string]string `json:"journal"`
	ACLOffset int64             `json:"acl_offset"`
	inited    bool
}

func LoadCursorStore(path string) (*CursorStore, error) {
	st := &CursorStore{Path: path, Journal: map[string]string{}}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, st); err != nil {
		return nil, err
	}
	st.Path = path
	if st.Journal == nil {
		st.Journal = map[string]string{}
	}
	st.inited = true
	return st, nil
}

func (s *CursorStore) IsFirstBoot() bool {
	return s == nil || !s.inited || len(s.Journal) == 0
}

func (s *CursorStore) InitNow(units map[string]string) {
	if s.Journal == nil {
		s.Journal = map[string]string{}
	}
	for u := range units {
		if _, ok := s.Journal[u]; !ok {
			s.Journal[u] = CursorNow
		}
	}
	s.inited = true
}

func (s *CursorStore) Save() error {
	if s == nil || s.Path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}
