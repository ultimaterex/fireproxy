package collect

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"fireproxy/pkg/snapshot"
)

type memRedis struct {
	hget     map[string]map[string]string
	hgetall  map[string]map[string]string
	gets     map[string]string
	zcard    map[string]int64
	zrev     map[string][]ZMember
	hgetN    map[string]int
	hgetallN map[string]int
}

func (m *memRedis) HGet(key, field string) (string, error) {
	if m.hgetN == nil {
		m.hgetN = map[string]int{}
	}
	m.hgetN[key+" "+field]++
	if m.hget != nil {
		if v, ok := m.hget[key][field]; ok {
			return v, nil
		}
	}
	if m.hgetall != nil {
		return m.hgetall[key][field], nil
	}
	return "", nil
}

func (m *memRedis) HGetAll(key string) (map[string]string, error) {
	if m.hgetallN == nil {
		m.hgetallN = map[string]int{}
	}
	m.hgetallN[key]++
	if m.hgetall == nil {
		return map[string]string{}, nil
	}
	return m.hgetall[key], nil
}

func TestParseCPUList(t *testing.T) {
	cases := map[string]int{
		"0-3":   4,
		"0":     1,
		"0,2-3": 3,
		"":      0,
	}
	for in, want := range cases {
		if got := parseCPUList(in); got != want {
			t.Fatalf("%q: got %d want %d", in, got, want)
		}
	}
}

