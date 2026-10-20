package fwapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"fireproxy/server/internal/tplink"
)

func TestPruneSpeedJobs(t *testing.T) {
	svc := &Service{speedJobs: map[string]*SpeedtestJob{}}
	now := time.Now()
	svc.speedJobs["old"] = &SpeedtestJob{ID: "old", State: "done", updatedAt: now.Add(-2 * speedJobTTL)}
	svc.speedJobs["fresh"] = &SpeedtestJob{ID: "fresh", State: "done", updatedAt: now}
	svc.speedJobs["run"] = &SpeedtestJob{ID: "run", State: "running", updatedAt: now.Add(-2 * speedJobTTL)}
	svc.pruneSpeedJobsLocked(now)
	if _, ok := svc.speedJobs["old"]; ok {
		t.Fatal("old done job should be pruned")
	}
	if _, ok := svc.speedJobs["fresh"]; !ok {
		t.Fatal("fresh done job kept")
	}
	if _, ok := svc.speedJobs["run"]; !ok {
		t.Fatal("running job kept despite age")
	}
	for i := 0; i < speedJobMax+5; i++ {
		id := fmt.Sprintf("x%d", i)
		svc.speedJobs[id] = &SpeedtestJob{ID: id, State: "error", updatedAt: now.Add(-time.Duration(i) * time.Second)}
	}
	svc.pruneSpeedJobsLocked(now)
	if len(svc.speedJobs) > speedJobMax {
		t.Fatalf("len=%d want <= %d", len(svc.speedJobs), speedJobMax)
	}
	if _, ok := svc.speedJobs["run"]; !ok {
		t.Fatal("running job must survive max prune")
	}
}

func TestNormalizeBoxIP(t *testing.T) {
	ok, err := NormalizeBoxIP("http://192.168.1.1/foo")
	if err != nil || ok != "192.168.1.1" {
		t.Fatalf("%q %v", ok, err)
	}
	ok, err = NormalizeBoxIP("[2001:db8::1]:8833")
	if err != nil || ok != "2001:db8::1" {
		t.Fatalf("%q %v", ok, err)
	}
	if _, err := NormalizeBoxIP("127.0.0.1@evil.com"); err == nil {
		t.Fatal("expected reject userinfo host")
	}
	if _, err := NormalizeBoxIP("firewalla.lan"); err == nil {
		t.Fatal("expected reject hostname")
	}
	if boxLANURLHost("2001:db8::1") != "[2001:db8::1]" {
		t.Fatal("ipv6 brackets")
	}
	if boxLANURLHost("192.168.1.1") != "192.168.1.1" {
		t.Fatal("ipv4 plain")
	}
}

