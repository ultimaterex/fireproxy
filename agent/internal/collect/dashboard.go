package collect

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	pkggeo "fireproxy/pkg/geo"
	"fireproxy/pkg/inventory"
)

func (c *Collector) collectDashboard(now time.Time, nets []inventory.NetworkIface, devs []inventory.Device, cache map[string]string) *inventory.Dashboard {
	if c.Redis == nil {
		return nil
	}
	out := &inventory.Dashboard{}
	if n, err := c.Redis.ZCard("alarm_active"); err == nil {
		out.AlarmCount = n
	}

	// Match Firewalla's 24h chart: hourly buckets (not 15m).
	up, _ := timedHits(c.Redis, "upload", granHour, 24, now)
	down, _ := timedHits(c.Redis, "download", granHour, 24, now)
	connPts, _ := timedHits(c.Redis, "conn", granHour, 24, now)
	out.Transfer24h = mergeTransfer(up, down, connPts)

	ipB, _ := timedHits(c.Redis, "ipB", granHour, 24, now)
	dnsB, _ := timedHits(c.Redis, "dnsB", granHour, 24, now)
	conn, _ := timedHits(c.Redis, "conn", granHour, 24, now)
	out.Blocked = inventory.BlockedMix{
		Blocked: sumHits(ipB) + sumHits(dnsB),
		Allowed: sumHits(conn),
	}

	days := daysThisCycle(now, 1)
	for _, n := range nets {
		if !isISPWan(n) {
			continue
		}
		metric := "wan:" + n.UUID
		wUp, _ := timedHits(c.Redis, "upload:"+metric, granDay, days, now)
		wDown, _ := timedHits(c.Redis, "download:"+metric, granDay, days, now)
		name := n.Desc
		if name == "" {
			name = n.Name
		}
		out.MonthlyWANs = append(out.MonthlyWANs, inventory.WANUsage{
			UUID:     n.UUID,
			Name:     name,
			Upload:   sumHits(wUp),
			Download: sumHits(wDown),
		})
	}

	names := map[string]string{}
	types := map[string]string{}
	for _, d := range devs {
		mac := strings.ToUpper(d.MAC)
		names[mac] = d.Name
		if names[mac] == "" {
			names[mac] = d.LocalDomain
		}
		if d.Type != "" {
			types[mac] = d.Type
		}
	}
	byDev := map[string]*rankAcc{}
	byDest := map[string]*rankAcc{}
	byReg := map[string]*rankAcc{}
	byRegDev := map[string]map[string]*rankAcc{}
	byRegDest := map[string]map[string]*rankAcc{}
	byDevDest := map[string]map[string]*rankAcc{}
	byDestDev := map[string]map[string]*rankAcc{}
	c.accumulateSumflow("lastsyssumflow:upload", now, names, types, true, byDev, byDest, byReg, byRegDev, byRegDest, byDevDest, byDestDev)
	c.accumulateSumflow("lastsyssumflow:download", now, names, types, false, byDev, byDest, byReg, byRegDev, byRegDest, byDevDest, byDestDev)
	out.TopUpload = topN(byDev, 5, "up")
	out.TopDownload = topN(byDev, 5, "down")
	out.DestFlows = topN(byDest, 0, "total")
	for i := range out.DestFlows {
		out.DestFlows[i].Devices = topN(byDestDev[out.DestFlows[i].ID], 8, "total")
	}
	out.TopDestUpload = topN(byDest, 5, "up")
	out.TopDestDownload = topN(byDest, 5, "down")
	out.TopRegions = topN(byReg, 5, "total")
	for i := range out.TopRegions {
		cc := out.TopRegions[i].Country
		if cc == "" {
			cc = out.TopRegions[i].ID
		}
		out.TopRegions[i].Devices = topN(byRegDev[cc], 8, "total")
		out.TopRegions[i].Targets = topN(byRegDest[cc], 8, "total")
	}
	for i := range devs {
		mac := strings.ToUpper(devs[i].MAC)
		if m := byDevDest[mac]; len(m) > 0 {
			devs[i].TopDests = topN(m, 8, "total")
		}
	}
	out.Speedtest = c.collectSpeedtest(nets, cache)
	out.DNS = c.collectDNSHealth(now)
	return out
}

