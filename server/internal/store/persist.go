package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fireproxy/pkg/inventory"
	"fireproxy/pkg/snapshot"

	_ "modernc.org/sqlite"
)

const defaultMaxHistoryPoints = 360

// Persist is a SQLite-backed archive for metrics history and last catalog.
type Persist struct {
	db            *sql.DB
	path          string
	retentionDays int
	writes        int
}

// OpenPersist creates/opens a WAL SQLite file. retentionDays <= 0 means 90.
func OpenPersist(path string, retentionDays int) (*Persist, error) {
	if path == "" {
		return nil, fmt.Errorf("empty persist path")
	}
	if retentionDays <= 0 {
		retentionDays = 90
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	p := &Persist{db: db, path: path, retentionDays: retentionDays}
	if err := p.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := p.migrateLogs(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := p.migrateAgentEvents(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := p.migrateControlEvents(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := p.migrateAuth(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := p.Prune(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := p.PruneLogs(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := p.PruneAgentEvents(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := p.PruneControlEvents(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return p, nil
}

func (p *Persist) migrate() error {
	_, err := p.db.Exec(`
CREATE TABLE IF NOT EXISTS metrics (
  ts INTEGER PRIMARY KEY,
  m1 REAL NOT NULL,
  m5 REAL NOT NULL,
  m15 REAL NOT NULL,
  cores INTEGER NOT NULL DEFAULT 0,
  dns_restarts INTEGER NOT NULL DEFAULT 0,
  rates_json TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS kv (
  k TEXT PRIMARY KEY,
  v BLOB NOT NULL
);
`)
	if err != nil {
		return err
	}
	for _, col := range []string{
		`ALTER TABLE metrics ADD COLUMN ub_queries INTEGER`,
		`ALTER TABLE metrics ADD COLUMN ub_hits INTEGER`,
	} {
		if _, err := p.db.Exec(col); err != nil && !isDupColumn(err) {
			return err
		}
	}
	return nil
}

func isDupColumn(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate column")
}

func (p *Persist) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Close()
}

func (p *Persist) Path() string {
	if p == nil {
		return ""
	}
	return p.path
}

func (p *Persist) RetentionDays() int {
	if p == nil {
		return 0
	}
	return p.retentionDays
}

func (p *Persist) Count() (int64, error) {
	if p == nil {
		return 0, nil
	}
	var n int64
	err := p.db.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&n)
	return n, err
}

func (p *Persist) InsertHistory(pt HistoryPoint) error {
	if p == nil {
		return nil
	}
	raw, err := json.Marshal(pt.Rates)
	if err != nil {
		return err
	}
	if raw == nil {
		raw = []byte("{}")
	}
	_, err = p.db.Exec(
		`INSERT OR REPLACE INTO metrics(ts,m1,m5,m15,cores,dns_restarts,rates_json,ub_queries,ub_hits) VALUES(?,?,?,?,?,?,?,?,?)`,
		pt.TS, pt.Load.M1, pt.Load.M5, pt.Load.M15, pt.Load.Cores, pt.DNSRestarts, string(raw),
		pt.UnboundQueries, pt.UnboundHits,
	)
	if err != nil {
		return err
	}
	p.writes++
	if p.writes%64 == 0 {
		_ = p.Prune()
	}
	return nil
}

// UnboundAtOrBefore returns the newest unbound counters at or before ts.
func (p *Persist) UnboundAtOrBefore(ts int64) (at, queries, hits int64, ok bool) {
	if p == nil {
		return 0, 0, 0, false
	}
	var q, h sql.NullInt64
	err := p.db.QueryRow(
		`SELECT ts, ub_queries, ub_hits FROM metrics
		 WHERE ts<=? AND ub_queries IS NOT NULL AND ub_hits IS NOT NULL
		 ORDER BY ts DESC LIMIT 1`,
		ts,
	).Scan(&at, &q, &h)
	if err != nil || !q.Valid || !h.Valid {
		return 0, 0, 0, false
	}
	return at, q.Int64, h.Int64, true
}

func (p *Persist) SaveSnapshot(snap snapshot.Snapshot) error {
	if p == nil {
		return nil
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	_, err = p.db.Exec(`INSERT OR REPLACE INTO kv(k,v) VALUES('last_snapshot', ?)`, raw)
	return err
}

func (p *Persist) LoadSnapshot() (snapshot.Snapshot, bool, error) {
	if p == nil {
		return snapshot.Snapshot{}, false, nil
	}
	var raw []byte
	err := p.db.QueryRow(`SELECT v FROM kv WHERE k='last_snapshot'`).Scan(&raw)
	if err == sql.ErrNoRows {
		return snapshot.Snapshot{}, false, nil
	}
	if err != nil {
		return snapshot.Snapshot{}, false, err
	}
	var snap snapshot.Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return snapshot.Snapshot{}, false, err
	}
	return snap, true, nil
}

func (p *Persist) SaveCatalog(cat inventory.Catalog) error {
	if p == nil {
		return nil
	}
	raw, err := json.Marshal(cat)
	if err != nil {
		return err
	}
	_, err = p.db.Exec(`INSERT OR REPLACE INTO kv(k,v) VALUES('last_catalog', ?)`, raw)
	return err
}

// PutKV stores an opaque blob under key k.
func (p *Persist) PutKV(k string, v []byte) error {
	if p == nil {
		return nil
	}
	_, err := p.db.Exec(`INSERT OR REPLACE INTO kv(k,v) VALUES(?, ?)`, k, v)
	return err
}

// GetKV loads an opaque blob. ok=false when missing.
func (p *Persist) GetKV(k string) (v []byte, ok bool, err error) {
	if p == nil {
		return nil, false, nil
	}
	err = p.db.QueryRow(`SELECT v FROM kv WHERE k=?`, k).Scan(&v)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

func (p *Persist) SaveModules(enable map[string]bool) error {
	if p == nil {
		return nil
	}
	if enable == nil {
		enable = map[string]bool{}
	}
	raw, err := json.Marshal(enable)
	if err != nil {
		return err
	}
	_, err = p.db.Exec(`INSERT OR REPLACE INTO kv(k,v) VALUES('modules', ?)`, raw)
	return err
}

func (p *Persist) LoadModules() (map[string]bool, bool, error) {
	if p == nil {
		return nil, false, nil
	}
	var raw []byte
	err := p.db.QueryRow(`SELECT v FROM kv WHERE k='modules'`).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var enable map[string]bool
	if err := json.Unmarshal(raw, &enable); err != nil {
		return nil, false, err
	}
	if enable == nil {
		enable = map[string]bool{}
	}
	return enable, true, nil
}

func (p *Persist) SaveModulePrefs(v map[string]any) error {
	if p == nil {
		return nil
	}
	if v == nil {
		v = map[string]any{}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = p.db.Exec(`INSERT OR REPLACE INTO kv(k,v) VALUES('module_prefs', ?)`, raw)
	return err
}

func (p *Persist) LoadModulePrefs() (map[string]any, bool, error) {
	if p == nil {
		return nil, false, nil
	}
	var raw []byte
	err := p.db.QueryRow(`SELECT v FROM kv WHERE k='module_prefs'`).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, false, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, true, nil
}

func (p *Persist) LoadCatalog() (inventory.Catalog, bool, error) {
	if p == nil {
		return inventory.Catalog{}, false, nil
	}
	var raw []byte
	err := p.db.QueryRow(`SELECT v FROM kv WHERE k='last_catalog'`).Scan(&raw)
	if err == sql.ErrNoRows {
		return inventory.Catalog{}, false, nil
	}
	if err != nil {
		return inventory.Catalog{}, false, err
	}
	var cat inventory.Catalog
	if err := json.Unmarshal(raw, &cat); err != nil {
		return inventory.Catalog{}, false, err
	}
	return cat, true, nil
}

func (p *Persist) Prune() error {
	if p == nil {
		return nil
	}
	cutoff := time.Now().Unix() - int64(p.retentionDays)*24*3600
	_, err := p.db.Exec(`DELETE FROM metrics WHERE ts < ?`, cutoff)
	return err
}

// QueryHistory returns points in [from,to], downsampled to at most maxPoints.
func (p *Persist) QueryHistory(from, to int64, maxPoints int) ([]HistoryPoint, int64, error) {
	if p == nil {
		return nil, 1, nil
	}
	if maxPoints <= 0 {
		maxPoints = defaultMaxHistoryPoints
	}
	if to <= from {
		to = time.Now().Unix()
		from = to - 6*3600
	}
	span := to - from
	bucket := span / int64(maxPoints)
	if bucket < 1 {
		bucket = 1
	}

	var rows *sql.Rows
	var err error
	if bucket <= 60 {
		rows, err = p.db.Query(
			`SELECT ts,m1,m5,m15,cores,dns_restarts,rates_json FROM metrics
			 WHERE ts>=? AND ts<=? ORDER BY ts`,
			from, to,
		)
	} else {
		rows, err = p.db.Query(
			`SELECT b.ts, b.m1, b.m5, b.m15, b.cores, b.dns_restarts, m.rates_json
			 FROM (
			   SELECT (ts / ?) * ? AS ts,
			          AVG(m1) AS m1, AVG(m5) AS m5, AVG(m15) AS m15,
			          MAX(cores) AS cores, MAX(dns_restarts) AS dns_restarts,
			          MAX(ts) AS last_ts
			   FROM metrics
			   WHERE ts>=? AND ts<=?
			   GROUP BY ts / ?
			 ) b
			 JOIN metrics m ON m.ts = b.last_ts
			 ORDER BY b.ts`,
			bucket, bucket, from, to, bucket,
		)
	}
	if err != nil {
		return nil, bucket, err
	}
	defer rows.Close()

	out := make([]HistoryPoint, 0, 128)
	for rows.Next() {
		var pt HistoryPoint
		var rates string
		if err := rows.Scan(&pt.TS, &pt.Load.M1, &pt.Load.M5, &pt.Load.M15, &pt.Load.Cores, &pt.DNSRestarts, &rates); err != nil {
			return nil, bucket, err
		}
		pt.Rates = map[string]Rates{}
		if rates != "" {
			_ = json.Unmarshal([]byte(rates), &pt.Rates)
		}
		out = append(out, pt)
	}
	return out, bucket, rows.Err()
}

// RangeWindow maps UI range ids to [from,to] unix seconds.
func RangeWindow(id string, now int64) (from, to int64) {
	if now <= 0 {
		now = time.Now().Unix()
	}
	to = now
	switch id {
	case "1h":
		from = to - 3600
	case "1d":
		from = to - 24*3600
	case "7d":
		from = to - 7*24*3600
	case "30d":
		from = to - 30*24*3600
	case "90d":
		from = to - 90*24*3600
	default: // 6h
		from = to - 6*3600
	}
	return from, to
}

// FilterHistory keeps points in [from,to] and downsamples to maxPoints.
func FilterHistory(in []HistoryPoint, from, to int64, maxPoints int) []HistoryPoint {
	if maxPoints <= 0 {
		maxPoints = defaultMaxHistoryPoints
	}
	tmp := make([]HistoryPoint, 0, len(in))
	for _, p := range in {
		if p.TS >= from && p.TS <= to {
			tmp = append(tmp, p)
		}
	}
	if len(tmp) <= maxPoints {
		return tmp
	}
	out := make([]HistoryPoint, 0, maxPoints)
	for i := 0; i < maxPoints; i++ {
		idx := i * (len(tmp) - 1) / (maxPoints - 1)
		out = append(out, tmp[idx])
	}
	return out
}
