package collect

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fireproxy/pkg/snapshot"
)

// RedisReader is the minimal Redis surface the agent needs (no KEYS in a loop).
type RedisReader interface {
	HGet(key, field string) (string, error)
	HGetAll(key string) (map[string]string, error)
	Get(key string) (string, error)
	Scan(pattern string, count int64) ([]string, error)
	HMGet(key string, fields ...string) ([]string, error)
	ZCard(key string) (int64, error)
	ZRevRangeByScore(key, max, min string, offset, count int64) ([]ZMember, error)
}

// ZMember is a sorted-set entry.
type ZMember struct {
	Member string
	Score  float64
}

// ACLTracker counts new [ACL][Blocked] lines since the last read via byte offset.
type ACLTracker struct {
	Path   string
	offset int64
}

// Collector gathers one Snapshot using cheap sources only.
type Collector struct {
	Hostname       string
	Ifaces         []string
	SysfsRoot      string
	ProcLoadPath   string
	CPUOnlinePath  string
	ProcStatPath   string
	CollectCPU     bool
	ProcRoot       string
	Redis          RedisReader
	ACL            *ACLTracker
	ReleasePath    string
	ConfigJSONPath string
	FirewallaHome  string
	FwapcURL       string // default http://127.0.0.1:8841/v1
	FireRouterURL  string // default http://127.0.0.1:8837/v1
	HTTPGet        func(url string) ([]byte, error)
	UnboundStats   func() (string, error)
}

// Collect builds a snapshot for the current tick.
func (c *Collector) Collect(now time.Time) (snapshot.Snapshot, error) {
	snap := snapshot.Snapshot{
		TS:     now.Unix(),
		Host:   c.Hostname,
		Ifaces: make(map[string]snapshot.IfaceStats),
		WAN:    make(map[string]snapshot.WANLink),
	}

	load, err := readLoadavg(c.ProcLoadPath)
	if err != nil {
		return snap, fmt.Errorf("loadavg: %w", err)
	}
	if cores := readCPUCores(c.CPUOnlinePath); cores > 0 {
		load.Cores = cores
	}
	snap.Load = load

	if c.CollectCPU && c.ProcStatPath != "" {
		if cpu, err := sampleCPU(c.ProcStatPath, 100*time.Millisecond); err == nil {
			snap.CPU = &cpu
		}
	}

	for _, iface := range c.Ifaces {
		st, err := readIface(c.SysfsRoot, iface)
		if err != nil {
			// Skip missing ifaces (down / not present).
			continue
		}
		snap.Ifaces[iface] = st
	}

	if c.Redis != nil {
		if v, err := c.Redis.HGet("stats:systemd:restart", "firerouter_dns"); err == nil && v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				snap.DNSRestarts = n
			}
		}
		if all, err := c.Redis.HGetAll("event:state:cache"); err == nil {
			snap.WAN = parseWANFromEventCache(all)
		}
		if info, err := c.Redis.HGetAll("sys:network:info"); err == nil {
			enrichWANFromNetworkInfo(snap.WAN, info)
		}
	}
	snap.DNSSvcs = c.collectDNS(now)
	snap.Unbound = c.collectUnbound()

	if c.ACL != nil && c.ACL.Path != "" {
		if n, err := c.ACL.DeltaBlocked(); err == nil {
			snap.DNSBlocksDelta = &n
		}
	}

	return snap, nil
}

func readLoadavg(path string) (snapshot.Load, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return snapshot.Load{}, err
	}
	fields := strings.Fields(string(b))
	if len(fields) < 3 {
		return snapshot.Load{}, fmt.Errorf("unexpected loadavg: %q", string(b))
	}
	m1, err1 := strconv.ParseFloat(fields[0], 64)
	m5, err5 := strconv.ParseFloat(fields[1], 64)
	m15, err15 := strconv.ParseFloat(fields[2], 64)
	if err1 != nil || err5 != nil || err15 != nil {
		return snapshot.Load{}, fmt.Errorf("parse loadavg: %v %v %v", err1, err5, err15)
	}
	return snapshot.Load{M1: m1, M5: m5, M15: m15}, nil
}

// readCPUCores parses /sys/devices/system/cpu/online (e.g. "0-3", "0,2-3").
func readCPUCores(path string) int {
	if path == "" {
		return 0
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return parseCPUList(strings.TrimSpace(string(b)))
}

func parseCPUList(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if lo, hi, ok := strings.Cut(part, "-"); ok {
			a, err1 := strconv.Atoi(strings.TrimSpace(lo))
			b, err2 := strconv.Atoi(strings.TrimSpace(hi))
			if err1 != nil || err2 != nil || b < a {
				continue
			}
			n += b - a + 1
			continue
		}
		if _, err := strconv.Atoi(part); err == nil {
			n++
		}
	}
	return n
}

