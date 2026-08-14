package store

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fireproxy/pkg/agentws"
)

const (
	defaultLogRetentionDays = 7
	kvLogRetention          = "log_retention_days"
)

// ServiceLog is one persisted line for the UI.
type ServiceLog struct {
	TS       int64  `json:"ts"`
	Source   string `json:"source"`
	Severity string `json:"severity,omitempty"`
	Line     string `json:"line"`
}

func (p *Persist) migrateLogs() error {
	_, err := p.db.Exec(`
CREATE TABLE IF NOT EXISTS service_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts INTEGER NOT NULL,
  source TEXT NOT NULL,
  severity TEXT NOT NULL DEFAULT '',
  line TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS service_logs_ts ON service_logs(ts);
CREATE INDEX IF NOT EXISTS service_logs_source_ts ON service_logs(source, ts);
`)
	return err
}

// InsertLogLines stores a batch and prunes by log retention.
func (p *Persist) InsertLogLines(lines []agentws.LogLine) error {
	if p == nil || len(lines) == 0 {
		return nil
	}
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO service_logs(ts, source, severity, line) VALUES(?,?,?,?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, ln := range lines {
		if _, err := stmt.Exec(ln.TS, ln.Source, ln.Severity, ln.Line); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return p.PruneLogs()
}

// QueryLogs returns newest-first lines matching filters.
func (p *Persist) QueryLogs(from, to int64, source, q string, limit int) ([]ServiceLog, error) {
	if p == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 500
	}
	if to <= 0 {
		to = time.Now().Unix()
	}
	if from <= 0 {
		from = to - 6*3600
	}
	args := []any{from, to}
	var b strings.Builder
	b.WriteString(`SELECT ts, source, severity, line FROM service_logs WHERE ts>=? AND ts<=?`)
	if source != "" && source != "all" {
		b.WriteString(` AND source=?`)
		args = append(args, source)
	}
	if q != "" {
		b.WriteString(` AND line LIKE ?`)
		args = append(args, "%"+q+"%")
	}
	b.WriteString(` ORDER BY ts DESC LIMIT ?`)
	args = append(args, limit)

	rows, err := p.db.Query(b.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ServiceLog, 0, 64)
	for rows.Next() {
		var ln ServiceLog
		if err := rows.Scan(&ln.TS, &ln.Source, &ln.Severity, &ln.Line); err != nil {
			return nil, err
		}
		out = append(out, ln)
	}
	return out, rows.Err()
}

// LogRetentionDays returns configured log retention (default 7).
func (p *Persist) LogRetentionDays() int {
	if p == nil {
		return defaultLogRetentionDays
	}
	var raw []byte
	err := p.db.QueryRow(`SELECT v FROM kv WHERE k=?`, kvLogRetention).Scan(&raw)
	if err == sql.ErrNoRows || err != nil {
		return defaultLogRetentionDays
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || n <= 0 {
		return defaultLogRetentionDays
	}
	return n
}

// SetLogRetentionDays persists log retention days.
func (p *Persist) SetLogRetentionDays(days int) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	if days <= 0 {
		days = defaultLogRetentionDays
	}
	_, err := p.db.Exec(`INSERT INTO kv(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`,
		kvLogRetention, []byte(strconv.Itoa(days)))
	if err != nil {
		return err
	}
	return p.PruneLogs()
}

// PruneLogs deletes service_logs older than log retention.
func (p *Persist) PruneLogs() error {
	if p == nil {
		return nil
	}
	days := p.LogRetentionDays()
	cutoff := time.Now().Unix() - int64(days)*24*3600
	_, err := p.db.Exec(`DELETE FROM service_logs WHERE ts < ?`, cutoff)
	return err
}
