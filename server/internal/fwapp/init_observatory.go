package fwapp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"fireproxy/pkg/inventory"
	"fireproxy/pkg/snapshot"
)

// ObservatorySnapshot is typed observatory data extracted from fw-app init.
// Ranked flow slices stay empty when systemFlows has no ranked rows.
type ObservatorySnapshot struct {
	AlarmCount         int64
	PendingAlarmCount  int64
	ArchivedAlarmCount int64
	NewAlarms          []AlarmSample
	RuleCount          int
	Transfer24h        inventory.Transfer
	Transfer30d        inventory.Transfer
	Transfer60         inventory.Transfer // last60 window (finer grain)
	Transfer12m        inventory.Transfer
	Blocked            inventory.BlockedMix
	DNS                *inventory.DNSHealth
	MonthlyWANs        []inventory.WANUsage
	MonthlyBeginTS     int64
	MonthlyEndTS       int64
	Speedtest          []inventory.SpeedtestWAN
	Box                *inventory.Box
	Devices            []inventory.Device
	Tags               []inventory.Tag
	SysMetrics         *InitSysMetrics
	NICStates          []InitNICState
	NICMetrics         []InitNICMetric
	WAN                map[string]snapshot.WANLink
	WGPeers            []InitWGPeer
	WGClients          []InitWGClient
	VIPs               []InitVIP
	VirtWANs           []InitVirtWAN
	TopUpload          []inventory.RankedFlow
	TopDownload        []inventory.RankedFlow
	TopDestUpload      []inventory.RankedFlow
	TopDestDownload    []inventory.RankedFlow
	TopRegions         []inventory.RankedFlow
	DestFlows          []inventory.RankedFlow
}

// AlarmSample is a condensed newAlarms row from init.
type AlarmSample struct {
	AID        int64   `json:"aid"`
	Type       string  `json:"type,omitempty"`
	Message    string  `json:"message,omitempty"`
	Timestamp  float64 `json:"timestamp,omitempty"`
	DeviceMAC  string  `json:"device_mac,omitempty"`
	DeviceIP   string  `json:"device_ip,omitempty"`
	DeviceName string  `json:"device_name,omitempty"`
}

// InitSysMetrics is a one-shot sysMetrics sample from init.
type InitSysMetrics struct {
	Load1    float64       `json:"load1"`
	Load5    float64       `json:"load5"`
	Load15   float64       `json:"load15"`
	MemUsage float64       `json:"mem_usage"`
	TotalMem float64       `json:"total_mem"`
	CPU      *snapshot.CPU `json:"cpu,omitempty"`
	Disks    []InitDisk    `json:"disks,omitempty"`
}

// InitNICState is carrier/speed state for one NIC from init nicStates.
type InitNICState struct {
	Name    string `json:"name"`
	Address string `json:"address,omitempty"`
	Carrier string `json:"carrier,omitempty"`
	Duplex  string `json:"duplex,omitempty"`
	Speed   string `json:"speed,omitempty"`
}

// ParseInitObservatory normalizes fw-app init JSON into observatory fallback fields.
// Accepts either the unwrapped init data object or a full envelope with mtype/data.
func ParseInitObservatory(raw []byte) (ObservatorySnapshot, error) {
	data, err := unwrapInitData(raw)
	if err != nil {
		return ObservatorySnapshot{}, err
	}

	var root initObservatoryRoot
	if err := json.Unmarshal(data, &root); err != nil {
		return ObservatorySnapshot{}, fmt.Errorf("fwapp: parse init observatory: %w", err)
	}

	transfer, blocked, dns := parseNewLast24(root.NewLast24)
	if resolvers := parseStateDNSResolvers(root.LatestAllStateEvents); len(resolvers) > 0 {
		if dns == nil {
			dns = &inventory.DNSHealth{}
		}
		dns.Resolvers = resolvers
	}
	out := ObservatorySnapshot{
		AlarmCount:         int64(root.ActiveAlarmCount),
		PendingAlarmCount:  int64(root.PendingAlarmCount),
		ArchivedAlarmCount: int64(root.ArchivedAlarmCount),
		NewAlarms:          parseAlarmSamples(root.NewAlarms),
		RuleCount:          parseRuleCount(root),
		Transfer24h:        transfer,
		Transfer30d:        parseTransferWindow(root.Last30),
		Transfer60:         parseTransferWindow(root.Last60),
		Transfer12m:        parseTransferWindow(root.Last12Months),
		Blocked:            blocked,
		DNS:                dns,
		MonthlyWANs:        parseMonthlyWANs(root.MonthlyDataUsageOnWans, root.NetworkProfiles, root.Network),
		Speedtest:          parseInitSpeedtest(root.InternetSpeedtestResults, root.NetworkProfiles, root.Network),
		Box:                parseInitBox(root),
		Devices:            parseInitDevices(root.Hosts),
		Tags:               parseInitTags(root.Tags, root.UserTags, root.DeviceTags),
		SysMetrics:         parseInitSysMetrics(root.SysMetrics),
		NICStates:          parseInitNICStates(root.NICStates),
		NICMetrics:         parseNICMetrics(root.NetworkMetrics),
		WAN:                parseStateWAN(root.LatestAllStateEvents),
		WGPeers:            parseWGPeers(root.WGPeers),
		WGClients:          parseWGClients(root.WGVPNClientProfiles),
		VIPs:               parseVIPs(root.VIPProfiles),
		VirtWANs:           parseVirtWANs(root.VirtWanGroups),
	}
	if root.MonthlyDataUsage != nil {
		out.MonthlyBeginTS = int64(root.MonthlyDataUsage.MonthlyBeginTs)
		out.MonthlyEndTS = int64(root.MonthlyDataUsage.MonthlyEndTs)
	}
	return out, nil
}