func TestCollectSysfsAndLoad(t *testing.T) {
	root := t.TempDir()
	sysfs := filepath.Join(root, "sys")
	mustWrite(t, filepath.Join(sysfs, "class", "net", "eth1", "statistics", "rx_bytes"), "1000\n")
	mustWrite(t, filepath.Join(sysfs, "class", "net", "eth1", "statistics", "tx_bytes"), "2000\n")
	mustWrite(t, filepath.Join(sysfs, "class", "net", "eth1", "carrier"), "1\n")
	mustWrite(t, filepath.Join(sysfs, "class", "net", "eth1", "speed"), "1000\n")
	mustWrite(t, filepath.Join(sysfs, "class", "net", "br0", "statistics", "rx_bytes"), "10\n")
	mustWrite(t, filepath.Join(sysfs, "class", "net", "br0", "statistics", "tx_bytes"), "20\n")
	mustWrite(t, filepath.Join(sysfs, "class", "net", "br0", "carrier"), "1\n")
	// bridges often have no readable speed — omit speed file

	loadPath := filepath.Join(root, "loadavg")
	mustWrite(t, loadPath, "1.5 2.5 3.5 2/100 1\n")
	cpuOnline := filepath.Join(root, "cpu_online")
	mustWrite(t, cpuOnline, "0-3\n")

	r := &memRedis{
		hget: map[string]map[string]string{
			"stats:systemd:restart": {"firerouter_dns": "42"},
		},
		hgetall: map[string]map[string]string{
			"event:state:cache": {
				"wan_state:eth1": `{"name":"ISP A","ready":true,"active":true}`,
			},
		},
	}

	aclPath := filepath.Join(root, "acl.log")
	mustWrite(t, aclPath, "noise\n[ACL][Blocked] one\n[ACL][Allowed] x\n[ACL][Blocked] two\n")

	c := &Collector{
		Hostname:      "Firewalla",
		Ifaces:        []string{"eth1", "br0", "missing0"},
		SysfsRoot:     sysfs,
		ProcLoadPath:  loadPath,
		CPUOnlinePath: cpuOnline,
		Redis:         r,
		ACL:           &ACLTracker{Path: aclPath},
	}

	snap, err := c.Collect(time.Unix(1700000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if snap.Load.M5 != 2.5 {
		t.Fatalf("load: %+v", snap.Load)
	}
	if snap.Load.Cores != 4 {
		t.Fatalf("cores: %d", snap.Load.Cores)
	}
	if snap.DNSRestarts != 42 {
		t.Fatalf("dns_restarts: %d", snap.DNSRestarts)
	}
	eth1 := snap.Ifaces["eth1"]
	if eth1.RxBytes != 1000 || eth1.SpeedMbps == nil || *eth1.SpeedMbps != 1000 || !eth1.Carrier {
		t.Fatalf("eth1: %+v", eth1)
	}
	br0 := snap.Ifaces["br0"]
	if br0.SpeedMbps != nil {
		t.Fatalf("br0 speed should be nil: %+v", br0)
	}
	if _, ok := snap.Ifaces["missing0"]; ok {
		t.Fatal("missing iface should be skipped")
	}
	wan := snap.WAN["eth1"]
	if wan.Name != "ISP A" || !wan.Ready || !wan.Active {
		t.Fatalf("wan: %+v", wan)
	}
	if snap.DNSBlocksDelta == nil || *snap.DNSBlocksDelta != 2 {
		t.Fatalf("acl delta: %v", snap.DNSBlocksDelta)
	}

	// Second collect: no new lines
	snap2, err := c.Collect(time.Unix(1700000060, 0))
	if err != nil {
		t.Fatal(err)
	}
	if snap2.DNSBlocksDelta == nil || *snap2.DNSBlocksDelta != 0 {
		t.Fatalf("second acl delta: %v", snap2.DNSBlocksDelta)
	}
}

func TestParseWANFromFirewallaEventCache(t *testing.T) {
	got := parseWANFromEventCache(map[string]string{
		"wan_state:eth1": `{
			"ts":1,"event_type":"state","state_type":"wan_state","state_key":"eth1","state_value":0,
			"labels":{"ready":true,"active":true,"wan_intf_name":"ISP A","failures":[]}
		}`,
		"wan_state:eth2": `{
			"ts":1,"event_type":"state","state_type":"wan_state","state_key":"eth2","state_value":1,
			"labels":{"ready":false,"active":false,"wan_intf_name":"ISP B"}
		}`,
		"wan_state:eth1.100":                  `{"state_value":1,"labels":{"ready":false}}`,
		"overall_wan_state:overall_wan_state": `{"state_value":0}`,
	})
	if !got["eth1"].Ready || !got["eth1"].Active || got["eth1"].Name != "ISP A" {
		t.Fatalf("eth1 %+v", got["eth1"])
	}
	if got["eth2"].Ready || got["eth2"].Name != "ISP B" {
		t.Fatalf("eth2 %+v", got["eth2"])
	}
	if _, ok := got["overall_wan_state"]; ok {
		t.Fatal("overall_wan_state should be ignored")
	}
	if _, ok := got["eth1.100"]; ok {
		t.Fatal("vlan wan should be ignored")
	}
}

func TestCollectDNSFromDnsmasqProc(t *testing.T) {
	proc := t.TempDir()
	mustWrite(t, filepath.Join(proc, "uptime"), "1000.00 0.00\n")
	mustWrite(t, filepath.Join(proc, "100", "comm"), "dnsmasq\n")
	mustWrite(t, filepath.Join(proc, "100", "stat"), fakeProcStat("dnsmasq", 20000))
	mustWrite(t, filepath.Join(proc, "net", "udp"), "  sl  local_address rem_address   st\n   0: 00000000:0035 00000000:0000 07\n")
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "loadavg"), "0.1 0.1 0.1 1/10 1\n")
	c := &Collector{
		Hostname:     "TestBox",
		ProcLoadPath: filepath.Join(root, "loadavg"),
		ProcRoot:     proc,
	}
	now := time.Unix(1_700_003_600, 0)
	snap, err := c.Collect(now)
	if err != nil {
		t.Fatal(err)
	}
	// starttime 20000 ticks / 100 Hz = 200s, uptime 1000s → age 800s
	dnsmasq := dnsByName(snap.DNSSvcs, "dnsmasq")
	if dnsmasq == nil || !dnsmasq.OK || dnsmasq.Since != now.Unix()-800 {
		t.Fatalf("dnsmasq %+v svcs %+v", dnsmasq, snap.DNSSvcs)
	}
	if fr := dnsByName(snap.DNSSvcs, "firerouter"); fr == nil || fr.OK {
		t.Fatalf("firerouter %+v", fr)
	}
	if dnsByName(snap.DNSSvcs, "unbound") != nil {
		t.Fatal("unbound should be omitted when disabled")
	}
}

func TestCollectDNSStack(t *testing.T) {
	proc := t.TempDir()
	mustWrite(t, filepath.Join(proc, "uptime"), "1000.00 0.00\n")
	mustWrite(t, filepath.Join(proc, "100", "comm"), "dnsmasq\n")
	mustWrite(t, filepath.Join(proc, "100", "cmdline"), "dnsmasq\x00-C\x00/home/pi/firerouter/etc/dnsmasq.dns.br0.conf\x00")
	mustWrite(t, filepath.Join(proc, "100", "stat"), fakeProcStat("dnsmasq", 20000))
	mustWrite(t, filepath.Join(proc, "101", "comm"), "unbound\n")
	mustWrite(t, filepath.Join(proc, "101", "stat"), fakeProcStat("unbound", 10000))
	mustWrite(t, filepath.Join(proc, "102", "comm"), "node\n")
	mustWrite(t, filepath.Join(proc, "102", "cmdline"), "/usr/bin/node\x00/home/pi/firerouter/bin/www\x00")
	mustWrite(t, filepath.Join(proc, "102", "stat"), fakeProcStat("node", 5000))
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "loadavg"), "0.1 0.1 0.1 1/10 1\n")
	c := &Collector{
		Hostname:     "TestBox",
		ProcLoadPath: filepath.Join(root, "loadavg"),
		ProcRoot:     proc,
		Redis:        &memRedis{hget: map[string]map[string]string{"sys:features": {"unbound": "1"}}},
	}
	snap, err := c.Collect(time.Unix(1_700_003_600, 0))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"unbound", "dnsmasq", "firerouter"} {
		s := dnsByName(snap.DNSSvcs, name)
		if s == nil || !s.OK {
			t.Fatalf("%s %+v svcs %+v", name, s, snap.DNSSvcs)
		}
	}
}

