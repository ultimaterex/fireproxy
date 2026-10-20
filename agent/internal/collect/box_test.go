package collect

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fireproxy/pkg/inventory"
)

func TestCollectBoxFromRedisAndRelease(t *testing.T) {
	dir := t.TempDir()
	rel := filepath.Join(dir, "firewalla_release")
	if err := os.WriteFile(rel, []byte("VERSION=1.983\nPLATFORM=goldse\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &memRedis{
		hgetall: map[string]map[string]string{
			"sys:ept":    {"eid": "test-eid", "token": "SECRET"},
			"sys:config": {"timezone": "UTC"},
			"sys:network:info": {
				"publicIp": "203.0.113.10",
				"ddns":     "box.example.com",
				"eth1":     `{"name":"eth1","mac_address":"02:00:00:00:00:01","type":"wan","uuid":"w1"}`,
				"eth2":     `{"name":"eth2","mac":"02:00:00:00:00:02","type":"wan","uuid":"w2"}`,
			},
		},
		gets: map[string]string{
			"mode":                "router",
			"groupName":           "HomeLab",
			"local:domain:suffix": "LAN",
		},
	}
	// SysfsRoot points at the (interface-free) tempdir so the sysfs MAC branch
	// yields nothing and the redis sys:network:info fallback is exercised.
	// Without this the collector reads the host's real /sys and returns its
	// interfaces instead of the test fixture.
	c := &Collector{Hostname: "TestBox", Redis: r, ReleasePath: rel, SysfsRoot: dir}
	cat, err := c.CollectCatalog(time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	b := cat.Box
	if b == nil {
		t.Fatal("box missing")
	}
	if b.Name != "HomeLab" || b.PublicIP != "203.0.113.10" || b.EID != "test-eid" {
		t.Fatalf("%+v", b)
	}
	if b.Timezone != "UTC" || b.Mode != "router" {
		t.Fatalf("%+v", b)
	}
	if b.LocalDomainSuffix != "lan" {
		t.Fatalf("local_domain_suffix %+v", b.LocalDomainSuffix)
	}
	if b.Version != "1.983" || b.License != "Firewalla Gold SE" {
		t.Fatalf("%+v", b)
	}
	if len(b.MACs) != 2 || b.MACs[0].Name == "" || b.MACs[0].MAC == "" {
		t.Fatalf("macs %+v", b.MACs)
	}
	if b.EID == "SECRET" {
		t.Fatal("token leaked into eid")
	}
}

func TestCollectBoxMACsFromSysfsAndHyphenRelease(t *testing.T) {
	dir := t.TempDir()
	rel := filepath.Join(dir, "firewalla-release")
	if err := os.WriteFile(rel, []byte("BOARD=gold-se\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeAddr := func(relPath, mac string) {
		p := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(mac+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeAddr("class/net/eth0/address", "02:00:00:00:00:04")
	writeAddr("class/net/eth1/address", "02:00:00:00:00:03")
	writeAddr("class/net/eth2/address", "02:00:00:00:00:02")
	writeAddr("class/net/eth3/address", "02:00:00:00:00:01")
	writeAddr("class/net/wlan0/address", "02:00:00:00:00:10")
	writeAddr("class/net/wlan1/address", "02:00:00:00:00:11")
	writeAddr("class/bluetooth/hci0/address", "02:00:00:00:00:20")
	r := &memRedis{
		hgetall: map[string]map[string]string{
			"sys:network:info": {
				"publicIp": `"203.0.113.10"`,
				"br0":      `{"name":"br0","mac_address":"02:00:00:00:00:ff","type":"lan","uuid":"x"}`,
			},
		},
	}
	c := &Collector{Hostname: "TestBox", Redis: r, ReleasePath: rel, SysfsRoot: dir}
	cat, err := c.CollectCatalog(time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	b := cat.Box
	if b.PublicIP != "203.0.113.10" {
		t.Fatalf("public ip %q", b.PublicIP)
	}
	if b.License != "Firewalla Gold SE" {
		t.Fatalf("license %q", b.License)
	}
	want := []inventory.BoxMAC{
		{Name: "Ethernet Port 1", MAC: "02:00:00:00:00:01"},
		{Name: "Ethernet Port 2", MAC: "02:00:00:00:00:02"},
		{Name: "Ethernet Port 3", MAC: "02:00:00:00:00:03"},
		{Name: "Ethernet Port 4", MAC: "02:00:00:00:00:04"},
		{Name: "Wi-Fi", MAC: "02:00:00:00:00:10"},
		{Name: "Bluetooth", MAC: "02:00:00:00:00:20"},
	}
	if len(b.MACs) != len(want) {
		t.Fatalf("macs %+v", b.MACs)
	}
	for i := range want {
		if b.MACs[i] != want[i] {
			t.Fatalf("mac %d %+v want %+v", i, b.MACs[i], want[i])
		}
	}
}

func TestCollectBoxBluetoothFromRedisWhenHCIMissing(t *testing.T) {
	dir := t.TempDir()
	rel := filepath.Join(dir, "firewalla-release")
	if err := os.WriteFile(rel, []byte("BOARD=gold-se\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeAddr := func(relPath, mac string) {
		p := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(mac+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeAddr("class/net/eth0/address", "02:00:00:00:00:04")
	writeAddr("class/net/eth1/address", "02:00:00:00:00:03")
	writeAddr("class/net/eth2/address", "02:00:00:00:00:02")
	writeAddr("class/net/eth3/address", "02:00:00:00:00:01")
	writeAddr("class/net/wlan0/address", "02:00:00:00:00:10")
	r := &memRedis{
		hgetall: map[string]map[string]string{
			"sys:network:info": {"publicIp": `"203.0.113.10"`},
		},
		gets: map[string]string{"sys:bt:mac": "02:00:00:00:00:20"},
	}
	c := &Collector{Hostname: "TestBox", Redis: r, ReleasePath: rel, SysfsRoot: dir}
	cat, err := c.CollectCatalog(time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	got := cat.Box.MACs
	if len(got) != 6 || got[5] != (inventory.BoxMAC{Name: "Bluetooth", MAC: "02:00:00:00:00:20"}) {
		t.Fatalf("macs %+v", got)
	}
}

func TestReadJSONVersionNumeric(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	if err := os.WriteFile(p, []byte(`{"version": 1.983, "other": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readJSONVersion(p); got != "1.983" {
		t.Fatalf("numeric version: %q", got)
	}
	if err := os.WriteFile(p, []byte(`{"version": "1.984"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readJSONVersion(p); got != "1.984" {
		t.Fatalf("string version: %q", got)
	}
}

func TestCollectBoxVersionFromConfigJSON(t *testing.T) {
	dir := t.TempDir()
	rel := filepath.Join(dir, "firewalla-release")
	if err := os.WriteFile(rel, []byte("BOARD=gold-se\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfg, []byte(`{"version": 1.983}`), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "firewalla")
	if err := os.MkdirAll(filepath.Join(repo, ".git", "refs", "heads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/release_11_0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "refs", "heads", "release_11_0"), []byte("237fa936aabbccddeeff00112233445566778899\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Collector{
		Hostname:       "TestBox",
		Redis:          &memRedis{},
		ReleasePath:    rel,
		ConfigJSONPath: cfg,
		FirewallaHome:  repo,
	}
	cat, err := c.CollectCatalog(time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if cat.Box.Version != "1.983 (237fa936)" {
		t.Fatalf("version %q", cat.Box.Version)
	}
}

func TestReadGitShortPackedRefs(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/release_11_0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "packed-refs"), []byte("# pack-refs\n237fa936aabbccddeeff00112233445566778899 refs/heads/release_11_0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readGitShort(dir); got != "237fa936" {
		t.Fatalf("packed: %q", got)
	}
}