type initObservatoryRoot struct {
	ActiveAlarmCount         flexFloat                     `json:"activeAlarmCount"`
	PendingAlarmCount        flexFloat                     `json:"pendingAlarmCount"`
	ArchivedAlarmCount       flexFloat                     `json:"archivedAlarmCount"`
	NewAlarms                []json.RawMessage             `json:"newAlarms"`
	PolicyRuleNumber         flexFloat                     `json:"policyRuleNumber"`
	PolicyRules              []json.RawMessage             `json:"policyRules"`
	NewLast24                *rawNewLast24                 `json:"newLast24"`
	Last30                   map[string]json.RawMessage    `json:"last30"`
	Last60                   map[string]json.RawMessage    `json:"last60"`
	Last12Months             map[string]json.RawMessage    `json:"last12Months"`
	MonthlyDataUsage         *rawMonthlyCycle              `json:"monthlyDataUsage"`
	MonthlyDataUsageOnWans   map[string]rawMonthlyWANUsage `json:"monthlyDataUsageOnWans"`
	InternetSpeedtestResults []json.RawMessage             `json:"internetSpeedtestResults"`
	Hosts                    []rawObsHost                  `json:"hosts"`
	Tags                     map[string]rawTagEntry        `json:"tags"`
	UserTags                 map[string]rawTagEntry        `json:"userTags"`
	DeviceTags               map[string]rawTagEntry        `json:"deviceTags"`
	SysMetrics               *rawSysMetrics                `json:"sysMetrics"`
	NICStates                map[string]rawNICState        `json:"nicStates"`
	NetworkMetrics           map[string]rawNICPercentiles  `json:"networkMetrics"`
	NetworkProfiles          map[string]rawNetProfile      `json:"networkProfiles"`
	Network                  *rawNetwork                   `json:"network"`
	LatestAllStateEvents     map[string]json.RawMessage    `json:"latestAllStateEvents"`
	WGPeers                  []rawWGPeer                   `json:"wgPeers"`
	WGVPNClientProfiles      []rawWGClient                 `json:"wgvpnClientProfiles"`
	VIPProfiles              []rawVIP                      `json:"vipProfiles"`
	VirtWanGroups            []rawVirtWAN                  `json:"virtWanGroups"`

	Model             string            `json:"model"`
	Device            string            `json:"device"`
	GroupName         string            `json:"groupName"`
	PublicIP          string            `json:"publicIp"`
	PublicIPs         map[string]string `json:"publicIps"`
	DDNS              string            `json:"ddns"`
	Mode              string            `json:"mode"`
	Timezone          string            `json:"timezone"`
	LocalDomainSuffix string            `json:"localDomainSuffix"`
	VersionStr        string            `json:"versionStr"`
	LongVersion       string            `json:"longVersion"`
	Version           flexFloat         `json:"version"`
	EID               string            `json:"eid"`
	Uptime            flexFloat         `json:"uptime"`
	OsUptime          flexFloat         `json:"osUptime"`
	CloudConnected    *bool             `json:"cloudConnected"`
}

type rawNewLast24 struct {
	TotalUpload   flexFloat     `json:"totalUpload"`
	TotalDownload flexFloat     `json:"totalDownload"`
	TotalConn     flexFloat     `json:"totalConn"`
	TotalDnsB     flexFloat     `json:"totalDnsB"`
	TotalIpB      flexFloat     `json:"totalIpB"`
	Upload        [][]flexFloat `json:"upload"`
	Download      [][]flexFloat `json:"download"`
	Conn          [][]flexFloat `json:"conn"`
	DNS           [][]flexFloat `json:"dns"`
}

