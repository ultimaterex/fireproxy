package tplink

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"fireproxy/pkg/inventory"
	"fireproxy/server/internal/store"
)

func TestLooksLikeEasySmartLogin(t *testing.T) {
	html := testdata("system_info.html") // not login — should be false
	if LooksLikeEasySmartLogin(html) {
		t.Fatal("system info is not login")
	}
	login := `<script>var logonInfo = new Array(0,0,0);</script><form action="/logon.cgi"><input name="username">`
	if !LooksLikeEasySmartLogin(login) {
		t.Fatal("expected easy smart login")
	}
	deco := `<html><title>Deco</title><form action="/login"></form>`
	if LooksLikeEasySmartLogin(deco) {
		t.Fatal("deco should not match")
	}
}

func TestProbeEasySmartGET(t *testing.T) {
	login := `<script>var logonInfo = new Array(0,0,0);</script><form action="/logon.cgi"><input id="username" name="username">`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(login))
	}))
	defer srv.Close()
	ip := stringsTrimHost(srv.URL)
	if !ProbeEasySmartGET(ip, srv.Client()) {
		t.Fatal("expected positive probe")
	}
}

func stringsTrimHost(u string) string {
	u = stringsTrimPrefix(u, "http://")
	u = stringsTrimPrefix(u, "https://")
	return u
}

func stringsTrimPrefix(s, p string) string {
	if len(s) >= len(p) && s[:len(p)] == p {
		return s[len(p):]
	}
	return s
}

func TestProbeCacheRemembers(t *testing.T) {
	p, err := store.OpenPersist(filepath.Join(t.TempDir(), "t.db"), 7)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	cache := NewProbeCache(p)
	cache.Set(ProbeResult{MAC: "aa:bb:cc:dd:ee:10", IP: "203.0.113.10", EasySmart: true, Via: "http"})
	ok, cached := cache.Resolve("aa:bb:cc:dd:ee:10", "203.0.113.10")
	if !ok || !cached {
		t.Fatalf("cache hit ok=%v cached=%v", ok, cached)
	}
	// Legacy hint-only positive must re-probe (no httptest here → expect false after failed dial)
	cache.Set(ProbeResult{MAC: "02:00:00:00:00:99", IP: "127.0.0.1:1", EasySmart: true})
	ok, cached = cache.Resolve("02:00:00:00:00:99", "127.0.0.1:1")
	if ok || cached {
		t.Fatalf("legacy hint should re-probe and fail ok=%v cached=%v", ok, cached)
	}
	cache.Set(ProbeResult{MAC: "AA:BB:CC:00:00:01", IP: "1.2.3.4", EasySmart: false})
	ok, cached = cache.Resolve("AA:BB:CC:00:00:01", "1.2.3.4")
	if ok || !cached {
		t.Fatalf("negative ok=%v cached=%v", ok, cached)
	}
}

func TestCandidatesWithProbeHint(t *testing.T) {
	mk := func(mac, ip, name, vendor, typ string) inventory.Device {
		d := inventory.Device{}
		d.MAC, d.IP, d.Name, d.Vendor, d.Type = mac, ip, name, vendor, typ
		return d
	}
	cat := inventory.Catalog{Devices: []inventory.Device{
		mk("AA:BB:CC:DD:EE:10", "192.168.1.10", "Office Switch", "TP-Link", ""), // no type, no TL-SG name
		mk("02:00:00:00:00:51", "192.168.1.190", "Desk Energy (KP115)", "TP-Link", "sensor&automation"),
	}}
	// Without cache, only name/type hints — Office Switch should NOT appear
	if cands := CandidatesFromCatalog(cat, nil, nil); len(cands) != 0 {
		t.Fatalf("nil cache should not include unnamed type-less: %+v", cands)
	}
	p, err := store.OpenPersist(filepath.Join(t.TempDir(), "t.db"), 7)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	cache := NewProbeCache(p)
	// Pre-seed positive for office switch (simulates successful soft probe)
	cache.Set(ProbeResult{MAC: "AA:BB:CC:DD:EE:10", IP: "192.168.1.10", EasySmart: true, Via: "http"})
	cache.Set(ProbeResult{MAC: "02:00:00:00:00:51", IP: "192.168.1.190", EasySmart: false})
	cands := CandidatesFromCatalog(cat, nil, cache)
	if len(cands) != 1 || cands[0].MAC != "AA:BB:CC:DD:EE:10" {
		t.Fatalf("%+v", cands)
	}
}

func TestCandidatesSwitchOnly(t *testing.T) {
	mk := func(mac, ip, name, vendor, typ string) inventory.Device {
		d := inventory.Device{}
		d.MAC, d.IP, d.Name, d.Vendor, d.Type = mac, ip, name, vendor, typ
		return d
	}
	cat := inventory.Catalog{Devices: []inventory.Device{
		mk("AA:BB:CC:DD:EE:10", "192.168.1.10", "TL-SG1016PE", "TP-Link", "switch"),
		mk("02:00:00:00:00:51", "192.168.1.190", "Desk Energy (KP115)", "TP-Link Systems Inc", "sensor&automation"),
		mk("02:00:00:00:00:52", "192.168.1.98", "Deco XE75 Living Room", "TP-Link Systems Inc", "ap"),
		mk("AA:BB:CC:00:00:99", "192.168.1.9", "TL-SG108E", "TP-Link", ""),
	}}
	cands := CandidatesFromCatalog(cat, nil, nil)
	if len(cands) != 2 {
		t.Fatalf("want 2 switches, got %+v", cands)
	}
}

func TestLooksLikeSwitch(t *testing.T) {
	if !LooksLikeSwitch("switch", "whatever") {
		t.Fatal("type switch")
	}
	if !LooksLikeSwitch("", "TL-SG1016PE") {
		t.Fatal("model name")
	}
	if LooksLikeSwitch("sensor&automation", "Desk Energy (KP115)") {
		t.Fatal("plug")
	}
}
