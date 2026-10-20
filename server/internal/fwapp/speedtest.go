package fwapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// SpeedtestResult is a normalized LAN speedtest outcome for the UI.
type SpeedtestResult struct {
	WanUUID  string  `json:"wan_uuid,omitempty"`
	Down     float64 `json:"down"` // Mbps
	Up       float64 `json:"up"`
	Ping     float64 `json:"ping,omitempty"`
	Jitter   float64 `json:"jitter,omitempty"`
	TS       int64   `json:"ts,omitempty"` // unix seconds
	ServerID string  `json:"server_id,omitempty"`
	Server   string  `json:"server,omitempty"`
	Location string  `json:"location,omitempty"`
}

// speedtestHTTP waits out a full internet speedtest (often 30–90s).
var speedtestHTTP = &http.Client{
	Timeout: 3 * time.Minute,
	Transport: &http.Transport{
		DisableKeepAlives:     true,
		ResponseHeaderTimeout: 3 * time.Minute,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
	},
}

// RunSpeedtest starts cmd runInternetSpeedtest for wanUUID and returns the box result.
// serverID is optional (Ookla server id); empty lets the box pick.
func (c *LANClient) RunSpeedtest(ctx context.Context, creds Creds, wanUUID, serverID string) (SpeedtestResult, error) {
	var zero SpeedtestResult
	wanUUID = strings.TrimSpace(wanUUID)
	if wanUUID == "" {
		return zero, fmt.Errorf("wan_uuid required")
	}
	// Dedicated client so we do not race with the shared health HTTP client.
	transport := http.DefaultTransport
	if c.HTTP != nil && c.HTTP.Transport != nil {
		transport = c.HTTP.Transport
	} else if speedtestHTTP.Transport != nil {
		transport = speedtestHTTP.Transport
	}
	long := &LANClient{
		HTTP: &http.Client{
			Timeout:   3 * time.Minute,
			Transport: transport,
		},
	}
	value := map[string]any{
		"wanUUID": wanUUID,
	}
	if sid := strings.TrimSpace(serverID); sid != "" {
		value["serverId"] = sid
	}
	raw, err := long.Send(ctx, creds, MTypeCmd, map[string]any{
		"item":  "runInternetSpeedtest",
		"value": value,
	})
	if err != nil {
		return zero, err
	}
	out, err := parseSpeedtestReply(raw, wanUUID)
	if err != nil {
		return zero, err
	}
	return out, nil
}

// parseSpeedtestHistoryReply parses a history/init row without stamping wall-clock now for missing TS.
func parseSpeedtestHistoryReply(raw json.RawMessage, wanUUID string) (SpeedtestResult, error) {
	return parseSpeedtestReplyTS(raw, wanUUID, false)
}

// GetSpeedtestResults fetches recent box history via get internetSpeedtestResults.
// beginUnix is inclusive (seconds); 0 means "as far back as the box returns".
func (c *LANClient) GetSpeedtestResults(ctx context.Context, creds Creds, beginUnix int64) ([]SpeedtestResult, error) {
	value := map[string]any{}
	if beginUnix > 0 {
		value["begin"] = beginUnix
	}
	raw, err := c.Send(ctx, creds, MTypeGet, map[string]any{
		"item":  "internetSpeedtestResults",
		"value": value,
	})
	if err != nil {
		return nil, err
	}
	return parseSpeedtestResults(raw)
}

