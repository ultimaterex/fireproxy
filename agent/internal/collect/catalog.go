package collect

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"fireproxy/pkg/device"
	"fireproxy/pkg/inventory"
)

// CollectCatalog builds devices + network + policies + tags (infrequent).
func (c *Collector) CollectCatalog(now time.Time) (inventory.Catalog, error) {
	cat := inventory.Catalog{
		TS:       now.Unix(),
		Host:     c.Hostname,
		Devices:  []inventory.Device{},
		Network:  []inventory.NetworkIface{},
		Policies: []inventory.Policy{},
		Tags:     []inventory.Tag{},
	}
	if c.Redis == nil {
		return cat, nil
	}

	devs, err := c.collectDevices(now)
	if err != nil {
		return cat, err
	}
	cat.Devices = devs
	c.attachDeviceTraffic(now, cat.Devices)

	info, _ := c.Redis.HGetAll("sys:network:info")
	cache, _ := c.Redis.HGetAll("event:state:cache")
	cat.Network = networkFromFields(info)
	switches, topo, network := c.collectAssets(cat.Network)
	overlaySwitchNames(switches, cat.Devices)
	cat.Switches = switches
	cat.Topo = topo
	cat.Network = network
	if pols, err := c.collectPolicies(); err == nil {
		cat.Policies = pols
	}
	if tags, err := c.collectTags(); err == nil {
		cat.Tags = tags
	}
	cat.Dashboard = c.collectDashboard(now, cat.Network, cat.Devices, cache)
	cat.Box = c.collectBox(cache, info)
	cat.Wifi = c.collectWifi(cat.Network)
	return cat, nil
}

// CollectInventory keeps the older devices-only helper for tests.
func (c *Collector) CollectInventory(now time.Time) (device.Inventory, error) {
	cat, err := c.CollectCatalog(now)
	if err != nil {
		return device.Inventory{}, err
	}
	out := device.Inventory{TS: cat.TS, Host: cat.Host, Devices: make([]device.Device, 0, len(cat.Devices))}
	for _, d := range cat.Devices {
		out.Devices = append(out.Devices, d.Device)
	}
	return out, nil
}

// hostActiveSpan matches Firewalla HostManager INACTIVE_TIME_SPAN (7 days).
const hostActiveSpan = 7 * 24 * time.Hour

func (c *Collector) collectDevices(now time.Time) ([]inventory.Device, error) {
	keys, err := c.Redis.Scan("host:mac:*", 100)
	if err != nil {
		return nil, err
	}
	active := c.activeHostMACs(now)
	out := make([]inventory.Device, 0, len(keys))
	for _, key := range keys {
		h, err := c.Redis.HGetAll(key)
		if err != nil || len(h) == 0 {
			continue
		}
		base := deviceFromHash(h)
		if base.MAC == "" {
			if parts := strings.SplitN(key, "host:mac:", 2); len(parts) == 2 {
				base.MAC = strings.ToUpper(parts[1])
			}
		}
		if base.MAC == "" {
			continue
		}
		on := false
		if _, ok := active[base.MAC]; ok {
			on = true
		}
		base.Active = &on
		src := h
		var dap *inventory.DAP
		// Firewalla stores membership + DAP on policy:mac:<MAC>, not host:mac:*.
		if pol, err := c.Redis.HGetAll("policy:mac:" + base.MAC); err == nil && len(pol) > 0 {
			src = pol
			dap = parseDAP(pol["dap"])
		}
		d := inventory.Device{
			Device:       base,
			IntfUUID:     strings.TrimSpace(first(h, "intf", "intf_uuid")),
			TagIDs:       parseTagField(src, "tags"),
			DeviceTagIDs: parseTagField(src, "deviceTags"),
			UserTagIDs:   parseTagField(src, "userTags"),
			DAP:          dap,
		}
		out = append(out, d)
	}
	return out, nil
}

func (c *Collector) activeHostMACs(now time.Time) map[string]struct{} {
	out := map[string]struct{}{}
	if c.Redis == nil {
		return out
	}
	// Exclusive min matches Firewalla: ZREVRANGEBYSCORE … +inf (now-span)
	min := "(" + strconv.FormatInt(now.Add(-hostActiveSpan).Unix(), 10)
	members, err := c.Redis.ZRevRangeByScore("host:active:mac", "+inf", min, 0, 0)
	if err != nil {
		return out
	}
	for _, m := range members {
		mac := strings.ToUpper(strings.TrimSpace(m.Member))
		if mac == "" {
			continue
		}
		out[mac] = struct{}{}
	}
	return out
}

func (c *Collector) collectNetwork() ([]inventory.NetworkIface, error) {
	// sys:network:info is a Redis HASH: field = ifname, value = JSON blob
	// (plus non-interface fields like ddns / publicIp).
	fields, err := c.Redis.HGetAll("sys:network:info")
	if err != nil {
		return nil, err
	}
	return networkFromFields(fields), nil
}

