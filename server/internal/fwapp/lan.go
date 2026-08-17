package fwapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// NetBot message types (Firewalla protocol).
const (
	MTypeGet  = "get"
	MTypeCmd  = "cmd"
	MTypeInit = "init"
	MTypeSet  = "set"
)

// LANClient talks only to the box on :8833 (never cloud).
type LANClient struct {
	HTTP *http.Client
}

func NewLANClient() *LANClient {
	return &LANClient{
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives:     true,
				ResponseHeaderTimeout: 45 * time.Second,
				DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			},
		},
	}
}

// NormalizeBoxIP strips schemes/paths/ports and requires a literal IP (no hostnames / userinfo).
// Returns the canonical IP string (IPv6 without brackets).
func NormalizeBoxIP(raw string) (string, error) {
	host := strings.TrimSpace(raw)
	if host == "" {
		return "", fmt.Errorf("box_ip required")
	}
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	host = strings.TrimSpace(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	} else {
		host = strings.Trim(host, "[]")
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "", fmt.Errorf("box_ip must be an IP address")
	}
	return ip.String(), nil
}

// boxLANURLHost formats an IP for use in http://{host}:8833/... (brackets for IPv6).
func boxLANURLHost(ipStr string) string {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return strings.TrimSpace(ipStr)
	}
	if ip.To4() == nil {
		return "[" + ip.String() + "]"
	}
	return ip.String()
}

// Send posts an encrypted NetBot message to http://{boxIP}:8833/v1/encipher/message/{gid}.
// Target defaults to 0.0.0.0 (box-wide). Host-scoped cmds (e.g. WoL) pass a MAC.
func (c *LANClient) Send(ctx context.Context, creds Creds, mtype string, data map[string]any) (json.RawMessage, error) {
	return c.SendTo(ctx, creds, mtype, data, "0.0.0.0")
}

// SendTo is Send with an explicit NetBot target (MAC or 0.0.0.0).
func (c *LANClient) SendTo(ctx context.Context, creds Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
	if strings.TrimSpace(creds.BoxIP) == "" || strings.TrimSpace(creds.Gid) == "" || strings.TrimSpace(creds.SymKey) == "" {
		return nil, ErrNotPaired
	}
	boxIP, err := NormalizeBoxIP(creds.BoxIP)
	if err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string]any{}
	}
	target = strings.TrimSpace(target)
	if target == "" {
		target = "0.0.0.0"
	}
	deviceName := strings.TrimSpace(creds.DeviceName)
	if deviceName == "" {
		deviceName = "FireProxy"
	}
	// Android / Gold SE expect a nested envelope: {mtype:"msg", message:{…}}.
	// Flat phone-style plaintext is accepted on some boxes but hangs or EOFs on gse.
	inner := map[string]any{
		"from": deviceName,
		"obj": map[string]any{
			"mtype":  mtype,
			"id":     RandomID(),
			"data":   data,
			"type":   "jsonmsg",
			"target": target,
		},
		"appInfo": map[string]any{
			"deviceName": deviceName,
			"appID":      ProtocolClientID,
			"platform":   runtime.GOOS,
			"timezone":   timezoneName(),
			"language":   "en",
			"version":    ProtocolClientVer,
			"eid":        creds.Eid,
		},
		"msg":          "",
		"type":         "jsondata",
		"compressMode": 1,
		"mtype":        "msg",
	}
	plainObj := map[string]any{
		"mtype":   "msg",
		"message": inner,
	}
	plain, err := json.Marshal(plainObj)
	if err != nil {
		return nil, err
	}
	ct, err := AESEncryptLegacy(creds.SymKey, string(plain))
	if err != nil {
		return nil, err
	}
	bodyMap := map[string]any{
		"timestamp": time.Now().Unix(),
		"message":   ct,
	}
	if creds.RKeyTS > 0 {
		bodyMap["rkeyts"] = creds.RKeyTS
	}
	body, _ := json.Marshal(bodyMap)
	url := fmt.Sprintf("http://%s:8833/v1/encipher/message/%s", boxLANURLHost(boxIP), creds.Gid)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Close = true
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLocalUnreach, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("local encipher HTTP %d: %s", res.StatusCode, formatEncipherErr(raw))
	}
	var wrap struct {
		Message json.RawMessage `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(raw, &wrap)
	msgField := string(wrap.Message)
	if msgField == "" && len(wrap.Data) > 0 {
		msgField = string(wrap.Data)
	}
	if msgField == "" {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			msgField = s
		} else {
			msgField = string(raw)
		}
	}
	var asStr string
	if json.Unmarshal([]byte(msgField), &asStr) == nil {
		msgField = asStr
	}
	dec, err := AESDecryptMessage(creds.SymKey, msgField)
	if err != nil {
		if json.Valid(raw) {
			return json.RawMessage(raw), nil
		}
		return nil, err
	}
	return json.RawMessage(dec), nil
}

// FetchInit loads the fw-app init payload (rules + hosts + tags).
func (c *LANClient) FetchInit(ctx context.Context, creds Creds) (json.RawMessage, error) {
	return c.Send(ctx, creds, MTypeInit, map[string]any{
		"get":             "0.0.0.0",
		"COMMAND_TIMEOUT": 15,
	})
}

// PingInit verifies LAN control with a lightweight cmd/ping (then init fallback).
func (c *LANClient) PingInit(ctx context.Context, creds Creds) error {
	return c.pingOnce(ctx, creds)
}

// PingInitReady retries while the box finishes post-pair init ("GID not exists").
func (c *LANClient) PingInitReady(ctx context.Context, creds Creds) error {
	var last error
	for i := 0; i < 15; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				if last != nil {
					return last
				}
				return ctx.Err()
			case <-time.After(3 * time.Second):
			}
		}
		last = c.pingOnce(ctx, creds)
		if last == nil {
			return nil
		}
		if !isTransientLAN(last) {
			return last
		}
	}
	return last
}

func (c *LANClient) pingOnce(ctx context.Context, creds Creds) error {
	_, err := c.Send(ctx, creds, MTypeCmd, map[string]any{
		"item":  "ping",
		"value": map[string]any{},
	})
	if err == nil {
		return nil
	}
	_, err2 := c.Send(ctx, creds, MTypeInit, map[string]any{
		"get":             "0.0.0.0",
		"COMMAND_TIMEOUT": 15,
	})
	if err2 == nil {
		return nil
	}
	// Prefer the more specific encipher body when present.
	if !errors.Is(err2, ErrLocalUnreach) {
		return err2
	}
	return err
}

func isTransientLAN(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrLocalUnreach) {
		return true
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "init phase"):
		return true
	case strings.Contains(s, "gid not exists"):
		return true
	case strings.Contains(s, "eof"):
		return true
	case strings.Contains(s, "connection refused"):
		return true
	case strings.Contains(s, "timeout"):
		return true
	case strings.Contains(s, "http 412"):
		// Box has not finished accepting the new pairing yet.
		return true
	default:
		return false
	}
}

func formatEncipherErr(raw []byte) string {
	var obj struct {
		Error  string `json:"error"`
		Status string `json:"status"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		if obj.Error != "" {
			return obj.Error
		}
		if obj.Status != "" {
			return obj.Status
		}
	}
	return truncate(string(raw), 200)
}

func timezoneName() string {
	if zone, _ := time.Now().Zone(); zone != "" {
		return zone
	}
	return "UTC"
}
