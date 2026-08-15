package fwapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PairQR is the JSON payload encoded in the Firewalla Additional Pairing QR.
type PairQR struct {
	GID         string `json:"gid"`
	Seed        string `json:"seed"`
	License     string `json:"license"`
	Ek          string `json:"ek"`
	IPAddress   string `json:"ipaddress"`
	DeviceName  string `json:"deviceName"`
	RR          string `json:"rr"`
	Service     string `json:"service"`
	Type        string `json:"type"`
	Model       string `json:"model"`
}

// PairRequest is the UI/API pairing input.
type PairRequest struct {
	QRJSON     string `json:"qr_json"`
	BoxIP      string `json:"box_ip"`
	Email      string `json:"email"`
	DeviceName string `json:"device_name,omitempty"`
	FirstBind  bool   `json:"first_bind,omitempty"`
}

type cloudClient struct {
	http    *http.Client
	baseURL string
}

func newCloudClient() *cloudClient {
	return &cloudClient{
		http:    &http.Client{Timeout: 45 * time.Second},
		baseURL: CloudAPIBase,
	}
}

type etokenResp struct {
	AccessToken string `json:"access_token"`
	EID         string `json:"eid"`
	AID         string `json:"aid"`
	Groups      []cloudGroup `json:"groups"`
}

type cloudGroup struct {
	ID            string           `json:"_id"`
	Name          string           `json:"name"`
	EID           string           `json:"eid"`
	AID           string           `json:"aid"`
	SymmetricKeys []cloudSymKeyEntry `json:"symmetricKeys"`
}

type cloudSymKeyEntry struct {
	Key  string `json:"key"`
	EID  string `json:"eid"`
	RKey string `json:"rkey"`
}

func (c *cloudClient) loginEPT(ctx context.Context, kp *KeyPair, email, deviceName string) (*etokenResp, error) {
	name := strings.TrimSpace(deviceName)
	if name == "" {
		name = strings.TrimSpace(email)
	}
	if name == "" {
		name = "FireProxy"
	}
	// Shape matches Firewalla encipher eptLogin + Additional Pairing clients.
	// assertion.name is what Paired Phones shows as the device label.
	body := map[string]any{
		"assertion": map[string]any{
			"name":      name,
			"info":      map[string]any{"name": "circle", "email": email},
			"publicKey": kp.PublicPEM,
			"appId":     ProtocolClientID,
			"appSecret": ProtocolClientSecret,
			"signature": "",
		},
	}
	var out etokenResp
	if err := c.postJSON(ctx, "/login/eptoken", "", body, &out); err != nil {
		return nil, err
	}
	if out.AccessToken == "" || out.EID == "" {
		return nil, fmt.Errorf("eptoken: missing token or eid")
	}
	return &out, nil
}

func (c *cloudClient) postJSON(ctx context.Context, path, bearer string, body any, dest any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.baseURL, "/")+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("cloud %s: HTTP %d: %s", path, res.StatusCode, truncate(string(b), 240))
	}
	if dest == nil || len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, dest)
}

func (c *cloudClient) getJSON(ctx context.Context, path, bearer string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.baseURL, "/")+path, nil)
	if err != nil {
		return err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("cloud GET %s: HTTP %d: %s", path, res.StatusCode, truncate(string(b), 240))
	}
	return json.Unmarshal(b, dest)
}

// PairWithCloud performs Additional Pairing against encipher (cloud used once).
func PairWithCloud(ctx context.Context, req PairRequest) (Creds, error) {
	var zero Creds
	qr, err := parseQR(req.QRJSON)
	if err != nil {
		return zero, err
	}
	boxIP := strings.TrimSpace(req.BoxIP)
	if boxIP == "" {
		boxIP = strings.TrimSpace(qr.IPAddress)
	}
	if boxIP == "" {
		return zero, fmt.Errorf("boxIP required")
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		return zero, fmt.Errorf("email required")
	}
	name := strings.TrimSpace(req.DeviceName)
	if name == "" {
		name = "FireProxy"
	}

	kp, err := GenerateKeyPair()
	if err != nil {
		return zero, err
	}
	cloud := newCloudClient()

	login, err := cloud.loginEPT(ctx, kp, email, name)
	if err != nil {
		return zero, fmt.Errorf("cloud login: %w", err)
	}

	rid, evalue, err := decryptPairingCode(qr, req.FirstBind)
	if err != nil {
		return zero, fmt.Errorf("decrypt QR ek: %w (turn on Additional Pairing and paste a fresh QR)", err)
	}
	if err := cloud.startRendezvous(ctx, login.AccessToken, rid, evalue); err != nil {
		return zero, fmt.Errorf("rendezvous: %w", err)
	}

	group, lastLogin, err := cloud.pollGroup(ctx, kp, email, name, login.AccessToken, qr.GID)
	if err != nil {
		return zero, err
	}

	aid := firstNonEmpty(group.AID, lastLogin.AID)
	eid := firstNonEmpty(lastLogin.EID, group.EID, login.EID)
	symKey, rkeyts, err := extractGroupSymmetricKey(kp, eid, group.SymmetricKeys)
	if err != nil {
		return zero, err
	}

	return Creds{
		PairedAt:   time.Now().UTC(),
		BoxIP:      boxIP,
		Gid:        qr.GID,
		Eid:        eid,
		Aid:        aid,
		SymKey:     symKey,
		RKeyTS:     rkeyts,
		PrivatePEM: kp.PrivatePEM,
		PublicPEM:  kp.PublicPEM,
		Email:      email,
		DeviceName: name,
	}, nil
}