type rawMonthlyWANUsage struct {
	TotalUpload   flexFloat `json:"totalUpload"`
	TotalDownload flexFloat `json:"totalDownload"`
}

type rawObsHost struct {
	MAC         string          `json:"mac"`
	Name        string          `json:"name"`
	BName       string          `json:"bname"`
	IP          string          `json:"ip"`
	MacVendor   string          `json:"macVendor"`
	Tags        []flexString    `json:"tags"`
	DeviceTags  []flexString    `json:"deviceTags"`
	UserTags    []flexString    `json:"userTags"`
	LastActive  flexFloat       `json:"lastActive"`
	LocalDomain string          `json:"localDomain"`
	FlowSummary *rawFlowSummary `json:"flowsummary"`
	Detect      json.RawMessage `json:"detect"`
}

type rawFlowSummary struct {
	InBytes  flexFloat `json:"inbytes"`
	OutBytes flexFloat `json:"outbytes"`
}

type rawSysMetrics struct {
	Load1     flexFloat      `json:"load1"`
	Load5     flexFloat      `json:"load5"`
	Load15    flexFloat      `json:"load15"`
	MemUsage  flexFloat      `json:"memUsage"`
	TotalMem  flexFloat      `json:"totalMem"`
	CPUUsage1 []rawCPUSample `json:"cpuUsage1"`
	DiskInfo  []rawDiskInfo  `json:"diskInfo"`
}

type rawNICState struct {
	Address string `json:"address"`
	Carrier string `json:"carrier"`
	Duplex  string `json:"duplex"`
	Speed   string `json:"speed"`
}

type rawNetProfile struct {
	UUID string `json:"uuid"`
	Type string `json:"type"`
	Name string `json:"name"`
	Desc string `json:"desc"`
	Intf string `json:"intf"`
}

