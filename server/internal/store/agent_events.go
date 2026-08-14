package store

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const agentEventRetentionDays = 365

const (
	kvAgentLastVersion   = "agent_last_version"
	kvAgentLastOfflineTS = "agent_last_offline_ts"
)

// AgentEvent is one persisted agent lifecycle row.
type AgentEvent struct {
	ID      int64  `json:"id"`
	TS      int64  `json:"ts"`
	Kind    string `json:"kind"`
	Detail  string `json:"detail,omitempty"`
	FromVer string `json:"from_ver,omitempty"`
	ToVer   string `json:"to_ver,omitempty"`
}

func (p *Persist) migrateAgentEvents() error {
	_, err := p.db.Exec(`
CREATE TABLE IF NOT EXISTS agent_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts INTEGER NOT NULL,
  kind TEXT NOT NULL,
  detail TEXT NOT NULL DEFAULT '',
  from_ver TEXT NOT NULL DEFAULT '',
  to_ver TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS agent_events_ts ON agent_events(ts);
CREATE INDEX IF NOT EXISTS agent_events_id ON agent_events(id);
`)
	return err
}

// InsertAgentEvent stores one event and prunes by retention.
func (p *Persist) InsertAgentEvent(e AgentEvent) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	if e.TS <= 0 {
		e.TS = time.Now().Unix()
	}
	_, err := p.db.Exec(
		`INSERT INTO agent_events(ts, kind, detail, from_ver, to_ver) VALUES(?,?,?,?,?)`,
		e.TS, e.Kind, e.Detail, e.FromVer, e.ToVer,
	)
	if err != nil {
		return err
	}
	return p.PruneAgentEvents()
}

// QueryAgentEvents returns newest-first rows. If beforeID > 0, only id < beforeID.
func (p *Persist) QueryAgentEvents(beforeID int64, limit int) ([]AgentEvent, error) {
	if p == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 25
	}
	args := []any{}
	var b strings.Builder
	b.WriteString(`SELECT id, ts, kind, detail, from_ver, to_ver FROM agent_events`)
	if beforeID > 0 {
		b.WriteString(` WHERE id < ?`)
		args = append(args, beforeID)
	}
	b.WriteString(` ORDER BY id DESC LIMIT ?`)
	args = append(args, limit)

	rows, err := p.db.Query(b.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AgentEvent
	for rows.Next() {
		var e AgentEvent
		if err := rows.Scan(&e.ID, &e.TS, &e.Kind, &e.Detail, &e.FromVer, &e.ToVer); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// PruneAgentEvents deletes rows older than 365 days.
func (p *Persist) PruneAgentEvents() error {
	if p == nil {
		return nil
	}
	cutoff := time.Now().Unix() - int64(agentEventRetentionDays)*24*3600
	_, err := p.db.Exec(`DELETE FROM agent_events WHERE ts < ?`, cutoff)
	return err
}

func (p *Persist) AgentLastVersion() (string, error) {
	return p.kvString(kvAgentLastVersion)
}

func (p *Persist) SetAgentLastVersion(v string) error {
	return p.kvSet(kvAgentLastVersion, strings.TrimSpace(v))
}

func (p *Persist) AgentLastOfflineTS() (int64, error) {
	s, err := p.kvString(kvAgentLastOfflineTS)
	if err != nil || s == "" {
		return 0, err
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func (p *Persist) SetAgentLastOfflineTS(ts int64) error {
	return p.kvSet(kvAgentLastOfflineTS, strconv.FormatInt(ts, 10))
}

func (p *Persist) ClearAgentLastOfflineTS() error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	_, err := p.db.Exec(`DELETE FROM kv WHERE k=?`, kvAgentLastOfflineTS)
	return err
}

func (p *Persist) kvString(key string) (string, error) {
	if p == nil {
		return "", nil
	}
	var raw []byte
	err := p.db.QueryRow(`SELECT v FROM kv WHERE k=?`, key).Scan(&raw)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (p *Persist) kvSet(key, val string) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	_, err := p.db.Exec(`INSERT INTO kv(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`,
		key, []byte(val))
	return err
}

// FormatOfflineDuration returns compact Xm / Xh / Xd for UI copy.
func FormatOfflineDuration(sec int64) string {
	if sec < 0 {
		sec = 0
	}
	if sec < 3600 {
		m := (sec + 59) / 60
		if m < 1 {
			m = 1
		}
		return fmt.Sprintf("%dm", m)
	}
	if sec < 86400 {
		h := (sec + 1799) / 3600
		if h < 1 {
			h = 1
		}
		return fmt.Sprintf("%dh", h)
	}
	d := (sec + 43199) / 86400
	if d < 1 {
		d = 1
	}
	return fmt.Sprintf("%dd", d)
}