func (c *cloudClient) startRendezvous(ctx context.Context, token, rid, evalue string) error {
	body := map[string]any{
		"rid":    rid,
		"evalue": evalue,
	}
	return c.postJSON(ctx, "/ept/rendezvous/me", token, body, nil)
}

func (c *cloudClient) pollGroup(ctx context.Context, kp *KeyPair, email, deviceName, token, gid string) (*cloudGroup, *etokenResp, error) {
	var lastLogin *etokenResp
	tok := token
	for i := 0; i < 20; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(3 * time.Second):
			}
		}
		for _, path := range []string{"/ept/group/me", "/ept/groups/me"} {
			if g, ok := c.fetchGroup(ctx, path, tok, gid); ok {
				return g, lastLogin, nil
			}
		}
		var err error
		lastLogin, err = c.loginEPT(ctx, kp, email, deviceName)
		if err != nil {
			return nil, nil, fmt.Errorf("cloud login poll: %w", err)
		}
		tok = lastLogin.AccessToken
		for i := range lastLogin.Groups {
			g := &lastLogin.Groups[i]
			if g.ID == gid {
				return g, lastLogin, nil
			}
		}
	}
	return nil, lastLogin, fmt.Errorf("handshake timeout: group %s not in membership yet — keep Additional Pairing on and use a fresh QR", gidHint(gid))
}

func (c *cloudClient) fetchGroup(ctx context.Context, path, token, gid string) (*cloudGroup, bool) {
	var asList []cloudGroup
	if err := c.getJSON(ctx, path, token, &asList); err == nil {
		for i := range asList {
			if asList[i].ID == gid {
				return &asList[i], true
			}
		}
	}
	var asObj struct {
		Groups []cloudGroup `json:"groups"`
	}
	if err := c.getJSON(ctx, path, token, &asObj); err == nil {
		for i := range asObj.Groups {
			if asObj.Groups[i].ID == gid {
				return &asObj.Groups[i], true
			}
		}
	}
	return nil, false
}

// extractGroupSymmetricKey prefers rotation key (rkey) when present — required on Gold SE / rekey boxes.
func extractGroupSymmetricKey(kp *KeyPair, eid string, keys []cloudSymKeyEntry) (sym string, rkeyts int64, err error) {
	var sk *cloudSymKeyEntry
	for i := range keys {
		if keys[i].EID == eid {
			sk = &keys[i]
			break
		}
	}
	if sk == nil && len(keys) > 0 {
		sk = &keys[0]
	}
	if sk == nil {
		return "", 0, fmt.Errorf("group has no symmetric key")
	}
	if strings.TrimSpace(sk.RKey) != "" {
		var rk struct {
			TS   int64  `json:"ts"`
			TTL  int64  `json:"ttl"`
			Key  string `json:"key"`
			NKey string `json:"nkey"`
		}
		if err := json.Unmarshal([]byte(sk.RKey), &rk); err == nil && rk.Key != "" {
			dec, err := kp.RSADecryptBase64(rk.Key)
			if err != nil {
				return "", 0, fmt.Errorf("decrypt rkey: %w", err)
			}
			return dec, rk.TS, nil
		}
	}
	if sk.Key == "" {
		return "", 0, fmt.Errorf("group has no symmetric key")
	}
	dec, err := kp.RSADecryptBase64(sk.Key)
	if err != nil {
		return "", 0, fmt.Errorf("decrypt group key: %w", err)
	}
	return dec, 0, nil
}

func decryptPairingCode(qr PairQR, firstBind bool) (rid, evalue string, err error) {
	try := func(fb bool) (string, string, error) {
		plain, err := DecryptQRPayload(qr.License, qr.Seed, qr.Ek, fb)
		if err != nil {
			return "", "", err
		}
		plain = strings.TrimSpace(plain)
		if !isPrintableASCII(plain) {
			return "", "", fmt.Errorf("%w: non-printable rendezvous payload", ErrCrypto)
		}
		var obj struct {
			R      string          `json:"r"`
			RID    string          `json:"rid"`
			EValue json.RawMessage `json:"evalue"`
		}
		if json.Unmarshal([]byte(plain), &obj) == nil && (obj.R != "" || obj.RID != "") {
			rid := firstNonEmpty(obj.R, obj.RID)
			ev := string(obj.EValue)
			if strings.TrimSpace(ev) == "" {
				ev = mustJSON(map[string]string{"license": qr.License})
			} else if len(ev) > 0 && ev[0] == '{' {
				// already a JSON object — keep compact
				var tmp any
				if json.Unmarshal(obj.EValue, &tmp) == nil {
					ev = mustJSON(tmp)
				}
			}
			return rid, ev, nil
		}
		// Older clients: ek decrypts to bare rendezvous id string.
		return plain, mustJSON(map[string]string{"license": qr.License}), nil
	}
	rid, evalue, err = try(firstBind)
	if err == nil {
		return rid, evalue, nil
	}
	rid, evalue, err2 := try(!firstBind)
	if err2 == nil {
		return rid, evalue, nil
	}
	return "", "", err
}

func isPrintableASCII(s string) bool {
	if len(s) < 8 || len(s) > 512 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 32 || s[i] > 126 {
			return false
		}
	}
	return true
}

func parseQR(raw string) (PairQR, error) {
	var q PairQR
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return q, ErrBadQR
	}
	if err := json.Unmarshal([]byte(raw), &q); err != nil {
		return q, fmt.Errorf("%w: %v", ErrBadQR, err)
	}
	if q.GID == "" || q.Seed == "" || q.Ek == "" || q.License == "" {
		return q, ErrBadQR
	}
	return q, nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
