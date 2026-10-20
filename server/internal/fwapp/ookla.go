package fwapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const ooklaServersURL = "https://www.speedtest.net/api/js/servers"

// OoklaServer is a nearby Speedtest.net server (from FireProxy egress).
type OoklaServer struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Sponsor  string  `json:"sponsor"`
	Country  string  `json:"country,omitempty"`
	Host     string  `json:"host,omitempty"`
	Distance float64 `json:"distance,omitempty"` // km from Ookla's view of the caller
}

var ooklaHTTP = &http.Client{Timeout: 15 * time.Second}

// FetchOoklaServers lists nearby Ookla servers as seen from this host.
func FetchOoklaServers(ctx context.Context, limit int) ([]OoklaServer, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	u, _ := url.Parse(ooklaServersURL)
	q := u.Query()
	q.Set("engine", "js")
	q.Set("https_functional", "true")
	q.Set("limit", strconv.Itoa(limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "FireProxy/1.0")

	res, err := ooklaHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("ookla servers HTTP %d", res.StatusCode)
	}
	return parseOoklaServers(raw)
}

func parseOoklaServers(raw []byte) ([]OoklaServer, error) {
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("ookla servers: %w", err)
	}
	out := make([]OoklaServer, 0, len(rows))
	for _, m := range rows {
		id := ooklaString(m, "id", "serverid")
		if id == "" {
			continue
		}
		out = append(out, OoklaServer{
			ID:       id,
			Name:     ooklaString(m, "name"),
			Sponsor:  ooklaString(m, "sponsor"),
			Country:  ooklaString(m, "country"),
			Host:     ooklaString(m, "host"),
			Distance: jsonFloat(m, "distance"),
		})
	}
	return out, nil
}

func ooklaString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			return strings.TrimSpace(t)
		case float64:
			return strconv.FormatInt(int64(t), 10)
		case json.Number:
			return t.String()
		}
	}
	return ""
}
