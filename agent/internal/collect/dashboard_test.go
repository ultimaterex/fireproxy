package collect

import (
	"strconv"
	"testing"
	"time"

	"fireproxy/pkg/inventory"
)

func TestCollectDashboardFromRedisRollups(t *testing.T) {
	now := time.Unix(1_700_000_400, 0)
	hourSlot := (now.Unix() / 3600) * 3600
	hourBucket := (hourSlot / (7 * 86400)) * (7 * 86400)
	daySlot := (now.Unix() / 86400) * 86400
	dayBucket := (daySlot / (52 * 7 * 86400)) * (52 * 7 * 86400)
	wanUUID := "wan-isp-a"
	put := func(metric, gran string, bucket, slot, val int64) (string, string, string) {
		key := "timedTraffic:" + metric + ":" + gran + ":" + strconv.FormatInt(bucket, 10)
		return key, strconv.FormatInt(slot, 10), strconv.FormatInt(val, 10)
	}

	r := &memRedis{
		hgetall: map[string]map[string]string{
			"sys:network:info": {
				"eth1": `{"name":"eth1","desc":"ISP A","type":"wan","uuid":"wan-isp-a","ip_address":"203.0.113.1"}`,
				"wg0":  `{"name":"wg0","desc":"VPN","type":"wan","uuid":"wan-vpn"}`,
				"eth3": `{"name":"eth3","desc":"","type":"wan","uuid":"wan-empty"}`,
			},
			"host:mac:AA:BB:CC:DD:EE:01": {
				"mac":    "AA:BB:CC:DD:EE:01",
				"name":   "aurora",
				"detect": `{"type":"nas"}`,
			},
		},
		zcard: map[string]int64{"alarm_active": 572},
		gets: map[string]string{
			"lastsyssumflow:upload":   "syssumflow:upload:1:2",
			"lastsyssumflow:download": "syssumflow:download:1:2",
		},
		zrev: map[string][]ZMember{
			"syssumflow:upload:1:2": {
				{Member: `{"device":"AA:BB:CC:DD:EE:01","destIP":"1.2.3.4","country":"NL"}`, Score: 1000},
				{Member: `_`, Score: 0},
			},
			"syssumflow:download:1:2": {
				{Member: `{"device":"AA:BB:CC:DD:EE:01","destIP":"9.9.9.9","domain":"hf.co"}`, Score: 500},
			},
		},
	}
	k, f, v := put("download", "1hour", hourBucket, hourSlot, 200)
	setHashField(r, k, f, v)
	k, f, v = put("upload", "1hour", hourBucket, hourSlot, 300)
	setHashField(r, k, f, v)
	k, f, v = put("conn", "1hour", hourBucket, hourSlot, 42)
	setHashField(r, k, f, v)
	k, f, v = put("ipB", "1hour", hourBucket, hourSlot, 10)
	setHashField(r, k, f, v)
	k, f, v = put("dnsB", "1hour", hourBucket, hourSlot, 5)
	setHashField(r, k, f, v)
	k, f, v = put("download:wan:"+wanUUID, "1day", dayBucket, daySlot, 1_000_000)
	setHashField(r, k, f, v)
	k, f, v = put("upload:wan:"+wanUUID, "1day", dayBucket, daySlot, 2_000_000)
	setHashField(r, k, f, v)
	k, f, v = put("upload:AA:BB:CC:DD:EE:01", "1hour", hourBucket, hourSlot, 484)
	setHashField(r, k, f, v)
	k, f, v = put("download:AA:BB:CC:DD:EE:01", "1hour", hourBucket, hourSlot, 293)
	setHashField(r, k, f, v)

	c := &Collector{Hostname: "Firewalla", Redis: r}
	cat, err := c.CollectCatalog(now)
	if err != nil {
		t.Fatal(err)
	}
	d := cat.Dashboard
	if d == nil {
		t.Fatal("dashboard missing")
	}
	if d.AlarmCount != 572 {
		t.Fatalf("alarms %d", d.AlarmCount)
	}
	if d.Transfer24h.Download != 200 || d.Transfer24h.Upload != 300 {
		t.Fatalf("transfer %+v", d.Transfer24h)
	}
	var gotConn int64
	for _, p := range d.Transfer24h.Points {
		gotConn += p.Conn
	}
	if gotConn != 42 {
		t.Fatalf("conn %d", gotConn)
	}
	if d.Blocked.Blocked != 15 || d.Blocked.Allowed != 42 {
		t.Fatalf("blocked %+v", d.Blocked)
	}
	if len(d.MonthlyWANs) != 1 || d.MonthlyWANs[0].Name != "ISP A" || d.MonthlyWANs[0].Download != 1_000_000 {
		t.Fatalf("monthly %+v", d.MonthlyWANs)
	}
	if n := len(d.Transfer24h.Points); n < 20 || n > 25 {
		t.Fatalf("want ~24 hourly points, got %d", n)
	}
	if len(d.TopUpload) != 1 || d.TopUpload[0].Name != "aurora" || d.TopUpload[0].Upload != 1000 || d.TopUpload[0].Type != "nas" {
		t.Fatalf("top up %+v", d.TopUpload)
	}
	if len(d.TopDestDownload) != 1 || d.TopDestDownload[0].Name != "hf.co" || d.TopDestDownload[0].DestIP != "9.9.9.9" {
		t.Fatalf("top dest %+v", d.TopDestDownload)
	}
	if d.TopDestDownload[0].Country != "" {
		t.Fatalf("agent must not intel-lookup dest country: %+v", d.TopDestDownload)
	}
	if len(d.DestFlows) < 2 {
		t.Fatalf("dest flows %+v", d.DestFlows)
	}
	if len(d.TopRegions) == 0 || d.TopRegions[0].Name != "Netherlands" {
		t.Fatalf("regions %+v", d.TopRegions)
	}
	nl := d.TopRegions[0]
	if len(nl.Devices) != 1 || nl.Devices[0].ID != "AA:BB:CC:DD:EE:01" || nl.Devices[0].Upload != 1000 {
		t.Fatalf("region devices %+v", nl.Devices)
	}
	if len(nl.Targets) != 1 || nl.Targets[0].Name != "1.2.3.4" || nl.Targets[0].Upload != 1000 {
		t.Fatalf("region targets %+v", nl.Targets)
	}
	var aurora *inventory.Device
	for i := range cat.Devices {
		if cat.Devices[i].MAC == "AA:BB:CC:DD:EE:01" {
			aurora = &cat.Devices[i]
			break
		}
	}
	if aurora == nil || aurora.Upload != 484 || aurora.Download != 293 {
		t.Fatalf("device traffic %+v", aurora)
	}
	if len(aurora.TopDests) < 2 {
		t.Fatalf("device top dests %+v", aurora.TopDests)
	}
	var destWithDev *inventory.RankedFlow
	for i := range d.DestFlows {
		if d.DestFlows[i].Name == "1.2.3.4" || d.DestFlows[i].ID == "1.2.3.4" {
			destWithDev = &d.DestFlows[i]
			break
		}
	}
	if destWithDev == nil || len(destWithDev.Devices) != 1 || destWithDev.Devices[0].ID != "AA:BB:CC:DD:EE:01" {
		t.Fatalf("dest devices %+v", destWithDev)
	}
}