type rawNetwork struct {
	UUID string `json:"uuid"`
	Desc string `json:"desc"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func parseAlarmSamples(raw []json.RawMessage) []AlarmSample {
	out := make([]AlarmSample, 0, len(raw))
	for _, item := range raw {
		var m map[string]any
		if json.Unmarshal(item, &m) != nil {
			continue
		}
		a := AlarmSample{
			AID:        int64(jsonFloat(m, "aid")),
			Type:       strings.TrimSpace(ooklaString(m, "type")),
			Message:    strings.TrimSpace(ooklaString(m, "message")),
			Timestamp:  jsonFloat(m, "timestamp", "alarmTimestamp"),
			DeviceMAC:  NormalizeMAC(ooklaString(m, "p.device.mac")),
			DeviceIP:   strings.TrimSpace(ooklaString(m, "p.device.ip")),
			DeviceName: strings.TrimSpace(ooklaString(m, "p.device.name")),
		}
		out = append(out, a)
	}
	return out
}

func parseRuleCount(root initObservatoryRoot) int {
	if n := int(root.PolicyRuleNumber); n > 0 {
		return n
	}
	return len(root.PolicyRules)
}

func parseNewLast24(src *rawNewLast24) (inventory.Transfer, inventory.BlockedMix, *inventory.DNSHealth) {
	if src == nil {
		return inventory.Transfer{}, inventory.BlockedMix{}, nil
	}
	upByTS := tsSeriesMap(src.Upload)
	downByTS := tsSeriesMap(src.Download)
	connByTS := tsSeriesMap(src.Conn)
	tsSet := map[int64]struct{}{}
	for ts := range upByTS {
		tsSet[ts] = struct{}{}
	}
	for ts := range downByTS {
		tsSet[ts] = struct{}{}
	}
	for ts := range connByTS {
		tsSet[ts] = struct{}{}
	}
	tss := make([]int64, 0, len(tsSet))
	for ts := range tsSet {
		tss = append(tss, ts)
	}
	sort.Slice(tss, func(i, j int) bool { return tss[i] < tss[j] })
	pts := make([]inventory.BytePoint, 0, len(tss))
	for _, ts := range tss {
		pts = append(pts, inventory.BytePoint{
			TS:       ts,
			Upload:   upByTS[ts],
			Download: downByTS[ts],
			Conn:     connByTS[ts],
		})
	}
	transfer := inventory.Transfer{
		Upload:   int64(src.TotalUpload),
		Download: int64(src.TotalDownload),
		Points:   pts,
	}
	// Match agent collectDashboard: blocked = dnsB+ipB; allowed = conn volume.
	blocked := inventory.BlockedMix{
		Blocked: int64(src.TotalDnsB) + int64(src.TotalIpB),
		Allowed: int64(src.TotalConn),
	}
	var dns *inventory.DNSHealth
	if queries := parseDNSQueries(src.DNS); len(queries) > 0 {
		dns = &inventory.DNSHealth{Queries: queries}
	}
	return transfer, blocked, dns
}

func parseDNSQueries(series [][]flexFloat) []inventory.DNSQuery {
	if len(series) == 0 {
		return nil
	}
	out := make([]inventory.DNSQuery, 0, len(series))
	for _, row := range series {
		if len(row) < 2 {
			continue
		}
		out = append(out, inventory.DNSQuery{
			TS:    int64(row[0]),
			Count: int64(row[1]),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TS < out[j].TS })
	return out
}

func tsSeriesMap(series [][]flexFloat) map[int64]int64 {
	out := map[int64]int64{}
	for _, row := range series {
		if len(row) < 2 {
			continue
		}
		out[int64(row[0])] = int64(row[1])
	}
	return out
}

func parseMonthlyWANs(src map[string]rawMonthlyWANUsage, profiles map[string]rawNetProfile, net *rawNetwork) []inventory.WANUsage {
	if len(src) == 0 {
		return nil
	}
	ids := make([]string, 0, len(src))
	for id := range src {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]inventory.WANUsage, 0, len(ids))
	for _, id := range ids {
		u := src[id]
		out = append(out, inventory.WANUsage{
			UUID:     id,
			Name:     wanDisplayName(id, profiles, net),
			Upload:   int64(u.TotalUpload),
			Download: int64(u.TotalDownload),
		})
	}
	return out
}

func wanDisplayName(uuid string, profiles map[string]rawNetProfile, net *rawNetwork) string {
	if net != nil && strings.TrimSpace(net.UUID) == uuid {
		if n := strings.TrimSpace(net.Desc); n != "" {
			return n
		}
		if n := strings.TrimSpace(net.Name); n != "" {
			return friendlyWANLabel(n)
		}
	}
	if p, ok := profiles[uuid]; ok {
		if n := strings.TrimSpace(p.Name); n != "" {
			return n
		}
		if n := strings.TrimSpace(p.Desc); n != "" {
			return n
		}
		if n := strings.TrimSpace(p.Intf); n != "" {
			return friendlyWANLabel(n)
		}
	}
	return uuid
}

// friendlyWANLabel turns bare iface names (eth1) into labels the Metrics UI
// keeps after isISPLabel filtering ("WAN (eth1)").
func friendlyWANLabel(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return n
	}
	lower := strings.ToLower(n)
	switch {
	case strings.HasPrefix(lower, "eth"),
		strings.HasPrefix(lower, "enp"),
		strings.HasPrefix(lower, "ens"),
		strings.HasPrefix(lower, "enx"),
		strings.HasPrefix(lower, "wlan"),
		strings.HasPrefix(lower, "wlp"),
		strings.HasPrefix(lower, "br"),
		strings.HasPrefix(lower, "bond"),
		strings.HasPrefix(lower, "vlan"),
		strings.HasPrefix(lower, "wg"),
		strings.HasPrefix(lower, "tun"),
		lower == "wan",
		strings.HasPrefix(lower, "wan") && len(lower) <= 4:
		return "WAN (" + n + ")"
	default:
		return n
	}
}

func parseInitSpeedtest(raw []json.RawMessage, profiles map[string]rawNetProfile, net *rawNetwork) []inventory.SpeedtestWAN {
	byUUID := map[string][]inventory.SpeedtestPoint{}
	for _, item := range raw {
		r, err := parseSpeedtestHistoryReply(item, "")
		if err != nil {
			continue
		}
		uuid := strings.TrimSpace(r.WanUUID)
		if uuid == "" {
			uuid = speedtestWanUUID(item)
		}
		if uuid == "" || r.TS == 0 {
			continue
		}
		byUUID[uuid] = append(byUUID[uuid], inventory.SpeedtestPoint{
			TS:       r.TS,
			Down:     r.Down,
			Up:       r.Up,
			Ping:     r.Ping,
			ServerID: r.ServerID,
			Server:   r.Server,
			Location: r.Location,
		})
	}
	if len(byUUID) == 0 {
		return nil
	}
	ids := make([]string, 0, len(byUUID))
	for id := range byUUID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]inventory.SpeedtestWAN, 0, len(ids))
	for _, id := range ids {
		pts := byUUID[id]
		sort.Slice(pts, func(i, j int) bool { return pts[i].TS < pts[j].TS })
		row := inventory.SpeedtestWAN{
			UUID:   id,
			Name:   wanDisplayName(id, profiles, net),
			Points: pts,
		}
		if n := len(pts); n > 0 {
			last := pts[n-1]
			row.Down, row.Up, row.Ping = last.Down, last.Up, last.Ping
			row.ServerID, row.Server, row.Location = last.ServerID, last.Server, last.Location
		}
		out = append(out, row)
	}
	return out
}

func parseInitBox(root initObservatoryRoot) *inventory.Box {
	name := strings.TrimSpace(root.GroupName)
	if name == "" {
		name = strings.TrimSpace(root.Device)
	}
	version := strings.TrimSpace(root.VersionStr)
	if version == "" {
		version = strings.TrimSpace(root.LongVersion)
	}
	if version == "" && root.Version > 0 {
		version = strconv.FormatFloat(float64(root.Version), 'f', -1, 64)
	}
	model := strings.TrimSpace(root.Model)
	if name == "" && model == "" && version == "" {
		return nil
	}
	box := &inventory.Box{
		Name:              name,
		PublicIP:          strings.TrimSpace(root.PublicIP),
		DDNS:              strings.TrimSpace(root.DDNS),
		Version:           version,
		Model:             model,
		EID:               strings.TrimSpace(root.EID),
		Mode:              strings.TrimSpace(root.Mode),
		Timezone:          strings.TrimSpace(root.Timezone),
		LocalDomainSuffix: strings.TrimSpace(root.LocalDomainSuffix),
	}
	if root.Uptime > 0 {
		u := int64(root.Uptime)
		box.UptimeSec = &u
	}
	if root.OsUptime > 0 {
		u := int64(root.OsUptime)
		box.OsUptimeSec = &u
	}
	if root.CloudConnected != nil {
		box.CloudConnected = root.CloudConnected
	}
	if len(root.PublicIPs) > 0 {
		box.PublicIPs = root.PublicIPs
	}
	return box
}

func parseInitDevices(hosts []rawObsHost) []inventory.Device {
	out := make([]inventory.Device, 0, len(hosts))
	for _, h := range hosts {
		mac := NormalizeMAC(h.MAC)
		if mac == "" {
			continue
		}
		name := strings.TrimSpace(h.Name)
		if name == "" {
			name = strings.TrimSpace(h.BName)
		}
		if name == "" {
			name = mac
		}
		dev := inventory.Device{}
		dev.MAC = mac
		dev.Name = name
		dev.IP = strings.TrimSpace(h.IP)
		dev.Vendor = strings.TrimSpace(h.MacVendor)
		dev.LocalDomain = strings.TrimSpace(h.LocalDomain)
		dev.LastActiveTS = float64(h.LastActive)
		dev.TagIDs = flexIDList(h.Tags)
		dev.DeviceTagIDs = flexIDList(h.DeviceTags)
		dev.UserTagIDs = flexIDList(h.UserTags)
		if h.FlowSummary != nil {
			// inbytes = download into the host; outbytes = upload from the host.
			dev.Download = int64(h.FlowSummary.InBytes)
			dev.Upload = int64(h.FlowSummary.OutBytes)
		}
		if typ := detectType(h.Detect); typ != "" {
			dev.Type = typ
		}
		out = append(out, dev)
	}
	return out
}

func detectType(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	return strings.TrimSpace(ooklaString(m, "type"))
}

func parseInitSysMetrics(src *rawSysMetrics) *InitSysMetrics {
	if src == nil {
		return nil
	}
	return &InitSysMetrics{
		Load1:    float64(src.Load1),
		Load5:    float64(src.Load5),
		Load15:   float64(src.Load15),
		MemUsage: float64(src.MemUsage),
		TotalMem: float64(src.TotalMem),
		CPU:      parseInitCPU(src.CPUUsage1),
		Disks:    parseInitDisks(src.DiskInfo),
	}
}

func parseInitNICStates(src map[string]rawNICState) []InitNICState {
	if len(src) == 0 {
		return nil
	}
	names := make([]string, 0, len(src))
	for name := range src {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]InitNICState, 0, len(names))
	for _, name := range names {
		n := src[name]
		out = append(out, InitNICState{
			Name:    name,
			Address: NormalizeMAC(n.Address),
			Carrier: strings.TrimSpace(n.Carrier),
			Duplex:  strings.TrimSpace(n.Duplex),
			Speed:   strings.TrimSpace(n.Speed),
		})
	}
	return out
}