func TestParseMAC(t *testing.T) {
	got, err := ParseMAC("aa-bb-cc-dd-ee-ff")
	if err != nil || got != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := ParseMAC("aa:bb:cc:dd:ee:ff"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseMAC("aabb.ccdd.eeff"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseMAC("nope"); err == nil {
		t.Fatal("expected reject")
	}
	if _, err := ParseMAC("aa-bb-cc-dd-ee-ff;foo"); err == nil {
		t.Fatal("expected reject trailing junk")
	}
}

func TestNormalizeHostDNS(t *testing.T) {
	got, err := NormalizeHostDNS("Paytons.Chromebook.3")
	if err != nil || got != "paytons.chromebook.3" {
		t.Fatalf("%q %v", got, err)
	}
	got, err = NormalizeHostDNS("  ")
	if err != nil || got != "" {
		t.Fatalf("clear %q %v", got, err)
	}
	if _, err := NormalizeHostDNS("bad host!"); err == nil {
		t.Fatal("expected reject")
	}
}

func TestParseOoklaServers(t *testing.T) {
	raw := []byte(`[{"id":"38427","name":"Balona","country":"Suriname","sponsor":"Telesur","host":"balona.speedtest.sr:8080","distance":4},{"id":70519,"sponsor":"Parbonet","name":"Paramaribo"}]`)
	got, err := parseOoklaServers(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "38427" || got[0].Sponsor != "Telesur" || got[1].ID != "70519" {
		t.Fatalf("%+v", got)
	}
}

func TestParseSpeedtestResults(t *testing.T) {
	raw := json.RawMessage(`{"code":200,"data":{"results":[
		{"uuid":"wan-a","success":true,"timestamp":1700000000,"result":{"download":619,"upload":410,"latency":11}},
		{"wanUUID":"wan-b","success":true,"timestamp":1700000100,"result":{"download":28,"upload":6,"latency":24}},
		{"uuid":"wan-a","success":false,"timestamp":1700000200,"result":{"download":1,"upload":1,"latency":1}}
	]}}`)
	got, err := parseSpeedtestResults(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d %+v", len(got), got)
	}
	if got[0].WanUUID != "wan-a" || got[0].Down != 619 || got[0].TS != 1700000000 {
		t.Fatalf("a %+v", got[0])
	}
	if got[1].WanUUID != "wan-b" || got[1].Down != 28 {
		t.Fatalf("b %+v", got[1])
	}
}

func TestParseSpeedtestReplyHistoryShape(t *testing.T) {
	raw := json.RawMessage(`{"uuid":"wan-a","success":true,"timestamp":1700000000,"server":{"id":"38427","sponsor":"Telesur","location":"Balona"},"result":{"download":619,"upload":410,"latency":11}}`)
	got, err := parseSpeedtestReply(raw, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.WanUUID != "wan-a" || got.Down != 619 || got.TS != 1700000000 {
		t.Fatalf("%+v", got)
	}
	if got.ServerID != "38427" || got.Server != "Telesur" || got.Location != "Balona" {
		t.Fatalf("server %+v", got)
	}
}

func TestAESLegacyRoundTrip(t *testing.T) {
	key := strings.Repeat("k", 40)
	plain := `{"mtype":"init","obj":{"item":"init"}}`
	ct, err := AESEncryptLegacy(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	got, err := AESDecryptMessage(key, ct)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("got %q", got)
	}
}

func TestAESEnvelopeRoundTrip(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	plain := "hello-firewalla"
	ct, err := AESEncryptLegacy(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	env, _ := json.Marshal(map[string]string{
		"iv":      "AAAAAAAAAAAAAAAAAAAAAA==",
		"message": ct,
	})
	got, err := AESDecryptMessage(key, string(env))
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("got %q", got)
	}
}

func TestCredentialVaultRoundTrip(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &CredentialVault{Store: NewMemStore(), Key: key}
	in := Creds{
		PairedAt: time.Now().UTC().Truncate(time.Second),
		BoxIP:    "192.168.1.1",
		Gid:      "gid-abc",
		Eid:      "eid-1",
		SymKey:   "sym-key-material-xxxxxxxxxxxxxxxx",
		Email:    "a@b.co",
	}
	if err := v.Save(in); err != nil {
		t.Fatal(err)
	}
	out, ok, err := v.Load()
	if err != nil || !ok {
		t.Fatalf("load: %v ok=%v", err, ok)
	}
	if out.BoxIP != in.BoxIP || out.Gid != in.Gid || out.SymKey != in.SymKey {
		t.Fatalf("%+v", out)
	}
	if err := v.Clear(); err != nil {
		t.Fatal(err)
	}
	_, ok, err = v.Load()
	if err != nil || ok {
		t.Fatalf("cleared: ok=%v err=%v", ok, err)
	}
}

func TestPairRequiresLAN(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("cd", 32))
	if err != nil {
		t.Fatal(err)
	}
	sym := strings.Repeat("s", 32)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, err := AESDecryptMessage(sym, body.Message); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		reply, _ := AESEncryptLegacy(sym, `{"code":200,"data":{"ok":true}}`)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": reply})
	}))
	defer ts.Close()
	base, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	v := &CredentialVault{Store: NewMemStore(), Key: key}
	lan := NewLANClient()
	lan.HTTP = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			u := *base
			u.Path = req.URL.Path
			req2 := req.Clone(req.Context())
			req2.URL = &u
			req2.Host = u.Host
			return http.DefaultTransport.RoundTrip(req2)
		}),
	}
	svc := NewServiceWithVault(v, lan)
	svc.SetPairFn(func(ctx context.Context, req PairRequest) (Creds, error) {
		return Creds{
			PairedAt: time.Now().UTC(),
			BoxIP:    "127.0.0.1",
			Gid:      "g1",
			Eid:      "e1",
			SymKey:   sym,
			Email:    req.Email,
		}, nil
	})

	st, err := svc.Pair(context.Background(), PairRequest{
		BoxIP: "127.0.0.1",
		Email: "t@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "lan-ok" || !st.Paired {
		t.Fatalf("%+v", st)
	}
	st2, err := svc.Ping(context.Background())
	if err != nil || st2.State != "lan-ok" {
		t.Fatalf("ping: %v %+v", err, st2)
	}
	if err := svc.Unpair(); err != nil {
		t.Fatal(err)
	}
	if svc.Status().Paired {
		t.Fatal("still paired")
	}
}

func TestPairFailsWhenLANDown(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("ef", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &CredentialVault{Store: NewMemStore(), Key: key}
	lan := NewLANClient()
	lan.HTTP = &http.Client{
		Timeout: 200 * time.Millisecond,
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}),
	}
	svc := NewServiceWithVault(v, lan)
	svc.SetPairFn(func(ctx context.Context, req PairRequest) (Creds, error) {
		return Creds{
			BoxIP:  "10.255.255.1",
			Gid:    "g1",
			Eid:    "e1",
			SymKey: strings.Repeat("s", 32),
			Email:  req.Email,
		}, nil
	})
	st, err := svc.Pair(context.Background(), PairRequest{BoxIP: "10.255.255.1", Email: "t@example.com"})
	if err == nil {
		t.Fatal("expected LAN failure")
	}
	if !st.Paired || st.State != "lan-down" {
		t.Fatalf("%+v", st)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