func TestCollectDashboardSpeedtestAndDNS(t *testing.T) {
	now := time.Unix(1_700_000_400, 0)
	hourSlot := (now.Unix() / 3600) * 3600
	hourBucket := (hourSlot / (7 * 86400)) * (7 * 86400)
	r := &memRedis{
		hgetall: map[string]map[string]string{
			"sys:network:info": {
				"eth1": `{"name":"eth1","desc":"Telesur","type":"wan","uuid":"wan-a","ip_address":"203.0.113.1"}`,
				"eth2": `{"name":"eth2","desc":"Digicel","type":"wan","uuid":"wan-b","ip_address":"203.0.113.2"}`,
			},
			"policy:system": {
				"stats:wifi_speedtest": `{"dlStatus":"1002.41","ulStatus":"644.43","pingStatus":"11.06","jitterStatus":"4.19"}`,
				"app":                  `{"bandwidth":{"upload":1000,"download":1000,"wanConfs":{"wan-a":{"upload":1000,"download":1000},"wan-b":{"upload":6,"download":30}}}}`,
			},
			"event:state:cache": {
				"wan_state:eth1": `{"labels":{"ready":true,"active":true,"wan_intf_name":"Telesur","wan_intf_uuid":"wan-a"}}`,
				"wan_state:eth2": `{"labels":{"ready":true,"active":false,"wan_intf_name":"Digicel","wan_intf_uuid":"wan-b"}}`,
				"dns:1.1.1.1":    `{"state_value":0,"labels":{"name_server":"1.1.1.1","wan_intf_name":"Telesur"}}`,
				"dns:8.8.8.8":    `{"state_value":0,"labels":{"name_server":"8.8.8.8","wan_intf_name":"Telesur"}}`,
			},
		},
		zrev: map[string][]ZMember{
			"internet_speedtest_results": {
				{Member: `{"uuid":"wan-a","success":true,"timestamp":1700000000,"server":{"id":"38427","sponsor":"Telesur","location":"Balona"},"result":{"download":619,"upload":410,"latency":11}}`, Score: 1700000000},
				{Member: `{"uuid":"wan-b","success":true,"timestamp":1700000000,"result":{"download":28,"upload":6,"latency":24}}`, Score: 1700000000},
				{Member: `{"uuid":"wan-a","success":true,"timestamp":1699913600,"server":{"id":"4175","sponsor":"Digicel","location":"Paramaribo"},"result":{"download":608,"upload":401,"latency":12}}`, Score: 1699913600},
				{Member: `{"uuid":"wan-a","success":false,"timestamp":1699827200,"result":{"download":1,"upload":1,"latency":99}}`, Score: 1699827200},
			},
		},
	}
	k := "timedTraffic:dns:1hour:" + strconv.FormatInt(hourBucket, 10)
	setHashField(r, k, strconv.FormatInt(hourSlot, 10), "42")

	c := &Collector{Hostname: "Firewalla", Redis: r}
	cat, err := c.CollectCatalog(now)
	if err != nil {
		t.Fatal(err)
	}
	d := cat.Dashboard
	if d == nil {
		t.Fatal("dashboard missing")
	}
	if len(d.Speedtest) != 2 {
		t.Fatalf("speedtest %+v", d.Speedtest)
	}
	a, b := d.Speedtest[0], d.Speedtest[1]
	if a.Name != "Telesur" || !a.Active || a.PlanDown != 1000 || a.Down != 619 || a.Up != 410 {
		t.Fatalf("wan a %+v", a)
	}
	if a.ServerID != "38427" || a.Server != "Telesur" || a.Location != "Balona" {
		t.Fatalf("wan a server %+v", a)
	}
	if len(a.Points) < 2 {
		t.Fatalf("wan a points %+v", a.Points)
	}
	if a.Points[len(a.Points)-1].ServerID != "38427" {
		t.Fatalf("latest point server %+v", a.Points[len(a.Points)-1])
	}
	if b.Name != "Digicel" || b.Active || b.PlanDown != 30 || b.Down != 28 || b.Up != 6 {
		t.Fatalf("wan b %+v", b)
	}
	if d.DNS == nil {
		t.Fatal("dns missing")
	}
	if len(d.DNS.Resolvers) != 0 {
		t.Fatalf("resolvers should be omitted %+v", d.DNS.Resolvers)
	}
	var q int64
	for _, p := range d.DNS.Queries {
		q += p.Count
	}
	if q != 42 {
		t.Fatalf("dns queries %d %+v", q, d.DNS.Queries)
	}
}

