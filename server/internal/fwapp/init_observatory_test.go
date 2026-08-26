package fwapp

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseInitObservatoryFromFixture(t *testing.T) {
	path := labInitFixturePath()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("lab fixture missing (%s): %v", path, err)
	}
	obs, err := ParseInitObservatory(raw)
	if err != nil {
		t.Fatal(err)
	}
	if obs.AlarmCount <= 0 {
		t.Fatalf("alarm_count=%d", obs.AlarmCount)
	}
	if len(obs.NewAlarms) == 0 {
		t.Fatal("expected newAlarms")
	}
	if obs.Transfer24h.Upload == 0 && obs.Transfer24h.Download == 0 {
		t.Fatalf("newLast24 totals %+v", obs.Transfer24h)
	}
	if len(obs.Transfer24h.Points) == 0 {
		t.Fatal("expected newLast24 points")
	}
	if len(obs.MonthlyWANs) == 0 {
		t.Fatal("expected monthlyDataUsageOnWans")
	}
	if obs.Box == nil || obs.Box.Model == "" {
		t.Fatalf("box %+v", obs.Box)
	}
	if len(obs.Devices) == 0 {
		t.Fatal("expected hosts")
	}
	if obs.SysMetrics == nil || obs.SysMetrics.Load1 == 0 {
		t.Fatalf("sysMetrics %+v", obs.SysMetrics)
	}
	if len(obs.NICStates) == 0 {
		t.Fatal("expected nicStates")
	}
	if len(obs.Speedtest) == 0 {
		t.Fatal("expected internetSpeedtestResults grouped by WAN")
	}
	for _, w := range obs.Speedtest {
		if w.UUID == "" {
			t.Fatalf("speedtest missing uuid: %+v", w)
		}
		if len(w.Points) == 0 {
			t.Fatalf("speedtest %s missing points", w.UUID)
		}
	}
	if obs.RuleCount <= 0 {
		t.Fatalf("rule_count=%d", obs.RuleCount)
	}
	if obs.Blocked.Blocked <= 0 || obs.Blocked.Allowed <= 0 {
		t.Fatalf("blocked mix %+v", obs.Blocked)
	}
	if obs.DNS == nil || len(obs.DNS.Queries) == 0 {
		t.Fatalf("dns %+v", obs.DNS)
	}
	if len(obs.DNS.Resolvers) == 0 {
		t.Fatal("expected dns resolvers from latestAllStateEvents")
	}
	if obs.SysMetrics == nil || obs.SysMetrics.CPU == nil {
		t.Fatalf("expected cpu from sysMetrics: %+v", obs.SysMetrics)
	}
	if len(obs.SysMetrics.Disks) == 0 {
		t.Fatal("expected diskInfo")
	}
	if len(obs.WAN) == 0 {
		t.Fatal("expected wan_state map")
	}
	if obs.Box == nil || obs.Box.UptimeSec == nil || obs.Box.CloudConnected == nil {
		t.Fatalf("expected box uptime/cloud: %+v", obs.Box)
	}
	if len(obs.Tags) == 0 {
		t.Fatal("expected tags")
	}
	if obs.Transfer30d.Upload == 0 && obs.Transfer30d.Download == 0 {
		t.Fatalf("expected last30 totals %+v", obs.Transfer30d)
	}
	if obs.MonthlyBeginTS == 0 || obs.MonthlyEndTS == 0 {
		t.Fatalf("monthly cycle begin=%d end=%d", obs.MonthlyBeginTS, obs.MonthlyEndTS)
	}
	if len(obs.NICMetrics) == 0 {
		t.Fatal("expected networkMetrics")
	}
	if len(obs.WGPeers) == 0 && len(obs.WGClients) == 0 {
		t.Fatal("expected wg peers or clients")
	}
	if len(obs.ClientProfiles) == 0 {
		t.Fatal("expected client_profiles catalog")
	}
	var wgFamily *InitVPNClientFamily
	emptyFamily := false
	for i := range obs.ClientProfiles {
		f := &obs.ClientProfiles[i]
		if f.Family == "wireguard" {
			wgFamily = f
		}
		if f.Family != "wireguard" && len(f.Profiles) == 0 {
			emptyFamily = true
		}
	}
	if wgFamily == nil || len(wgFamily.Profiles) < 4 {
		t.Fatalf("wireguard client profiles: %+v", obs.ClientProfiles)
	}
	if wgFamily.Profiles[0].ProfileID == "" || wgFamily.Profiles[0].DisplayName == "" {
		t.Fatalf("profile fields: %+v", wgFamily.Profiles[0])
	}
	if wgFamily.Profiles[0].Status != "disconnected" && wgFamily.Profiles[0].Status != "connected" {
		t.Fatalf("expected normalized status, got %q", wgFamily.Profiles[0].Status)
	}
	if !emptyFamily {
		t.Fatal("expected empty families in catalog")
	}
	if len(obs.WGClients) < 4 || obs.WGClients[0].Status == "false" || obs.WGClients[0].Status == "true" {
		t.Fatalf("legacy wg_clients should reuse normalized status: %+v", obs.WGClients)
	}
	var sawDigicel, sawTelesur bool
	for _, w := range obs.MonthlyWANs {
		if w.Name == "" || w.Name == w.UUID {
			t.Fatalf("monthly WAN needs display name: %+v", w)
		}
		if strings.Contains(strings.ToLower(w.Name), "digicel") {
			sawDigicel = true
		}
		if strings.Contains(strings.ToLower(w.Name), "telesur") {
			sawTelesur = true
		}
		if strings.HasPrefix(w.Name, "WAN (") {
			t.Fatalf("monthly WAN still iface-fallback: %+v", w)
		}
	}
	if !sawDigicel || !sawTelesur {
		t.Fatalf("expected Digicel + Telesur monthly names: %+v", obs.MonthlyWANs)
	}
	// Ranked flows stay empty — systemFlows.flows is empty in lab; do not invent.
	if len(obs.TopUpload) != 0 || len(obs.TopDownload) != 0 ||
		len(obs.TopDestUpload) != 0 || len(obs.TopDestDownload) != 0 ||
		len(obs.TopRegions) != 0 || len(obs.DestFlows) != 0 {
		t.Fatalf("ranked flows must stay empty: up=%d down=%d destUp=%d destDown=%d regions=%d dest=%d",
			len(obs.TopUpload), len(obs.TopDownload), len(obs.TopDestUpload),
			len(obs.TopDestDownload), len(obs.TopRegions), len(obs.DestFlows))
	}
}