func readIface(sysfsRoot, name string) (snapshot.IfaceStats, error) {
	base := filepath.Join(sysfsRoot, "class", "net", name)
	if _, err := os.Stat(base); err != nil {
		return snapshot.IfaceStats{}, err
	}
	rx, err := readUintFile(filepath.Join(base, "statistics", "rx_bytes"))
	if err != nil {
		return snapshot.IfaceStats{}, err
	}
	tx, err := readUintFile(filepath.Join(base, "statistics", "tx_bytes"))
	if err != nil {
		return snapshot.IfaceStats{}, err
	}
	carrier := false
	if v, err := os.ReadFile(filepath.Join(base, "carrier")); err == nil {
		carrier = strings.TrimSpace(string(v)) == "1"
	}
	var speed *int
	if v, err := os.ReadFile(filepath.Join(base, "speed")); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(v))); err == nil && n > 0 {
			speed = &n
		}
	}
	return snapshot.IfaceStats{
		RxBytes:   rx,
		TxBytes:   tx,
		SpeedMbps: speed,
		Carrier:   carrier,
	}, nil
}

func readUintFile(path string) (uint64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
}

func sampleCPU(statPath string, delay time.Duration) (snapshot.CPU, error) {
	a, err := readProcStat(statPath)
	if err != nil {
		return snapshot.CPU{}, err
	}
	time.Sleep(delay)
	b, err := readProcStat(statPath)
	if err != nil {
		return snapshot.CPU{}, err
	}
	du := b.user - a.user
	ds := b.sys - a.sys
	di := b.idle - a.idle
	dsoft := b.softirq - a.softirq
	total := du + ds + di + dsoft + (b.nice - a.nice) + (b.irq - a.irq) + (b.steal - a.steal)
	if total == 0 {
		return snapshot.CPU{}, fmt.Errorf("zero cpu delta")
	}
	f := 100.0 / float64(total)
	return snapshot.CPU{
		User:    float64(du) * f,
		Sys:     float64(ds) * f,
		Idle:    float64(di) * f,
		Softirq: float64(dsoft) * f,
	}, nil
}

type procStat struct {
	user, nice, sys, idle, irq, softirq, steal uint64
}

func readProcStat(path string) (procStat, error) {
	f, err := os.Open(path)
	if err != nil {
		return procStat{}, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			return procStat{}, fmt.Errorf("short cpu line")
		}
		var p procStat
		p.user, _ = strconv.ParseUint(fields[1], 10, 64)
		p.nice, _ = strconv.ParseUint(fields[2], 10, 64)
		p.sys, _ = strconv.ParseUint(fields[3], 10, 64)
		p.idle, _ = strconv.ParseUint(fields[4], 10, 64)
		p.irq, _ = strconv.ParseUint(fields[6], 10, 64)
		p.softirq, _ = strconv.ParseUint(fields[7], 10, 64)
		if len(fields) > 8 {
			p.steal, _ = strconv.ParseUint(fields[8], 10, 64)
		}
		return p, nil
	}
	return procStat{}, fmt.Errorf("cpu line not found")
}

// parseWANFromEventCache reads Firewalla event:state:cache wan_state:* blobs.
// Live values are wrapped: {state_value:0|1, labels:{ready,active,wan_intf_name}}.
// state_value 0 means ready.
func parseWANFromEventCache(cache map[string]string) map[string]snapshot.WANLink {
	out := make(map[string]snapshot.WANLink)
	for field, raw := range cache {
		if !strings.HasPrefix(field, "wan_state:") {
			continue
		}
		iface := strings.TrimPrefix(field, "wan_state:")
		if iface == "" || strings.ContainsAny(iface, ":.") {
			continue
		}
		link := snapshot.WANLink{Name: iface}
		raw = strings.TrimSpace(raw)
		var ev struct {
			StateValue *float64 `json:"state_value"`
			Labels     *struct {
				Name        string `json:"name"`
				WanIntfName string `json:"wan_intf_name"`
				Ready       bool   `json:"ready"`
				Active      bool   `json:"active"`
			} `json:"labels"`
			Name   string `json:"name"`
			Ready  bool   `json:"ready"`
			Active bool   `json:"active"`
		}
		if json.Unmarshal([]byte(raw), &ev) == nil {
			link.Ready = ev.Ready
			link.Active = ev.Active
			if ev.Name != "" {
				link.Name = ev.Name
			}
			if ev.Labels != nil {
				link.Ready = ev.Labels.Ready
				link.Active = ev.Labels.Active
				switch {
				case ev.Labels.WanIntfName != "":
					link.Name = ev.Labels.WanIntfName
				case ev.Labels.Name != "":
					link.Name = ev.Labels.Name
				}
			}
			if ev.StateValue != nil {
				link.Ready = *ev.StateValue == 0
			}
		}
		out[iface] = link
	}
	return out
}