func TestCollectCatalogRedisBudget(t *testing.T) {
	now := time.Unix(1_700_000_400, 0)
	r := speedtestRedis(now)
	c := &Collector{Hostname: "Firewalla", Redis: r}
	if _, err := c.CollectCatalog(now); err != nil {
		t.Fatal(err)
	}
	if n := r.hgetallN["event:state:cache"]; n != 1 {
		t.Fatalf("event:state:cache HGetAll %d want 1", n)
	}
	if n := r.hgetallN["sys:network:info"]; n != 1 {
		t.Fatalf("sys:network:info HGetAll %d want 1", n)
	}
	if n := r.hgetallN["policy:system"]; n != 0 {
		t.Fatalf("policy:system HGetAll %d want 0", n)
	}
	if n := r.hgetN["policy:system app"]; n != 1 {
		t.Fatalf("policy:system app HGet %d want 1", n)
	}
	if n := r.hgetN["policy:system stats:wifi_speedtest"]; n != 0 {
		t.Fatalf("wifi_speedtest HGet %d want 0 (history present)", n)
	}
}

func TestCollectSpeedtestWifiFallbackWhenNoHistory(t *testing.T) {
	now := time.Unix(1_700_000_400, 0)
	r := speedtestRedis(now)
	r.zrev = nil
	c := &Collector{Hostname: "Firewalla", Redis: r}
	cat, err := c.CollectCatalog(now)
	if err != nil {
		t.Fatal(err)
	}
	if n := r.hgetN["policy:system stats:wifi_speedtest"]; n != 1 {
		t.Fatalf("wifi_speedtest HGet %d want 1", n)
	}
	var a *inventory.SpeedtestWAN
	for i := range cat.Dashboard.Speedtest {
		if cat.Dashboard.Speedtest[i].Name == "Telesur" {
			a = &cat.Dashboard.Speedtest[i]
		}
	}
	if a == nil || a.Down != 1002.41 || a.Up != 644.43 {
		t.Fatalf("wifi fallback %+v", a)
	}
}