func networkFromFields(fields map[string]string) []inventory.NetworkIface {
	if len(fields) == 0 {
		return nil
	}
	seenUUID := map[string]int{} // uuid -> index in out
	out := make([]inventory.NetworkIface, 0, len(fields))
	for k, raw := range fields {
		raw = strings.TrimSpace(raw)
		if raw == "" || !strings.HasPrefix(raw, "{") {
			continue
		}
		var v struct {
			Name   string `json:"name"`
			Desc   string `json:"desc"`
			Type   string `json:"type"`
			UUID   string `json:"uuid"`
			IP     string `json:"ip_address"`
			Subnet string `json:"subnet"`
			VID    int    `json:"vid"`
		}
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			continue
		}
		if v.UUID == "" {
			continue
		}
		if v.Desc == "" && v.Type == "" && v.IP == "" && v.Subnet == "" {
			// raw NIC / unused VLAN — not a network
			continue
		}
		name := v.Name
		if name == "" {
			name = k
		}
		if v.Type != "wan" && isPhyIfName(name) {
			// Leftover NIC/VLAN (e.g. eth0.250 "IOT") — real LANs are br*/bond*.
			continue
		}
		vid := v.VID
		if vid == 0 {
			vid = vidFromIntfs([]string{name})
		}
		iface := inventory.NetworkIface{
			Name:   name,
			Desc:   v.Desc,
			Type:   v.Type,
			UUID:   v.UUID,
			IP:     v.IP,
			Subnet: v.Subnet,
			VID:    vid,
		}
		if idx, ok := seenUUID[v.UUID]; ok {
			// Prefer br* over bond* over eth*.N so FireRouter lans merge by name.
			if logicalRank(out[idx].Name) >= logicalRank(name) {
				continue
			}
			out[idx] = iface
			continue
		}
		seenUUID[v.UUID] = len(out)
		out = append(out, iface)
	}
	return out
}

func logicalRank(name string) int {
	switch {
	case strings.HasPrefix(name, "br"):
		return 2
	case strings.HasPrefix(name, "bond"):
		return 1
	default:
		return 0
	}
}

func isPhyIfName(name string) bool {
	base := name
	if i := strings.IndexByte(name, '.'); i >= 0 {
		base = name[:i]
	}
	return strings.HasPrefix(base, "eth") || strings.HasPrefix(base, "wlan")
}

func (c *Collector) collectPolicies() ([]inventory.Policy, error) {
	keys, err := c.Redis.Scan("policy:*", 100)
	if err != nil {
		return nil, err
	}
	out := make([]inventory.Policy, 0, len(keys))
	for _, key := range keys {
		id := strings.TrimPrefix(key, "policy:")
		if id == "" || id == "system" || strings.Contains(id, ":") {
			continue
		}
		if _, err := strconv.Atoi(id); err != nil {
			continue
		}
		h, err := c.Redis.HGetAll(key)
		if err != nil || len(h) == 0 {
			continue
		}
		hits, _ := strconv.ParseInt(strings.TrimSpace(h["hitCount"]), 10, 64)
		disabled := h["disabled"] == "1" || strings.EqualFold(h["disabled"], "true")
		notes := strings.TrimSpace(h["_name"])
		out = append(out, inventory.Policy{
			ID:       id,
			Action:   strings.TrimSpace(h["action"]),
			Type:     strings.TrimSpace(h["type"]),
			Target:   strings.TrimSpace(h["target"]),
			HitCount: hits,
			Disabled: disabled,
			Notes:    notes,
		})
	}
	return out, nil
}

func (c *Collector) collectTags() ([]inventory.Tag, error) {
	// Firewalla keeps four tag namespaces (see net2/Constants.js TAG_TYPE_MAP).
	prefixes := []struct {
		pattern string
		prefix  string
		typ     string
	}{
		{"tag:uid:*", "tag:uid:", "group"},
		{"userTag:uid:*", "userTag:uid:", "user"},
		{"deviceTag:uid:*", "deviceTag:uid:", "device"},
		{"ssidTag:uid:*", "ssidTag:uid:", "ssid"},
	}
	out := make([]inventory.Tag, 0)
	seen := map[string]struct{}{}
	for _, p := range prefixes {
		keys, err := c.Redis.Scan(p.pattern, 50)
		if err != nil {
			continue
		}
		for _, key := range keys {
			h, err := c.Redis.HGetAll(key)
			if err != nil || len(h) == 0 {
				continue
			}
			id := strings.TrimSpace(h["uid"])
			if id == "" {
				id = strings.TrimPrefix(key, p.prefix)
			}
			if id == "" {
				continue
			}
			// IDs can collide across namespaces; key by type:id.
			dedupe := p.typ + ":" + id
			if _, ok := seen[dedupe]; ok {
				continue
			}
			seen[dedupe] = struct{}{}
			name := strings.TrimSpace(h["name"])
			if name == "" {
				name = id
			}
			out = append(out, inventory.Tag{
				ID:            id,
				Name:          name,
				Type:          p.typ,
				AffiliatedTag: strings.TrimSpace(h["affiliatedTag"]),
			})
		}
	}
	return out, nil
}

func first(h map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(h[k]); v != "" {
			return v
		}
	}
	return ""
}

func parseTagField(h map[string]string, field string) []string {
	raw := strings.TrimSpace(h[field])
	if raw == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		id = strings.TrimPrefix(id, "tag:")
		id = strings.TrimPrefix(id, "deviceTag:")
		id = strings.TrimPrefix(id, "userTag:")
		id = strings.TrimPrefix(id, "ssidTag:")
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if strings.HasPrefix(raw, "[") {
		var arr []any
		if json.Unmarshal([]byte(raw), &arr) == nil {
			for _, v := range arr {
				switch t := v.(type) {
				case string:
					add(t)
				case float64:
					add(strconv.FormatInt(int64(t), 10))
				}
			}
			return out
		}
	}
	cleaned := strings.NewReplacer("[", "", "]", "", "\"", "", "'", "").Replace(raw)
	for _, part := range strings.FieldsFunc(cleaned, func(r rune) bool {
		return r == ',' || r == ' '
	}) {
		add(part)
	}
	return out
}