func (c *Collector) collectSpeedtest(nets []inventory.NetworkIface, cache map[string]string) []inventory.SpeedtestWAN {
	app, _ := c.Redis.HGet("policy:system", "app")
	plans := parseWANPlans(app)
	active := wanActiveByID(cache)
	hist := c.loadSpeedHistory()
	needWifi := false
	for _, n := range nets {
		if !isISPWan(n) || len(hist[n.UUID]) > 0 {
			continue
		}
		if active[n.UUID] || active[n.Name] || active[n.Desc] {
			needWifi = true
			break
		}
	}
	var latest speedLast
	if needWifi {
		raw, _ := c.Redis.HGet("policy:system", "stats:wifi_speedtest")
		latest, _ = parseSpeedPoint(raw, 0)
	}

	var out []inventory.SpeedtestWAN
	for _, n := range nets {
		if !isISPWan(n) {
			continue
		}
		name := n.Desc
		if name == "" {
			name = n.Name
		}
		row := inventory.SpeedtestWAN{
			UUID:   n.UUID,
			Name:   name,
			Active: active[n.UUID] || active[n.Name] || active[name],
			Points: hist[n.UUID],
		}
		if p, ok := plans[n.UUID]; ok {
			row.PlanDown = p.down
			row.PlanUp = p.up
		}
		if n := len(row.Points); n > 0 {
			last := row.Points[n-1]
			row.Down, row.Up, row.Ping = last.Down, last.Up, last.Ping
			row.ServerID, row.Server, row.Location = last.ServerID, last.Server, last.Location
		} else if row.Active && (latest.Down > 0 || latest.Up > 0) {
			row.Down, row.Up, row.Ping, row.Jitter = latest.Down, latest.Up, latest.Ping, latest.jitter
			row.ServerID, row.Server, row.Location = latest.ServerID, latest.Server, latest.Location
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Active != out[j].Active {
			return out[i].Active
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (c *Collector) loadSpeedHistory() map[string][]inventory.SpeedtestPoint {
	out := map[string][]inventory.SpeedtestPoint{}
	members, err := c.Redis.ZRevRangeByScore("internet_speedtest_results", "+inf", "0", 0, 50)
	if err != nil {
		return out
	}
	for i := len(members) - 1; i >= 0; i-- {
		p, ok := parseSpeedPoint(members[i].Member, members[i].Score)
		if !ok || p.uuid == "" {
			continue
		}
		out[p.uuid] = append(out[p.uuid], p.SpeedtestPoint)
	}
	return out
}

func (c *Collector) collectDNSHealth(now time.Time) *inventory.DNSHealth {
	hits, _ := timedHits(c.Redis, "dns", granHour, 24, now)
	queries := make([]inventory.DNSQuery, 0, len(hits))
	var n int64
	for _, h := range hits {
		queries = append(queries, inventory.DNSQuery{TS: h.TS, Count: h.Value})
		n += h.Value
	}
	if n == 0 {
		return nil
	}
	return &inventory.DNSHealth{Queries: queries}
}

type wanPlan struct {
	down, up float64
}

type speedLast struct {
	inventory.SpeedtestPoint
	jitter float64
	uuid   string
}

func parseWANPlans(appJSON string) map[string]wanPlan {
	out := map[string]wanPlan{}
	if strings.TrimSpace(appJSON) == "" {
		return out
	}
	var app struct {
		Bandwidth struct {
			WanConfs map[string]struct {
				Upload   float64 `json:"upload"`
				Download float64 `json:"download"`
			} `json:"wanConfs"`
		} `json:"bandwidth"`
	}
	if json.Unmarshal([]byte(appJSON), &app) != nil {
		return out
	}
	for uuid, w := range app.Bandwidth.WanConfs {
		out[uuid] = wanPlan{down: w.Download, up: w.Upload}
	}
	return out
}

func wanActiveByID(cache map[string]string) map[string]bool {
	out := map[string]bool{}
	for field, raw := range cache {
		if !strings.HasPrefix(field, "wan_state:") {
			continue
		}
		iface := strings.TrimPrefix(field, "wan_state:")
		var ev struct {
			Active bool `json:"active"`
			Labels *struct {
				Active      bool   `json:"active"`
				WanIntfName string `json:"wan_intf_name"`
				WanIntfUUID string `json:"wan_intf_uuid"`
			} `json:"labels"`
		}
		if json.Unmarshal([]byte(raw), &ev) != nil {
			continue
		}
		on := ev.Active
		if ev.Labels != nil {
			on = ev.Labels.Active
			if ev.Labels.WanIntfUUID != "" {
				out[ev.Labels.WanIntfUUID] = on
			}
			if ev.Labels.WanIntfName != "" {
				out[ev.Labels.WanIntfName] = on
			}
		}
		if iface != "" {
			out[iface] = on
		}
	}
	return out
}

func parseSpeedPoint(raw string, score float64) (speedLast, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "{") {
		return speedLast{}, false
	}
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return speedLast{}, false
	}
	p := speedLast{
		SpeedtestPoint: inventory.SpeedtestPoint{
			TS:   int64(score),
			Down: jsonFloat(m, "download", "dlStatus", "dl"),
			Up:   jsonFloat(m, "upload", "ulStatus", "ul"),
			Ping: jsonFloat(m, "latency", "pingStatus", "ping"),
		},
		jitter: jsonFloat(m, "jitter", "jitterStatus"),
	}
	if res, ok := m["result"].(map[string]any); ok {
		if v := jsonFloat(res, "download"); v > 0 {
			p.Down = v
		}
		if v := jsonFloat(res, "upload"); v > 0 {
			p.Up = v
		}
		if v := jsonFloat(res, "latency"); v > 0 {
			p.Ping = v
		}
		if v := jsonFloat(res, "jitter"); v > 0 {
			p.jitter = v
		}
	}
	p.ServerID, p.Server, p.Location = speedtestServerFields(m)
	if u, ok := m["uuid"].(string); ok {
		p.uuid = u
	}
	if v, ok := m["success"].(bool); ok && !v {
		return speedLast{}, false
	}
	if ts := jsonFloat(m, "timestamp", "ts"); ts > 0 {
		if ts > 1e12 {
			ts /= 1000
		}
		p.TS = int64(ts)
	}
	if p.Down == 0 && p.Up == 0 {
		return speedLast{}, false
	}
	return p, true
}

func speedtestServerFields(m map[string]any) (id, sponsor, location string) {
	srv, _ := m["server"].(map[string]any)
	if srv == nil {
		return "", "", ""
	}
	id = jsonString(srv, "id", "serverid")
	sponsor = jsonString(srv, "sponsor")
	location = jsonString(srv, "location", "name")
	return id, sponsor, location
}

func jsonString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				return s
			}
		case float64:
			return strconv.FormatInt(int64(t), 10)
		case json.Number:
			return t.String()
		}
	}
	return ""
}

func jsonFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case float64:
			return t
		case json.Number:
			f, err := t.Float64()
			if err == nil {
				return f
			}
		case string:
			f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
			if err == nil {
				return f
			}
		}
	}
	return 0
}

func mergeTransfer(up, down, conn []tsHit) inventory.Transfer {
	byTS := map[int64]*inventory.BytePoint{}
	order := []int64{}
	add := func(hits []tsHit, kind string) {
		for _, h := range hits {
			p := byTS[h.TS]
			if p == nil {
				p = &inventory.BytePoint{TS: h.TS}
				byTS[h.TS] = p
				order = append(order, h.TS)
			}
			switch kind {
			case "up":
				p.Upload = h.Value
			case "down":
				p.Download = h.Value
			case "conn":
				p.Conn = h.Value
			}
		}
	}
	add(up, "up")
	add(down, "down")
	add(conn, "conn")
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	pts := make([]inventory.BytePoint, 0, len(order))
	var tu, td int64
	for _, ts := range order {
		p := *byTS[ts]
		pts = append(pts, p)
		tu += p.Upload
		td += p.Download
	}
	return inventory.Transfer{Upload: tu, Download: td, Points: pts}
}

func isISPWan(n inventory.NetworkIface) bool {
	if !strings.EqualFold(n.Type, "wan") || strings.TrimSpace(n.UUID) == "" {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(n.Name))
	desc := strings.TrimSpace(n.Desc)
	if desc == "" || strings.EqualFold(desc, n.Name) {
		return false
	}
	dlow := strings.ToLower(desc)
	if strings.Contains(name, "vpn") || strings.Contains(dlow, "vpn") {
		return false
	}
	for _, p := range []string{"wg", "tun", "ipsec", "tailscale", "zt"} {
		if strings.HasPrefix(name, p) {
			return false
		}
	}
	return !looksLikeNIC(dlow)
}

