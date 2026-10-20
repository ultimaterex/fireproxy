package store

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	defaultControlHistoryRetentionDays = 365
	maxControlHistoryRetentionDays     = 3650
	kvControlHistoryRetention          = "control_history_retention_days"
)

// ControlEvent is one persisted control-plane write row.
type ControlEvent struct {
	ID         int64  `json:"id"`
	TS         int64  `json:"ts"`
	Scheme     string `json:"scheme"`
	Action     string `json:"action"`
	ActorKind  string `json:"actor_kind"`
	Actor      string `json:"actor"`
	Target     string `json:"target"`
	Summary    string `json:"summary,omitempty"`
	Result     string `json:"result"`
	Error      string `json:"error,omitempty"`
	BeforeJSON string `json:"before_json,omitempty"`
	AfterJSON  string `json:"after_json,omitempty"`
}

// ControlEventQuery filters control history rows (newest-first).
type ControlEventQuery struct {
	Scheme    string
	Action    string
	ActorKind string
	Result    string
	Q         string
	BeforeID  int64
	Limit     int
}

func (p *Persist) migrateControlEvents() error {
	_, err := p.db.Exec(`
CREATE TABLE IF NOT EXISTS control_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts INTEGER NOT NULL,
  scheme TEXT NOT NULL,
  action TEXT NOT NULL,
  actor_kind TEXT NOT NULL,
  actor TEXT NOT NULL DEFAULT '',
  target TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  result TEXT NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  before_json TEXT NOT NULL DEFAULT '',
  after_json TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS control_events_ts ON control_events(ts);
CREATE INDEX IF NOT EXISTS control_events_scheme_action ON control_events(scheme, action);
CREATE INDEX IF NOT EXISTS control_events_actor_kind ON control_events(actor_kind);
CREATE INDEX IF NOT EXISTS control_events_result ON control_events(result);
CREATE INDEX IF NOT EXISTS control_events_id ON control_events(id);
`)
	return err
}

// InsertControlEvent stores one row and prunes by retention.
func (p *Persist) InsertControlEvent(e ControlEvent) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	if e.TS <= 0 {
		e.TS = time.Now().UnixMilli()
	}
	_, err := p.db.Exec(
		`INSERT INTO control_events(ts, scheme, action, actor_kind, actor, target, summary, result, error, before_json, after_json)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		e.TS, e.Scheme, e.Action, e.ActorKind, e.Actor, e.Target, e.Summary, e.Result, e.Error, e.BeforeJSON, e.AfterJSON,
	)
	if err != nil {
		return err
	}
	return p.PruneControlEvents()
}

// QueryControlEvents returns newest-first rows matching filters.
// If BeforeID > 0, only id < BeforeID (cursor from previous page's oldest id).
func (p *Persist) QueryControlEvents(q ControlEventQuery) ([]ControlEvent, error) {
	if p == nil {
		return nil, nil
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 25
	}
	args := []any{}
	var b strings.Builder
	b.WriteString(`SELECT id, ts, scheme, action, actor_kind, actor, target, summary, result, error, before_json, after_json FROM control_events WHERE 1=1`)
	if q.Scheme != "" {
		b.WriteString(` AND scheme=?`)
		args = append(args, q.Scheme)
	}
	if q.Action != "" {
		b.WriteString(` AND action=?`)
		args = append(args, q.Action)
	}
	if q.ActorKind != "" {
		b.WriteString(` AND actor_kind=?`)
		args = append(args, q.ActorKind)
	}
	if q.Result != "" {
		b.WriteString(` AND result=?`)
		args = append(args, q.Result)
	}
	if q.Q != "" {
		like := "%" + q.Q + "%"
		b.WriteString(` AND (target LIKE ? OR summary LIKE ? OR actor LIKE ?)`)
		args = append(args, like, like, like)
	}
	if q.BeforeID > 0 {
		b.WriteString(` AND id < ?`)
		args = append(args, q.BeforeID)
	}
	b.WriteString(` ORDER BY id DESC LIMIT ?`)
	args = append(args, limit)

	rows, err := p.db.Query(b.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ControlEvent
	for rows.Next() {
		var e ControlEvent
		if err := rows.Scan(
			&e.ID, &e.TS, &e.Scheme, &e.Action, &e.ActorKind, &e.Actor, &e.Target, &e.Summary,
			&e.Result, &e.Error, &e.BeforeJSON, &e.AfterJSON,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ControlHistoryRetentionDays returns configured retention (default 365).
func (p *Persist) ControlHistoryRetentionDays() int {
	if p == nil {
		return defaultControlHistoryRetentionDays
	}
	var raw []byte
	err := p.db.QueryRow(`SELECT v FROM kv WHERE k=?`, kvControlHistoryRetention).Scan(&raw)
	if err == sql.ErrNoRows || err != nil {
		return defaultControlHistoryRetentionDays
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || n <= 0 {
		return defaultControlHistoryRetentionDays
	}
	if n > maxControlHistoryRetentionDays {
		return maxControlHistoryRetentionDays
	}
	return n
}

// SetControlHistoryRetentionDays persists retention days (clamped 1–3650).
func (p *Persist) SetControlHistoryRetentionDays(days int) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	if days <= 0 {
		days = defaultControlHistoryRetentionDays
	}
	if days > maxControlHistoryRetentionDays {
		days = maxControlHistoryRetentionDays
	}
	_, err := p.db.Exec(`INSERT INTO kv(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`,
		kvControlHistoryRetention, []byte(strconv.Itoa(days)))
	if err != nil {
		return err
	}
	return p.PruneControlEvents()
}

// PruneControlEvents deletes rows older than control history retention.
func (p *Persist) PruneControlEvents() error {
	if p == nil {
		return nil
	}
	days := p.ControlHistoryRetentionDays()
	cutoff := time.Now().UnixMilli() - int64(days)*24*3600*1000
	_, err := p.db.Exec(`DELETE FROM control_events WHERE ts < ?`, cutoff)
	return err
}
