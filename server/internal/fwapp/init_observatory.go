package fwapp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"fireproxy/pkg/inventory"
)

// ObservatorySnapshot is typed observatory data extracted from fw-app init.
// Ranked flow slices stay empty when systemFlows has no ranked rows.
type ObservatorySnapshot struct {
	AlarmCount         int64
	PendingAlarmCount  int64
	ArchivedAlarmCount int64
	NewAlarms          []AlarmSample
	Transfer24h        inventory.Transfer
	MonthlyWANs        []inventory.WANUsage
	Speedtest          []inventory.SpeedtestWAN
	Box                *inventory.Box
	Devices            []inventory.Device
	SysMetrics         *InitSysMetrics
	NICStates          []InitNICState
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
	Load1    float64 `json:"load1"`
	Load5    float64 `json:"load5"`
	Load15   float64 `json:"load15"`
	MemUsage float64 `json:"mem_usage"`
	TotalMem float64 `json:"total_mem"`
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

	out := ObservatorySnapshot{
		AlarmCount:         int64(root.ActiveAlarmCount),
		PendingAlarmCount:  int64(root.PendingAlarmCount),
		ArchivedAlarmCount: int64(root.ArchivedAlarmCount),
		NewAlarms:          parseAlarmSamples(root.NewAlarms),
		Transfer24h:        parseNewLast24(root.NewLast24),
		MonthlyWANs:        parseMonthlyWANs(root.MonthlyDataUsageOnWans, root.NetworkProfiles, root.Network),
		Speedtest:          parseInitSpeedtest(root.InternetSpeedtestResults, root.NetworkProfiles, root.Network),
		Box:                parseInitBox(root),
		Devices:            parseInitDevices(root.Hosts),
		SysMetrics:         parseInitSysMetrics(root.SysMetrics),
		NICStates:          parseInitNICStates(root.NICStates),
	}
	return out, nil
}

type initObservatoryRoot struct {
	ActiveAlarmCount         flexFloat                     `json:"activeAlarmCount"`
	PendingAlarmCount        flexFloat                     `json:"pendingAlarmCount"`
	ArchivedAlarmCount       flexFloat                     `json:"archivedAlarmCount"`
	NewAlarms                []json.RawMessage             `json:"newAlarms"`
	NewLast24                *rawNewLast24                 `json:"newLast24"`
	MonthlyDataUsageOnWans   map[string]rawMonthlyWANUsage `json:"monthlyDataUsageOnWans"`
	InternetSpeedtestResults []json.RawMessage             `json:"internetSpeedtestResults"`
	Hosts                    []rawObsHost                  `json:"hosts"`
	SysMetrics               *rawSysMetrics                `json:"sysMetrics"`
	NICStates                map[string]rawNICState        `json:"nicStates"`
	NetworkProfiles          map[string]rawNetProfile      `json:"networkProfiles"`
	Network                  *rawNetwork                   `json:"network"`

	Model             string    `json:"model"`
	Device            string    `json:"device"`
	GroupName         string    `json:"groupName"`
	PublicIP          string    `json:"publicIp"`
	DDNS              string    `json:"ddns"`
	Mode              string    `json:"mode"`
	Timezone          string    `json:"timezone"`
	LocalDomainSuffix string    `json:"localDomainSuffix"`
	VersionStr        string    `json:"versionStr"`
	LongVersion       string    `json:"longVersion"`
	Version           flexFloat `json:"version"`
	EID               string    `json:"eid"`
}

type rawNewLast24 struct {
	TotalUpload   flexFloat     `json:"totalUpload"`
	TotalDownload flexFloat     `json:"totalDownload"`
	Upload        [][]flexFloat `json:"upload"`
	Download      [][]flexFloat `json:"download"`
	Conn          [][]flexFloat `json:"conn"`
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
	Load1    flexFloat `json:"load1"`
	Load5    flexFloat `json:"load5"`
	Load15   flexFloat `json:"load15"`
	MemUsage flexFloat `json:"memUsage"`
	TotalMem flexFloat `json:"totalMem"`
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

func parseNewLast24(src *rawNewLast24) inventory.Transfer {
	if src == nil {
		return inventory.Transfer{}
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
	return inventory.Transfer{
		Upload:   int64(src.TotalUpload),
		Download: int64(src.TotalDownload),
		Points:   pts,
	}
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
			return n
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
			return n
		}
	}
	return uuid
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
	return &inventory.Box{
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
		tags := make([]string, 0, len(h.Tags))
		for _, t := range h.Tags {
			s := strings.TrimSpace(string(t))
			if s == "" {
				continue
			}
			tags = append(tags, s)
		}
		dev := inventory.Device{}
		dev.MAC = mac
		dev.Name = name
		dev.IP = strings.TrimSpace(h.IP)
		dev.Vendor = strings.TrimSpace(h.MacVendor)
		dev.LocalDomain = strings.TrimSpace(h.LocalDomain)
		dev.LastActiveTS = float64(h.LastActive)
		dev.TagIDs = tags
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