func looksLikeNIC(s string) bool {
	for _, p := range []string{"eth", "enp", "ens", "enx", "wlan", "wlp", "br", "bond", "vlan"} {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return strings.HasPrefix(s, "wan") && len(s) <= 5
}

func daysThisCycle(now time.Time, billingDay int) int {
	if billingDay < 1 || billingDay > 28 {
		billingDay = 1
	}
	y, m, d := now.Date()
	beginDay := billingDay
	beginMonth := m
	beginYear := y
	if d < billingDay {
		beginMonth--
		if beginMonth == 0 {
			beginMonth = 12
			beginYear--
		}
	}
	begin := time.Date(beginYear, beginMonth, beginDay, 0, 0, 0, 0, time.UTC)
	n := int(now.Sub(begin).Hours()/24) + 1
	if n < 1 {
		return 1
	}
	if n > 40 {
		return 40
	}
	return n
}

func (c *Collector) attachDeviceTraffic(now time.Time, devs []inventory.Device) {
	if c.Redis == nil {
		return
	}
	for i := range devs {
		mac := strings.ToUpper(strings.TrimSpace(devs[i].MAC))
		if mac == "" {
			continue
		}
		up, _ := timedHits(c.Redis, "upload:"+mac, granHour, 24, now)
		down, _ := timedHits(c.Redis, "download:"+mac, granHour, 24, now)
		devs[i].Upload = sumHits(up)
		devs[i].Download = sumHits(down)
	}
}

func (c *Collector) accumulateSumflow(
	pointer string,
	now time.Time,
	names, types map[string]string,
	upload bool,
	byDev, byDest, byReg map[string]*rankAcc,
	byRegDev, byRegDest, byDevDest, byDestDev map[string]map[string]*rankAcc,
) {
	key, err := c.Redis.Get(pointer)
	if err == nil && key != "" {
		if c.accumulateSumflowKey(key, names, types, upload, byDev, byDest, byReg, byRegDev, byRegDest, byDevDest, byDestDev) {
			return
		}
	}
	dim := strings.TrimPrefix(pointer, "last")
	if !strings.HasPrefix(dim, "syssumflow:") {
		return
	}
	hour := (now.Unix() / 3600) * 3600
	for i := 0; i < 24; i++ {
		begin := hour - int64(i)*3600
		end := begin + 3600
		hourly := dim + ":" + strconv.FormatInt(begin, 10) + ":" + strconv.FormatInt(end, 10)
		c.accumulateSumflowKey(hourly, names, types, upload, byDev, byDest, byReg, byRegDev, byRegDest, byDevDest, byDestDev)
	}
}

func (c *Collector) accumulateSumflowKey(
	key string,
	names, types map[string]string,
	upload bool,
	byDev, byDest, byReg map[string]*rankAcc,
	byRegDev, byRegDest, byDevDest, byDestDev map[string]map[string]*rankAcc,
) bool {
	members, err := c.Redis.ZRevRangeByScore(key, "+inf", "0", 0, 80)
	if err != nil {
		return false
	}
	got := false
	for _, z := range members {
		if z.Member == "_" || z.Member == "" || z.Score <= 0 {
			continue
		}
		var row struct {
			Device  string `json:"device"`
			DestIP  string `json:"destIP"`
			Domain  string `json:"domain"`
			Country string `json:"country"`
		}
		if json.Unmarshal([]byte(z.Member), &row) != nil {
			continue
		}
		got = true
		bytes := int64(z.Score)
		mac := ""
		if row.Device != "" {
			mac = strings.ToUpper(row.Device)
			a := byDev[mac]
			if a == nil {
				name := names[mac]
				if name == "" {
					name = row.Device
				}
				a = &rankAcc{id: mac, name: name, typ: types[mac]}
				byDev[mac] = a
			}
			addDir(a, upload, bytes)
		}
		destID := row.Domain
		destName := row.Domain
		if destID == "" {
			destID = row.DestIP
			destName = row.DestIP
		}
		cc := pkggeo.Code(row.Country)
		region := countryName(cc)
		if destID != "" && destID != "0.0.0.0" {
			a := byDest[destID]
			if a == nil {
				a = &rankAcc{id: destID, name: destName, destIP: row.DestIP, cc: cc, region: region}
				byDest[destID] = a
			}
			if a.destIP == "" {
				a.destIP = row.DestIP
			}
			if a.cc == "" {
				a.cc = cc
			}
			if a.region == "" {
				a.region = region
			}
			addDir(a, upload, bytes)
		}
		if cc != "" {
			a := byReg[cc]
			if a == nil {
				a = &rankAcc{id: cc, name: region, cc: cc}
				byReg[cc] = a
			}
			addDir(a, upload, bytes)
			if mac != "" {
				devName := names[mac]
				if devName == "" {
					devName = row.Device
				}
				bumpNested(byRegDev, cc, mac, devName, types[mac], "", cc, region, upload, bytes)
			}
			if destID != "" && destID != "0.0.0.0" {
				bumpNested(byRegDest, cc, destID, destName, "", row.DestIP, cc, region, upload, bytes)
			}
		}
		if mac != "" && destID != "" && destID != "0.0.0.0" {
			devName := names[mac]
			if devName == "" {
				devName = row.Device
			}
			bumpNested(byDevDest, mac, destID, destName, "", row.DestIP, cc, region, upload, bytes)
			bumpNested(byDestDev, destID, mac, devName, types[mac], "", cc, region, upload, bytes)
		}
	}
	return got
}

func bumpNested(
	m map[string]map[string]*rankAcc,
	outer, id, name, typ, destIP, cc, region string,
	upload bool, n int64,
) {
	inner := m[outer]
	if inner == nil {
		inner = map[string]*rankAcc{}
		m[outer] = inner
	}
	a := inner[id]
	if a == nil {
		a = &rankAcc{id: id, name: name, typ: typ, destIP: destIP, cc: cc, region: region}
		inner[id] = a
	}
	if a.name == "" && name != "" {
		a.name = name
	}
	if a.destIP == "" {
		a.destIP = destIP
	}
	if a.cc == "" {
		a.cc = cc
	}
	if a.region == "" {
		a.region = region
	}
	addDir(a, upload, n)
}

type rankAcc struct {
	id     string
	name   string
	typ    string
	destIP string
	cc     string
	region string
	up     int64
	down   int64
}

func addDir(a *rankAcc, upload bool, n int64) {
	if upload {
		a.up += n
	} else {
		a.down += n
	}
}

func topN(m map[string]*rankAcc, n int, key string) []inventory.RankedFlow {
	out := make([]inventory.RankedFlow, 0, len(m))
	for _, a := range m {
		score := a.up + a.down
		switch key {
		case "up":
			score = a.up
		case "down":
			score = a.down
		}
		if score == 0 {
			continue
		}
		out = append(out, inventory.RankedFlow{
			ID:       a.id,
			Name:     a.name,
			Type:     a.typ,
			DestIP:   a.destIP,
			Country:  a.cc,
			Region:   a.region,
			Upload:   a.up,
			Download: a.down,
			Bytes:    a.up + a.down,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		vi, vj := out[i].Bytes, out[j].Bytes
		switch key {
		case "up":
			vi, vj = out[i].Upload, out[j].Upload
		case "down":
			vi, vj = out[i].Download, out[j].Download
		}
		if vi != vj {
			return vi > vj
		}
		return out[i].Name < out[j].Name
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}