func parseSpeedtestResults(raw json.RawMessage) ([]SpeedtestResult, error) {
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	payload := raw
	if json.Unmarshal(raw, &envelope) == nil && len(envelope.Data) > 0 {
		if envelope.Code != 0 && envelope.Code != 200 {
			return nil, fmt.Errorf("speedtest results code %d", envelope.Code)
		}
		payload = envelope.Data
	}

	var wrap struct {
		Results json.RawMessage `json:"results"`
	}
	body := payload
	if json.Unmarshal(payload, &wrap) == nil && len(wrap.Results) > 0 {
		body = wrap.Results
	}

	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("speedtest results: %w", err)
	}
	out := make([]SpeedtestResult, 0, len(items))
	for _, item := range items {
		r, err := parseSpeedtestHistoryReply(item, "")
		if err != nil {
			continue
		}
		if r.WanUUID == "" {
			r.WanUUID = speedtestWanUUID(item)
		}
		if r.WanUUID == "" || r.TS == 0 {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func speedtestWanUUID(raw json.RawMessage) string {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	for _, k := range []string{"uuid", "wanUUID", "wanUuid", "intf"} {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func parseSpeedtestReply(raw json.RawMessage, wanUUID string) (SpeedtestResult, error) {
	// Live runInternetSpeedtest replies may omit timestamp; stamp now for the UI job.
	return parseSpeedtestReplyTS(raw, wanUUID, true)
}

func parseSpeedtestReplyTS(raw json.RawMessage, wanUUID string, allowNow bool) (SpeedtestResult, error) {
	var zero SpeedtestResult
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	payload := raw
	if json.Unmarshal(raw, &envelope) == nil && len(envelope.Data) > 0 {
		if envelope.Code != 0 && envelope.Code != 200 {
			return zero, fmt.Errorf("speedtest code %d", envelope.Code)
		}
		payload = envelope.Data
	}

	// cmd reply often wraps once more: {result:{success,timestamp,result:{…}}}
	// History rows keep uuid/success/timestamp on the outer object with rates in result — do not unwrap those.
	var outerWrap struct {
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(payload, &outerWrap) == nil && len(outerWrap.Result) > 0 {
		var probe map[string]any
		if json.Unmarshal(outerWrap.Result, &probe) == nil {
			if probe["success"] != nil || probe["timestamp"] != nil || probe["result"] != nil {
				payload = outerWrap.Result
			}
		}
	}

	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		return zero, fmt.Errorf("speedtest reply: %w", err)
	}
	if v, ok := m["success"].(bool); ok && !v {
		return zero, fmt.Errorf("speedtest failed on box")
	}

	out := SpeedtestResult{WanUUID: wanUUID}
	if out.WanUUID == "" {
		out.WanUUID = speedtestWanUUID(payload)
	}
	out.Down = jsonFloat(m, "download", "dlStatus", "dl")
	out.Up = jsonFloat(m, "upload", "ulStatus", "ul")
	out.Ping = jsonFloat(m, "latency", "pingStatus", "ping")
	out.Jitter = jsonFloat(m, "jitter", "jitterStatus")
	if res, ok := m["result"].(map[string]any); ok {
		if v := jsonFloat(res, "download"); v > 0 {
			out.Down = v
		}
		if v := jsonFloat(res, "upload"); v > 0 {
			out.Up = v
		}
		if v := jsonFloat(res, "latency"); v > 0 {
			out.Ping = v
		}
		if v := jsonFloat(res, "jitter"); v > 0 {
			out.Jitter = v
		}
	}
	if ts := jsonFloat(m, "timestamp", "ts"); ts > 0 {
		if ts > 1e12 {
			ts /= 1000
		}
		out.TS = int64(ts)
	}
	if out.TS == 0 && allowNow {
		out.TS = time.Now().Unix()
	}
	out.ServerID, out.Server, out.Location = speedtestServerFields(m)
	if out.Down == 0 && out.Up == 0 {
		return zero, fmt.Errorf("speedtest returned no rates")
	}
	return out, nil
}

func speedtestServerFields(m map[string]any) (id, sponsor, location string) {
	srv, _ := m["server"].(map[string]any)
	if srv == nil {
		return "", "", ""
	}
	id = ooklaString(srv, "id", "serverid")
	sponsor = ooklaString(srv, "sponsor")
	location = ooklaString(srv, "location", "name")
	return id, sponsor, location
}

func jsonFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case float64:
			return t
		case json.Number:
			f, _ := t.Float64()
			return f
		case string:
			var f float64
			if _, err := fmt.Sscanf(strings.TrimSpace(t), "%f", &f); err == nil {
				return f
			}
		case int:
			return float64(t)
		case int64:
			return float64(t)
		}
	}
	return 0
}
