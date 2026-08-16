package collect

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"fireproxy/pkg/inventory"
)

func (c *Collector) collectBox(cache, info map[string]string) *inventory.Box {
	b := &inventory.Box{Name: c.Hostname}
	fields := info
	btMac := ""
	if c.Redis != nil {
		if eid, err := c.Redis.HGet("sys:ept", "eid"); err == nil {
			b.EID = strings.TrimSpace(eid)
		}
		if tz, err := c.Redis.HGet("sys:config", "timezone"); err == nil {
			b.Timezone = strings.TrimSpace(tz)
		}
		if mode, err := c.Redis.Get("mode"); err == nil {
			b.Mode = strings.TrimSpace(mode)
		}
		if name, err := c.Redis.Get("groupName"); err == nil {
			if n := strings.TrimSpace(unquoteJSON(name)); n != "" {
				b.Name = n
			}
		}
		if bt, err := c.Redis.Get("sys:bt:mac"); err == nil {
			btMac = strings.TrimSpace(unquoteJSON(bt))
		}
		// Firewalla Constants.REDIS_KEY_LOCAL_DOMAIN_SUFFIX.
		if suf, err := c.Redis.Get("local:domain:suffix"); err == nil {
			if s := strings.ToLower(strings.TrimSpace(unquoteJSON(suf))); s != "" {
				b.LocalDomainSuffix = s
			}
		}
		if fields == nil {
			if all, err := c.Redis.HGetAll("sys:network:info"); err == nil {
				fields = all
			}
		}
		if cache == nil {
			if all, err := c.Redis.HGetAll("event:state:cache"); err == nil {
				cache = all
			}
		}
		if fields != nil {
			b.PublicIP = unquoteJSON(fields["publicIp"])
			b.DDNS = unquoteJSON(fields["ddns"])
		}
		b.WanType = wanTypeFromEventCache(cache)
	}
	b.MACs = collectBoxMACs(c.sysfsRoot(), fields, btMac)
	kv := readReleaseFiles(c.releasePaths())
	if v := firstKV(kv, "VERSION"); v != "" {
		b.Version = v
	}
	if p := firstKV(kv, "PLATFORM", "BOARD"); p != "" {
		b.Model = p
		b.License = licenseName(p)
	}
	if b.Version == "" {
		cfg := c.ConfigJSONPath
		if cfg == "" {
			cfg = filepath.Join(c.firewallaHome(), "net2", "config.json")
		}
		b.Version = readJSONVersion(cfg)
	}
	if hash := readGitShort(c.firewallaHome()); hash != "" && b.Version != "" {
		b.Version = b.Version + " (" + hash + ")"
	}
	return b
}

func (c *Collector) firewallaHome() string {
	if c.FirewallaHome != "" {
		return c.FirewallaHome
	}
	return "/home/pi/firewalla"
}

func (c *Collector) sysfsRoot() string {
	if c.SysfsRoot != "" {
		return c.SysfsRoot
	}
	return "/sys"
}

func (c *Collector) releasePaths() []string {
	if c.ReleasePath != "" {
		return []string{c.ReleasePath}
	}
	return []string{"/etc/firewalla_release", "/etc/firewalla-release"}
}

func readReleaseFiles(paths []string) map[string]string {
	out := map[string]string{}
	for _, p := range paths {
		for k, v := range readKVFile(p) {
			if _, ok := out[k]; !ok {
				out[k] = v
			}
		}
	}
	return out
}

func firstKV(kv map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(kv[k]); v != "" {
			return v
		}
	}
	return ""
}

func collectBoxMACs(sysfs string, fields map[string]string, btMac string) []inventory.BoxMAC {
	var out []inventory.BoxMAC
	seen := map[string]struct{}{}
	add := func(name, mac string) {
		mac = strings.ToUpper(strings.TrimSpace(mac))
		if mac == "" {
			return
		}
		if _, ok := seen[mac]; ok {
			return
		}
		seen[mac] = struct{}{}
		out = append(out, inventory.BoxMAC{Name: name, MAC: mac})
	}

	// Gold series silk-screen: Port 1=eth3 … Port 4=eth0 (WAN default).
	for port := 1; port <= 4; port++ {
		eth := 4 - port
		add("Ethernet Port "+strconv.Itoa(port), readSysAddr(sysfs, "class/net/eth"+strconv.Itoa(eth)+"/address"))
	}
	if mac := readSysAddr(sysfs, "class/net/wlan0/address"); mac != "" {
		add("Wi-Fi", mac)
	}
	if mac := readBluetoothAddr(sysfs, btMac); mac != "" {
		add("Bluetooth", mac)
	}
	if len(out) > 0 {
		return out
	}

	ethN := regexp.MustCompile(`^eth(\d+)$`)
	for name, raw := range fields {
		raw = strings.TrimSpace(raw)
		if raw == "" || !strings.HasPrefix(raw, "{") {
			continue
		}
		var v struct {
			MAC    string `json:"mac_address"`
			MACAlt string `json:"mac"`
		}
		if json.Unmarshal([]byte(raw), &v) != nil {
			continue
		}
		mac := v.MAC
		if mac == "" {
			mac = v.MACAlt
		}
		label := nicLabel(name, ethN)
		if label == "" {
			continue
		}
		add(label, mac)
	}
	add("Bluetooth", btMac)
	return out
}