func enrichWANFromNetworkInfo(wan map[string]snapshot.WANLink, fields map[string]string) {
	if wan == nil {
		return
	}
	for name, raw := range fields {
		raw = strings.TrimSpace(raw)
		if raw == "" || !strings.HasPrefix(raw, "{") {
			continue
		}
		var v struct {
			Desc   string `json:"desc"`
			Type   string `json:"type"`
			Ready  *bool  `json:"ready"`
			Active *bool  `json:"active"`
		}
		if json.Unmarshal([]byte(raw), &v) != nil || !strings.EqualFold(v.Type, "wan") || strings.Contains(name, ".") {
			continue
		}
		link, ok := wan[name]
		if !ok {
			link = snapshot.WANLink{Name: name}
			if v.Ready != nil {
				link.Ready = *v.Ready
			}
			if v.Active != nil {
				link.Active = *v.Active
			}
		}
		if v.Desc != "" && (link.Name == "" || link.Name == name) {
			link.Name = v.Desc
		}
		wan[name] = link
	}
}

const linuxClkTck = 100

func (c *Collector) procRoot() string {
	if c.ProcRoot != "" {
		return c.ProcRoot
	}
	return "/proc"
}

func (c *Collector) collectDNS(now time.Time) []snapshot.DNSSvc {
	root := c.procRoot()
	if _, err := os.Stat(root); err != nil {
		return nil
	}
	unboundWanted := false
	if c.Redis != nil {
		if v, err := c.Redis.HGet("sys:features", "unbound"); err == nil {
			u := strings.TrimSpace(strings.ToLower(v))
			unboundWanted = u == "1" || u == "true"
		}
	}
	return dnsSvcsFromProc(root, now, unboundWanted)
}

func (c *Collector) collectUnbound() *snapshot.Unbound {
	run := c.UnboundStats
	if run == nil {
		run = defaultUnboundStats
	}
	raw, err := run()
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil
	}
	u, ok := parseUnboundStats(raw)
	if !ok {
		return nil
	}
	return &u
}

func defaultUnboundStats() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	bin := unboundControlBin()
	var cmd *exec.Cmd
	if bin == firewallaUnboundControl {
		// Firewalla ships libhiredis + unbound.conf next to unbound-control.
		cmd = exec.CommandContext(ctx, bin, "-c", firewallaUnboundConf, "stats_noreset")
		cmd.Env = withLDLibraryPath(os.Environ(), firewallaUnboundDir)
	} else {
		cmd = exec.CommandContext(ctx, bin, "-s", "127.0.0.1@9053", "stats_noreset")
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

const (
	firewallaUnboundDir     = "/home/pi/.firewalla/run/unbound"
	firewallaUnboundControl = firewallaUnboundDir + "/unbound-control"
	firewallaUnboundConf    = firewallaUnboundDir + "/unbound.conf"
)

func unboundControlBin() string {
	if _, err := os.Stat(firewallaUnboundControl); err == nil {
		return firewallaUnboundControl
	}
	return "unbound-control"
}

func withLDLibraryPath(env []string, dir string) []string {
	const key = "LD_LIBRARY_PATH="
	out := make([]string, 0, len(env)+1)
	found := false
	for _, e := range env {
		if strings.HasPrefix(e, key) {
			found = true
			rest := strings.TrimPrefix(e, key)
			if rest == "" {
				out = append(out, key+dir)
			} else {
				out = append(out, key+dir+string(os.PathListSeparator)+rest)
			}
			continue
		}
		out = append(out, e)
	}
	if !found {
		out = append(out, key+dir)
	}
	return out
}

func parseUnboundStats(raw string) (snapshot.Unbound, bool) {
	var u snapshot.Unbound
	for _, line := range strings.Split(raw, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			continue
		}
		switch k {
		case "total.num.queries":
			u.Queries = n
		case "total.num.cachehits":
			u.Hits = n
		case "total.num.cachemiss":
			u.Misses = n
		}
	}
	if u.Queries <= 0 {
		return snapshot.Unbound{}, false
	}
	u.HitPct = 100 * float64(u.Hits) / float64(u.Queries)
	return u, true
}

func dnsSvcsFromProc(procRoot string, now time.Time, unboundWanted bool) []snapshot.DNSSvc {
	ticks := scanDNSRoles(procRoot)
	if _, ok := ticks["dnsmasq"]; !ok && udpPortOpen(procRoot, 53) {
		ticks["dnsmasq"] = 0
	}
	var out []snapshot.DNSSvc
	if _, unboundUp := ticks["unbound"]; unboundWanted || unboundUp {
		out = append(out, dnsSvc("unbound", ticks, procRoot, now))
	}
	out = append(out, dnsSvc("dnsmasq", ticks, procRoot, now))
	out = append(out, dnsSvc("firerouter", ticks, procRoot, now))
	return out
}

