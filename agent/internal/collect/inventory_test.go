package collect

import (
	"strings"
	"testing"
	"time"

	"fireproxy/pkg/inventory"
)

func (m *memRedis) Get(key string) (string, error) {
	if m.gets != nil {
		if v, ok := m.gets[key]; ok {
			return v, nil
		}
	}
	if m.hgetall == nil {
		return "", nil
	}
	if v, ok := m.hgetall[key][""]; ok {
		return v, nil
	}
	return "", nil
}

func (m *memRedis) HMGet(key string, fields ...string) ([]string, error) {
	out := make([]string, len(fields))
	if m.hgetall == nil {
		return out, nil
	}
	bag := m.hgetall[key]
	for i, f := range fields {
		if bag == nil {
			continue
		}
		out[i] = bag[f]
	}
	return out, nil
}

func (m *memRedis) ZCard(key string) (int64, error) {
	if m.zcard == nil {
		return 0, nil
	}
	return m.zcard[key], nil
}

func (m *memRedis) ZRevRangeByScore(key, max, min string, offset, count int64) ([]ZMember, error) {
	_ = max
	_ = min
	_ = offset
	if m.zrev == nil {
		return nil, nil
	}
	all := m.zrev[key]
	if count > 0 && int64(len(all)) > count {
		return all[:count], nil
	}
	return all, nil
}

func (m *memRedis) Scan(pattern string, count int64) ([]string, error) {
	_ = count
	var out []string
	for k := range m.hgetall {
		if matchHostMac(pattern, k) {
			out = append(out, k)
		}
	}
	return out, nil
}

func matchHostMac(pattern, key string) bool {
	switch pattern {
	case "host:mac:*":
		return strings.HasPrefix(key, "host:mac:")
	case "policy:*":
		return strings.HasPrefix(key, "policy:")
	case "tag:uid:*":
		return strings.HasPrefix(key, "tag:uid:")
	case "userTag:uid:*":
		return strings.HasPrefix(key, "userTag:uid:")
	case "deviceTag:uid:*":
		return strings.HasPrefix(key, "deviceTag:uid:")
	case "ssidTag:uid:*":
		return strings.HasPrefix(key, "ssidTag:uid:")
	default:
		return pattern == key
	}
}