func nicLabel(name string, ethN *regexp.Regexp) string {
	if m := ethN.FindStringSubmatch(name); m != nil {
		n, _ := strconv.Atoi(m[1])
		if n < 0 || n > 3 {
			return ""
		}
		return "Ethernet Port " + strconv.Itoa(4-n)
	}
	if name == "wlan0" {
		return "Wi-Fi"
	}
	return ""
}

func readSysAddr(root, rel string) string {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func readBluetoothAddr(sysfs, redisMAC string) string {
	if mac := readSysAddr(sysfs, "class/bluetooth/hci0/address"); mac != "" {
		return mac
	}
	matches, err := filepath.Glob(filepath.Join(sysfs, "class", "bluetooth", "*", "address"))
	if err == nil {
		for _, p := range matches {
			raw, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			if mac := strings.TrimSpace(string(raw)); mac != "" {
				return mac
			}
		}
	}
	return strings.TrimSpace(redisMAC)
}

func unquoteJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s[0] != '"' {
		return s
	}
	var out string
	if json.Unmarshal([]byte(s), &out) == nil {
		return out
	}
	return strings.Trim(s, `"`)
}

func licenseName(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "goldse", "gse", "gold-se":
		return "Firewalla Gold SE"
	case "goldpro", "gpx", "gold-pro", "gold_pro":
		return "Firewalla Gold Pro"
	case "gold":
		return "Firewalla Gold"
	case "purple":
		return "Firewalla Purple"
	case "purple_se", "pse", "purple-se":
		return "Firewalla Purple SE"
	case "navy":
		return "Firewalla Navy"
	case "red":
		return "Firewalla Red"
	case "blue":
		return "Firewalla Blue"
	default:
		if platform == "" {
			return ""
		}
		return "Firewalla " + platform
	}
}

func readKVFile(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			if k, v, ok = strings.Cut(line, ":"); !ok {
				continue
			}
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return out
}

func wanTypeFromEventCache(cache map[string]string) string {
	for field, raw := range cache {
		var candidate string
		switch {
		case strings.HasPrefix(field, "dualwan_state:"):
			candidate = strings.TrimPrefix(field, "dualwan_state:")
		case field == "overall_wan_state" || strings.HasPrefix(field, "overall_wan_state:"):
			if i := strings.IndexByte(field, ':'); i >= 0 {
				candidate = field[i+1:]
			}
		default:
			continue
		}
		if wt := wanTypeLabel(raw); wt != "" {
			if parsed := parseWanType(wt); parsed != "" {
				return parsed
			}
		}
		if parsed := parseWanType(candidate); parsed != "" {
			return parsed
		}
	}
	return ""
}

func wanTypeLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var ev struct {
		Labels *struct {
			WanType string `json:"wanType"`
		} `json:"labels"`
	}
	if json.Unmarshal([]byte(raw), &ev) == nil && ev.Labels != nil {
		return strings.TrimSpace(ev.Labels.WanType)
	}
	return ""
}

func readGitShort(repo string) string {
	raw, err := os.ReadFile(filepath.Join(repo, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(head, "ref:") {
		return shortSHA(head)
	}
	ref := strings.TrimSpace(strings.TrimPrefix(head, "ref:"))
	if ref == "" {
		return ""
	}
	if b, err := os.ReadFile(filepath.Join(repo, ".git", filepath.FromSlash(ref))); err == nil {
		return shortSHA(string(b))
	}
	return shortSHAFromPacked(filepath.Join(repo, ".git", "packed-refs"), ref)
}

func shortSHA(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 8 {
		return ""
	}
	for _, c := range s[:8] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return ""
		}
	}
	return strings.ToLower(s[:8])
}

func shortSHAFromPacked(path, ref string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "^") {
			continue
		}
		sha, name, ok := strings.Cut(line, " ")
		if ok && strings.TrimSpace(name) == ref {
			return shortSHA(sha)
		}
	}
	return ""
}

func readJSONVersion(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var v struct {
		Version json.Number `json:"version"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return ""
	}
	return strings.TrimSpace(v.Version.String())
}