func dnsSvc(name string, ticks map[string]uint64, procRoot string, now time.Time) snapshot.DNSSvc {
	t, ok := ticks[name]
	svc := snapshot.DNSSvc{Name: name, OK: ok}
	if ok && t > 0 {
		svc.Since = sinceFromTicks(procRoot, t, now)
	}
	return svc
}

func scanDNSRoles(procRoot string) map[string]uint64 {
	out := map[string]uint64{}
	ents, err := os.ReadDir(procRoot)
	if err != nil {
		return out
	}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		dir := filepath.Join(procRoot, e.Name())
		comm, err := os.ReadFile(filepath.Join(dir, "comm"))
		if err != nil {
			continue
		}
		cmd, _ := os.ReadFile(filepath.Join(dir, "cmdline"))
		role := classifyDNSProc(string(comm), string(cmd))
		if role == "" {
			continue
		}
		t, err := readStartTicks(filepath.Join(dir, "stat"))
		if err != nil {
			t = 1
		}
		if t >= out[role] {
			out[role] = t
		}
	}
	return out
}

func classifyDNSProc(comm, cmdline string) string {
	co := strings.ToLower(strings.TrimSpace(comm))
	cl := strings.ToLower(strings.ReplaceAll(cmdline, "\x00", " "))
	switch {
	case strings.Contains(co, "unbound") || strings.Contains(cl, "unbound"):
		return "unbound"
	case strings.Contains(cl, "dnsmasq.dhcp"):
		return ""
	case strings.Contains(co, "dnsmasq") || strings.Contains(cl, "dnsmasq.dns") || strings.Contains(cl, "dnsmasq"):
		return "dnsmasq"
	case strings.Contains(cl, "firerouter_dns"):
		return "firerouter"
	case strings.Contains(cl, "/firerouter/") && (strings.Contains(cl, "node") || strings.Contains(co, "node") || strings.Contains(co, "firerouter")):
		return "firerouter"
	default:
		return ""
	}
}

func readStartTicks(statPath string) (uint64, error) {
	raw, err := os.ReadFile(statPath)
	if err != nil {
		return 0, err
	}
	s := string(raw)
	i := strings.LastIndex(s, ")")
	if i < 0 {
		return 0, fmt.Errorf("stat comm")
	}
	fields := strings.Fields(s[i+1:])
	if len(fields) < 20 {
		return 0, fmt.Errorf("short stat")
	}
	return strconv.ParseUint(fields[19], 10, 64)
}

func sinceFromTicks(procRoot string, ticks uint64, now time.Time) int64 {
	raw, err := os.ReadFile(filepath.Join(procRoot, "uptime"))
	if err != nil {
		return 0
	}
	upStr, _, _ := strings.Cut(strings.TrimSpace(string(raw)), " ")
	up, err := strconv.ParseFloat(upStr, 64)
	if err != nil {
		return 0
	}
	age := up - float64(ticks)/linuxClkTck
	if age < 0 {
		age = 0
	}
	return now.Unix() - int64(age)
}

func udpPortOpen(procRoot string, port int) bool {
	for _, name := range []string{"net/udp", "net/udp6"} {
		raw, err := os.ReadFile(filepath.Join(procRoot, filepath.FromSlash(name)))
		if err != nil {
			continue
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if i == 0 {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			_, hexPort, ok := strings.Cut(fields[1], ":")
			if !ok {
				continue
			}
			p, err := strconv.ParseUint(hexPort, 16, 16)
			if err == nil && int(p) == port {
				return true
			}
		}
	}
	return false
}

// DeltaBlocked reads new bytes from the ACL log and counts blocked lines.
func (t *ACLTracker) DeltaBlocked() (int64, error) {
	fi, err := os.Stat(t.Path)
	if err != nil {
		return 0, err
	}
	size := fi.Size()
	if size < t.offset {
		// Log rotated.
		t.offset = 0
	}
	if size == t.offset {
		return 0, nil
	}
	f, err := os.Open(t.Path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if _, err := f.Seek(t.offset, 0); err != nil {
		return 0, err
	}
	var n int64
	sc := bufio.NewScanner(f)
	// Raise token size for long log lines.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		if strings.Contains(sc.Text(), "[ACL][Blocked]") {
			n++
		}
	}
	if err := sc.Err(); err != nil {
		return n, err
	}
	t.offset = size
	return n, nil
}
