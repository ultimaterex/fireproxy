package store

import (
	"database/sql"
	"fmt"
	"strings"
)

const kvPublicBaseURL = "agent_public_base_url"

func (p *Persist) PublicBaseURL() string {
	if p == nil {
		return ""
	}
	var raw []byte
	err := p.db.QueryRow(`SELECT v FROM kv WHERE k=?`, kvPublicBaseURL).Scan(&raw)
	if err == sql.ErrNoRows || err != nil {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(string(raw)), "/")
}

func (p *Persist) SetPublicBaseURL(url string) error {
	if p == nil {
		return fmt.Errorf("nil persist")
	}
	url = strings.TrimRight(strings.TrimSpace(url), "/")
	_, err := p.db.Exec(`INSERT INTO kv(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`,
		kvPublicBaseURL, []byte(url))
	return err
}