func TestCollectDNSDownWithoutListener(t *testing.T) {
	proc := t.TempDir()
	mustWrite(t, filepath.Join(proc, "uptime"), "10.00 0.00\n")
	mustWrite(t, filepath.Join(proc, "net", "udp"), "  sl  local_address rem_address   st\n")
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "loadavg"), "0.1 0.1 0.1 1/10 1\n")
	c := &Collector{Hostname: "TestBox", ProcLoadPath: filepath.Join(root, "loadavg"), ProcRoot: proc}
	snap, err := c.Collect(time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if s := dnsByName(snap.DNSSvcs, "dnsmasq"); s == nil || s.OK {
		t.Fatalf("dnsmasq %+v", s)
	}
	if s := dnsByName(snap.DNSSvcs, "firerouter"); s == nil || s.OK {
		t.Fatalf("firerouter %+v", s)
	}
}

func dnsByName(svcs []snapshot.DNSSvc, name string) *snapshot.DNSSvc {
	for i := range svcs {
		if svcs[i].Name == name {
			return &svcs[i]
		}
	}
	return nil
}

func TestParseUnboundStats(t *testing.T) {
	raw := "thread0.num.queries=10\ntotal.num.queries=1200\ntotal.num.cachehits=900\ntotal.num.cachemiss=300\n"
	got, ok := parseUnboundStats(raw)
	if !ok || got.Queries != 1200 || got.Hits != 900 || got.Misses != 300 || got.HitPct < 74.9 || got.HitPct > 75.1 {
		t.Fatalf("%+v ok=%v", got, ok)
	}
}

func TestCollectUnboundStats(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "loadavg"), "0.1 0.1 0.1 1/10 1\n")
	c := &Collector{
		Hostname:     "TestBox",
		ProcLoadPath: filepath.Join(root, "loadavg"),
		UnboundStats: func() (string, error) {
			return "total.num.queries=40\ntotal.num.cachehits=30\ntotal.num.cachemiss=10\n", nil
		},
	}
	snap, err := c.Collect(time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if snap.Unbound == nil || snap.Unbound.Queries != 40 || snap.Unbound.HitPct < 74.9 {
		t.Fatalf("%+v", snap.Unbound)
	}
}

func TestUnboundControlBinPrefersFirewallaPath(t *testing.T) {
	if unboundControlBin() != "unbound-control" && unboundControlBin() != firewallaUnboundControl {
		t.Fatalf("unexpected bin %q", unboundControlBin())
	}
	// On this host the Firewalla path is absent → PATH name.
	if _, err := os.Stat(firewallaUnboundControl); err == nil {
		if unboundControlBin() != firewallaUnboundControl {
			t.Fatalf("want firewalla path, got %q", unboundControlBin())
		}
	} else if unboundControlBin() != "unbound-control" {
		t.Fatalf("want unbound-control, got %q", unboundControlBin())
	}
}

func TestWithLDLibraryPath(t *testing.T) {
	got := withLDLibraryPath([]string{"FOO=1", "LD_LIBRARY_PATH=/other"}, "/home/pi/.firewalla/run/unbound")
	want := "LD_LIBRARY_PATH=/home/pi/.firewalla/run/unbound" + string(os.PathListSeparator) + "/other"
	found := false
	for _, e := range got {
		if e == want {
			found = true
		}
		if e == "FOO=1" {
			continue
		}
	}
	if !found {
		t.Fatalf("got %v want %s", got, want)
	}
	got = withLDLibraryPath([]string{"FOO=1"}, "/lib")
	if got[len(got)-1] != "LD_LIBRARY_PATH=/lib" {
		t.Fatalf("%v", got)
	}
}

func fakeProcStat(comm string, startTicks int) string {
	// /proc/pid/stat: after ") " field[0]=state, field[19]=starttime (kernel field 22).
	fields := make([]string, 22)
	fields[0] = "S"
	for i := 1; i < 19; i++ {
		fields[i] = "0"
	}
	fields[19] = strconv.Itoa(startTicks)
	fields[20] = "0"
	fields[21] = "0"
	return "100 (" + comm + ") " + strings.Join(fields, " ") + "\n"
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