func TestParseInitObservatoryEnvelope(t *testing.T) {
	raw := []byte(`{"mtype":"init","data":{
		"activeAlarmCount":2,
		"pendingAlarmCount":1,
		"archivedAlarmCount":3,
		"policyRuleNumber":7,
		"newAlarms":[{"aid":9,"type":"ALARM_INTEL","message":"hi","timestamp":1}],
		"newLast24":{
			"totalUpload":10,"totalDownload":20,"totalConn":100,"totalDnsB":3,"totalIpB":4,
			"upload":[[100,1]],"download":[[100,2]],"conn":[[100,3]],
			"dns":[[100,50],[200,60]]
		},
		"monthlyDataUsageOnWans":{"u1":{"totalUpload":4,"totalDownload":5}},
		"networkProfiles":{"u1":{"uuid":"u1","type":"wan","intf":"eth2"}},
		"networkConfig":{"interface":{"phy":{"eth2":{"meta":{"name":"Digicel","type":"wan","uuid":"u1"}}}}},
		"internetSpeedtestResults":[],
		"model":"gold",
		"device":"Box",
		"hosts":[{"mac":"aa:bb:cc:dd:ee:01","name":"Phone","ip":"10.0.0.2","macVendor":"Acme","tags":["1"],"flowsummary":{"inbytes":8,"outbytes":9}}],
		"sysMetrics":{"load1":1.5,"load5":1.1,"load15":0.9,"memUsage":0.4,"totalMem":1000},
		"nicStates":{"eth0":{"address":"aa:bb:cc:dd:ee:ff","carrier":"1","duplex":"full","speed":"1000"}},
		"systemFlows":{"upload":{"flows":[]},"download":{"flows":[]}}
	}}`)
	obs, err := ParseInitObservatory(raw)
	if err != nil {
		t.Fatal(err)
	}
	if obs.AlarmCount != 2 || obs.PendingAlarmCount != 1 || obs.ArchivedAlarmCount != 3 {
		t.Fatalf("counts %+v", obs)
	}
	if obs.RuleCount != 7 {
		t.Fatalf("rules=%d", obs.RuleCount)
	}
	if len(obs.NewAlarms) != 1 || obs.NewAlarms[0].AID != 9 {
		t.Fatalf("alarms %+v", obs.NewAlarms)
	}
	if obs.Transfer24h.Upload != 10 || obs.Transfer24h.Download != 20 || len(obs.Transfer24h.Points) != 1 {
		t.Fatalf("transfer %+v", obs.Transfer24h)
	}
	if obs.Transfer24h.Points[0].TS != 100 || obs.Transfer24h.Points[0].Upload != 1 ||
		obs.Transfer24h.Points[0].Download != 2 || obs.Transfer24h.Points[0].Conn != 3 {
		t.Fatalf("point %+v", obs.Transfer24h.Points[0])
	}
	if obs.Blocked.Blocked != 7 || obs.Blocked.Allowed != 100 {
		t.Fatalf("blocked %+v", obs.Blocked)
	}
	if obs.DNS == nil || len(obs.DNS.Queries) != 2 || obs.DNS.Queries[0].Count != 50 {
		t.Fatalf("dns %+v", obs.DNS)
	}
	if len(obs.MonthlyWANs) != 1 || obs.MonthlyWANs[0].UUID != "u1" || obs.MonthlyWANs[0].Name != "Digicel" {
		t.Fatalf("monthly %+v", obs.MonthlyWANs)
	}
	if obs.Box == nil || obs.Box.Model != "gold" || obs.Box.Name != "Box" {
		t.Fatalf("box %+v", obs.Box)
	}
	if len(obs.Devices) != 1 {
		t.Fatalf("devices %+v", obs.Devices)
	}
	d := obs.Devices[0]
	if d.MAC != "AA:BB:CC:DD:EE:01" || d.Name != "Phone" || d.IP != "10.0.0.2" ||
		d.Vendor != "Acme" || d.Download != 8 || d.Upload != 9 || len(d.TagIDs) != 1 || d.TagIDs[0] != "1" {
		t.Fatalf("device %+v", d)
	}
	if obs.SysMetrics == nil || obs.SysMetrics.Load1 != 1.5 {
		t.Fatalf("sys %+v", obs.SysMetrics)
	}
	if len(obs.NICStates) != 1 || obs.NICStates[0].Name != "eth0" {
		t.Fatalf("nics %+v", obs.NICStates)
	}
}

func TestParseInitSpeedtestSkipsMissingTimestamp(t *testing.T) {
	before := time.Now().Unix()
	raw := []byte(`{
		"internetSpeedtestResults":[
			{"uuid":"wan-a","success":true,"result":{"download":100,"upload":50,"latency":5}},
			{"uuid":"wan-a","success":true,"timestamp":1700000000,"result":{"download":200,"upload":80,"latency":6}}
		],
		"model":"gold"
	}`)
	obs, err := ParseInitObservatory(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs.Speedtest) != 1 {
		t.Fatalf("speedtest=%+v", obs.Speedtest)
	}
	w := obs.Speedtest[0]
	if len(w.Points) != 1 || w.Points[0].TS != 1700000000 || w.Points[0].Down != 200 {
		t.Fatalf("points %+v", w.Points)
	}
	if w.Down != 200 {
		t.Fatalf("latest %+v", w)
	}
	after := time.Now().Unix()
	for _, p := range w.Points {
		if p.TS >= before && p.TS <= after+1 {
			t.Fatalf("history point stamped with wall clock: ts=%d before=%d after=%d", p.TS, before, after)
		}
	}
}