func TestCollectInventory(t *testing.T) {
	r := &memRedis{
		hgetall: map[string]map[string]string{
			"host:mac:AA:00:00:00:00:99": {
				"mac":                 "AA:00:00:00:00:99",
				"name":                "Cam 1",
				"ipv4Addr":            "192.168.1.228",
				"ipv6Addr":            `["fe80::1","2001:db8::99"]`,
				"macVendor":           "TP-Link",
				"localDomain":         "backyard.t1",
				"lastActiveTimestamp": "1786381391.352",
				"intf":                "2302ded1-3084-4126-91b2-5f08302779fb",
				"detect":              `{"type":"camera","brand":"TP-Link","feedback":{"type":"iot"}}`,
			},
			"policy:mac:AA:00:00:00:00:99": {
				"tags":       `["12","57"]`,
				"deviceTags": `["29"]`,
				"dap":        `{"state":"optimizing","learnedCount":6,"allowedDomains":["a.example","b.example"],"allowedIPs":["1.2.3.4"],"domainsBlockedByRules":["bad.example"],"ipsBlockedByRules":[]}`,
			},
			"deviceTag:uid:29": {
				"uid":  "29",
				"name": "Energy Monitors",
			},
			"tag:uid:12": {
				"uid":  "12",
				"name": "IOT Cameras",
			},
			"userTag:uid:3": {
				"uid":           "3",
				"name":          "User A",
				"affiliatedTag": "2",
			},
			"tag:uid:2": {
				"uid":  "2",
				"name": "cf2a3118-3736-4338-ab7e-fc888b0e90db",
			},
			"host:mac:AA:BB:CC:DD:EE:FF": {
				"mac":   "AA:BB:CC:DD:EE:FF",
				"bname": "OnlyBname",
				"ipv4":  "10.0.0.2",
			},
			"sys:network:info": {
				"br2":  `{"name":"br2","desc":"RLAN","type":"lan","uuid":"2302ded1-3084-4126-91b2-5f08302779fb","ip_address":"192.168.1.1","subnet":"192.168.1.1/24"}`,
				"eth0": `{"name":"eth0","uuid":"a14a4a80-03b4-4bac-9b84-73958b347222"}`,
				"ddns": `"example.d.firewalla.org"`,
			},
			"event:state:cache": {"x": "y"},
		},
		zrev: map[string][]ZMember{
			"host:active:mac": {
				{Member: "aa:00:00:00:00:99", Score: 100},
			},
		},
	}
	c := &Collector{Hostname: "Firewalla", Redis: r}
	inv, err := c.CollectInventory(time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Devices) != 2 {
		t.Fatalf("devices: %+v", inv.Devices)
	}
	cat, err := c.CollectCatalog(time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Network) != 1 || cat.Network[0].Desc != "RLAN" {
		t.Fatalf("network: %+v", cat.Network)
	}
	var backyard *inventory.Device
	for i := range cat.Devices {
		if cat.Devices[i].MAC == "AA:00:00:00:00:99" {
			backyard = &cat.Devices[i]
			break
		}
	}
	if backyard == nil {
		t.Fatal("missing backyard device")
	}
	if backyard.Active == nil || !*backyard.Active {
		t.Fatalf("backyard should be active: %+v", backyard.Active)
	}
	var inactive *inventory.Device
	for i := range cat.Devices {
		if cat.Devices[i].MAC == "AA:BB:CC:DD:EE:FF" {
			inactive = &cat.Devices[i]
			break
		}
	}
	if inactive == nil || inactive.Active == nil || *inactive.Active {
		t.Fatalf("OnlyBname should be inactive: %+v", inactive)
	}
	if backyard.Type != "iot" {
		t.Fatalf("detect type (feedback wins): %q", backyard.Type)
	}
	if len(backyard.IPv6) != 2 || backyard.IPv6[1] != "2001:db8::99" {
		t.Fatalf("ipv6: %v", backyard.IPv6)
	}
	if len(backyard.TagIDs) < 2 {
		t.Fatalf("expected group tags from policy:mac, got %v", backyard.TagIDs)
	}
	if len(backyard.DeviceTagIDs) != 1 || backyard.DeviceTagIDs[0] != "29" {
		t.Fatalf("device tags: %v", backyard.DeviceTagIDs)
	}
	foundDeviceTag := false
	var userTag, afGroup *inventory.Tag
	for i := range cat.Tags {
		tagn := &cat.Tags[i]
		if tagn.ID == "29" && tagn.Type == "device" && tagn.Name == "Energy Monitors" {
			foundDeviceTag = true
		}
		if tagn.ID == "3" && tagn.Type == "user" {
			userTag = tagn
		}
		if tagn.ID == "2" && tagn.Type == "group" {
			afGroup = tagn
		}
	}
	if !foundDeviceTag {
		t.Fatalf("expected deviceTag catalog entry, got %+v", cat.Tags)
	}
	if userTag == nil || userTag.Name != "User A" || userTag.AffiliatedTag != "2" {
		t.Fatalf("user affiliatedTag: %+v", userTag)
	}
	if afGroup == nil || !strings.Contains(afGroup.Name, "-") {
		t.Fatalf("affiliated group: %+v", afGroup)
	}
	if backyard.DAP == nil {
		t.Fatal("expected DAP from policy:mac")
	}
	if backyard.DAP.Status != "optimizing" || backyard.DAP.Learned != 6 || backyard.DAP.Trusted != 3 || backyard.DAP.Blocked != 1 {
		t.Fatalf("dap: %+v", backyard.DAP)
	}
}

func TestParseDAP(t *testing.T) {
	na := parseDAP(`{"lastUpdated":1785850403,"learnedCount":0,"localLearnedCount":0,"reason":"This device type is not supported: desktop","state":"not_applicable"}`)
	if na == nil || na.Status != "not_applicable" || na.Learned != 0 || na.Reason == "" {
		t.Fatalf("na: %+v", na)
	}
	if parseDAP("") != nil || parseDAP("{") != nil {
		t.Fatal("invalid dap should be nil")
	}
}

func TestCollectNetworkDropsPhyLeftover(t *testing.T) {
	r := &memRedis{
		hgetall: map[string]map[string]string{
			"sys:network:info": {
				"br4":      `{"name":"br4","desc":"SWARM","type":"lan","uuid":"u4","ip_address":"192.168.50.1","subnet":"192.168.50.1/24","vid":50}`,
				"eth0.250": `{"name":"eth0.250","desc":"IOT","type":"lan","uuid":"stale","ip_address":"192.168.10.1","subnet":"192.168.10.1/24"}`,
				"eth1":     `{"name":"eth1","desc":"ISP A","type":"wan","uuid":"w1","ip_address":"203.0.113.10","subnet":"203.0.113.10/30"}`,
			},
		},
	}
	c := &Collector{Hostname: "box", Redis: r}
	nets, err := c.collectNetwork()
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 2 {
		t.Fatalf("nets: %+v", nets)
	}
	for _, n := range nets {
		if n.Desc == "IOT" || n.Name == "eth0.250" {
			t.Fatalf("phy leftover kept: %+v", n)
		}
	}
}