func speedtestRedis(now time.Time) *memRedis {
	hourSlot := (now.Unix() / 3600) * 3600
	hourBucket := (hourSlot / (7 * 86400)) * (7 * 86400)
	r := &memRedis{
		hgetall: map[string]map[string]string{
			"sys:network:info": {
				"eth1": `{"name":"eth1","desc":"Telesur","type":"wan","uuid":"wan-a","ip_address":"203.0.113.1"}`,
				"eth2": `{"name":"eth2","desc":"Digicel","type":"wan","uuid":"wan-b","ip_address":"203.0.113.2"}`,
			},
			"policy:system": {
				"stats:wifi_speedtest": `{"dlStatus":"1002.41","ulStatus":"644.43","pingStatus":"11.06","jitterStatus":"4.19"}`,
				"app":                  `{"bandwidth":{"upload":1000,"download":1000,"wanConfs":{"wan-a":{"upload":1000,"download":1000},"wan-b":{"upload":6,"download":30}}}}`,
			},
			"event:state:cache": {
				"wan_state:eth1": `{"labels":{"ready":true,"active":true,"wan_intf_name":"Telesur","wan_intf_uuid":"wan-a"}}`,
				"wan_state:eth2": `{"labels":{"ready":true,"active":false,"wan_intf_name":"Digicel","wan_intf_uuid":"wan-b"}}`,
			},
		},
		zrev: map[string][]ZMember{
			"internet_speedtest_results": {
				{Member: `{"uuid":"wan-a","success":true,"timestamp":1700000000,"server":{"id":"38427","sponsor":"Telesur","location":"Balona"},"result":{"download":619,"upload":410,"latency":11}}`, Score: 1700000000},
				{Member: `{"uuid":"wan-b","success":true,"timestamp":1700000000,"result":{"download":28,"upload":6,"latency":24}}`, Score: 1700000000},
				{Member: `{"uuid":"wan-a","success":true,"timestamp":1699913600,"server":{"id":"4175","sponsor":"Digicel","location":"Paramaribo"},"result":{"download":608,"upload":401,"latency":12}}`, Score: 1699913600},
			},
		},
	}
	k := "timedTraffic:dns:1hour:" + strconv.FormatInt(hourBucket, 10)
	setHashField(r, k, strconv.FormatInt(hourSlot, 10), "42")
	return r
}

func TestIsISPWan(t *testing.T) {
	cases := []struct {
		n    inventory.NetworkIface
		want bool
	}{
		{inventory.NetworkIface{Type: "wan", UUID: "1", Name: "eth1", Desc: "ISP A"}, true},
		{inventory.NetworkIface{Type: "wan", UUID: "2", Name: "eth2", Desc: "ISP B"}, true},
		{inventory.NetworkIface{Type: "wan", UUID: "3", Name: "wg0", Desc: "VPN"}, false},
		{inventory.NetworkIface{Type: "wan", UUID: "4", Name: "eth3", Desc: ""}, false},
		{inventory.NetworkIface{Type: "wan", UUID: "5", Name: "eth4", Desc: "eth4"}, false},
		{inventory.NetworkIface{Type: "lan", UUID: "6", Name: "br0", Desc: "Core"}, false},
	}
	for _, c := range cases {
		if got := isISPWan(c.n); got != c.want {
			t.Fatalf("%+v got %v", c.n, got)
		}
	}
}

func setHashField(r *memRedis, key, field, val string) {
	if r.hgetall[key] == nil {
		r.hgetall[key] = map[string]string{}
	}
	r.hgetall[key][field] = val
}
