package fwapp

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"fireproxy/pkg/inventory"
	"fireproxy/pkg/snapshot"
)

// InitDisk is one sysMetrics.diskInfo mount from init.
type InitDisk struct {
	Mount      string  `json:"mount"`
	Filesystem string  `json:"filesystem,omitempty"`
	Capacity   float64 `json:"capacity"` // used fraction 0–1
	Size       int64   `json:"size,omitempty"`
	Used       int64   `json:"used,omitempty"`
	Available  int64   `json:"available,omitempty"`
}

// InitNICMetric is coarse rx/tx percentile traffic from networkMetrics (bytes, not live rates).
type InitNICMetric struct {
	Name     string `json:"name"`
	RxMedian int64  `json:"rx_median,omitempty"`
	TxMedian int64  `json:"tx_median,omitempty"`
	RxPt90   int64  `json:"rx_pt90,omitempty"`
	TxPt90   int64  `json:"tx_pt90,omitempty"`
}

// InitWGPeer is a WireGuard server peer from init wgPeers.
type InitWGPeer struct {
	Name       string  `json:"name,omitempty"`
	PublicKey  string  `json:"public_key,omitempty"`
	Intf       string  `json:"intf,omitempty"`
	RxBytes    int64   `json:"rx_bytes,omitempty"`
	TxBytes    int64   `json:"tx_bytes,omitempty"`
	LastActive float64 `json:"last_active,omitempty"`
}

// InitWGClient is a WireGuard VPN client profile.
type InitWGClient struct {
	ProfileID string `json:"profile_id,omitempty"`
	Status    string `json:"status,omitempty"`
	RemoteIP  string `json:"remote_ip,omitempty"`
	LocalIP   string `json:"local_ip,omitempty"`
}

// InitVIP is a VIP profile from init.
type InitVIP struct {
	UID  string `json:"uid,omitempty"`
	Name string `json:"name,omitempty"`
	IP   string `json:"ip,omitempty"`
}

// InitVirtWAN is a virtual WAN group (failover / load-balance).
type InitVirtWAN struct {
	UUID      string   `json:"uuid,omitempty"`
	Name      string   `json:"name,omitempty"`
	Type      string   `json:"type,omitempty"`
	ConnState string   `json:"conn_state,omitempty"`
	WANs      []string `json:"wans,omitempty"`
}

type rawCPUSample struct {
	User    flexFloat `json:"user"`
	Sys     flexFloat `json:"sys"`
	Idle    flexFloat `json:"idle"`
	Softirq flexFloat `json:"softirq"`
	IOWait  flexFloat `json:"iowait"`
}

type rawDiskInfo struct {
	Mount      string    `json:"mount"`
	Filesystem string    `json:"filesystem"`
	Capacity   flexFloat `json:"capacity"`
	Size       flexFloat `json:"size"`
	Used       flexFloat `json:"used"`
	Available  flexFloat `json:"available"`
}

type rawTagEntry struct {
	UID           flexString `json:"uid"`
	Name          string     `json:"name"`
	Type          string     `json:"type"`
	AffiliatedTag flexString `json:"affiliatedTag"`
}

type rawStateEvent struct {
	StateValue *flexFloat `json:"state_value"`
	Labels     *struct {
		NameServer  string `json:"name_server"`
		WanIntfName string `json:"wan_intf_name"`
		Name        string `json:"name"`
		Ready       *bool  `json:"ready"`
		Active      *bool  `json:"active"`
	} `json:"labels"`
}

type rawNICPercentiles struct {
	Rx *rawPercentileSet `json:"rx"`
	Tx *rawPercentileSet `json:"tx"`
}

type rawPercentileSet struct {
	Median flexString `json:"median"`
	Pt90   flexString `json:"pt90"`
	Min    flexString `json:"min"`
	Max    flexString `json:"max"`
	Pt75   flexString `json:"pt75"`
}

type rawMonthlyCycle struct {
	MonthlyBeginTs flexFloat `json:"monthlyBeginTs"`
	MonthlyEndTs   flexFloat `json:"monthlyEndTs"`
	TotalUpload    flexFloat `json:"totalUpload"`
	TotalDownload  flexFloat `json:"totalDownload"`
}

type rawWGPeer struct {
	Name                string    `json:"name"`
	PublicKey           string    `json:"publicKey"`
	Intf                string    `json:"intf"`
	RxBytes             flexFloat `json:"rxBytes"`
	TxBytes             flexFloat `json:"txBytes"`
	LastActiveTimestamp flexFloat `json:"lastActiveTimestamp"`
}

type rawWGClient struct {
	ProfileID string     `json:"profileId"`
	Status    flexString `json:"status"`
	RemoteIP  string     `json:"remoteIP"`
	LocalIP   string     `json:"localIP"`
	Message   string     `json:"message"`
}

type rawVIP struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}

type rawVirtWAN struct {
	UUID      string          `json:"uuid"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	ConnState json.RawMessage `json:"connState"`
	WANs      json.RawMessage `json:"wans"`
}

func parseInitCPU(samples []rawCPUSample) *snapshot.CPU {
	if len(samples) == 0 {
		return nil
	}
	s := samples[0]
	return &snapshot.CPU{
		User:    float64(s.User),
		Sys:     float64(s.Sys),
		Idle:    float64(s.Idle),
		Softirq: float64(s.Softirq),
	}
}

func parseInitDisks(raw []rawDiskInfo) []InitDisk {
	if len(raw) == 0 {
		return nil
	}
	out := make([]InitDisk, 0, len(raw))
	for _, d := range raw {
		mount := strings.TrimSpace(d.Mount)
		if mount == "" {
			continue
		}
		out = append(out, InitDisk{
			Mount:      mount,
			Filesystem: strings.TrimSpace(d.Filesystem),
			Capacity:   float64(d.Capacity),
			Size:       int64(d.Size),
			Used:       int64(d.Used),
			Available:  int64(d.Available),
		})
	}
	return out
}

func parseInitTags(groups, users, devices map[string]rawTagEntry) []inventory.Tag {
	out := make([]inventory.Tag, 0, len(groups)+len(users)+len(devices))
	appendMap := func(src map[string]rawTagEntry, defaultType string) {
		ids := make([]string, 0, len(src))
		for id := range src {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			e := src[id]
			uid := strings.TrimSpace(string(e.UID))
			if uid == "" {
				uid = id
			}
			typ := strings.TrimSpace(e.Type)
			if typ == "" {
				typ = defaultType
			}
			out = append(out, inventory.Tag{
				ID:            uid,
				Name:          strings.TrimSpace(e.Name),
				Type:          typ,
				AffiliatedTag: strings.TrimSpace(string(e.AffiliatedTag)),
			})
		}
	}
	appendMap(groups, "group")
	appendMap(users, "user")
	appendMap(devices, "device")
	return out
}

func parseStateDNSResolvers(events map[string]json.RawMessage) []inventory.DNSResolver {
	if len(events) == 0 {
		return nil
	}
	keys := make([]string, 0, len(events))
	for k := range events {
		if strings.HasPrefix(k, "dns:") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([]inventory.DNSResolver, 0, len(keys))
	for _, k := range keys {
		var ev rawStateEvent
		if json.Unmarshal(events[k], &ev) != nil {
			continue
		}
		server := strings.TrimPrefix(k, "dns:")
		wan := ""
		if ev.Labels != nil {
			if n := strings.TrimSpace(ev.Labels.NameServer); n != "" {
				server = n
			}
			wan = strings.TrimSpace(ev.Labels.WanIntfName)
		}
		ok := true
		if ev.StateValue != nil {
			// Match agent WAN/DNS convention: state_value 0 == healthy.
			ok = float64(*ev.StateValue) == 0
		}
		out = append(out, inventory.DNSResolver{Server: server, WAN: wan, OK: ok})
	}
	return out
}

func parseStateWAN(events map[string]json.RawMessage) map[string]snapshot.WANLink {
	if len(events) == 0 {
		return nil
	}
	out := map[string]snapshot.WANLink{}
	for k, raw := range events {
		if !strings.HasPrefix(k, "wan_state:") {
			continue
		}
		iface := strings.TrimPrefix(k, "wan_state:")
		if iface == "" || strings.ContainsAny(iface, ":.") {
			continue
		}
		var ev rawStateEvent
		if json.Unmarshal(raw, &ev) != nil {
			continue
		}
		link := snapshot.WANLink{Name: iface}
		if ev.Labels != nil {
			if n := strings.TrimSpace(ev.Labels.WanIntfName); n != "" {
				link.Name = n
			} else if n := strings.TrimSpace(ev.Labels.Name); n != "" {
				link.Name = n
			}
			if ev.Labels.Ready != nil {
				link.Ready = *ev.Labels.Ready
			}
			if ev.Labels.Active != nil {
				link.Active = *ev.Labels.Active
			}
		}
		// Prefer state_value when present (0 = ready), matching agent parseWANFromEventCache.
		if ev.StateValue != nil {
			link.Ready = float64(*ev.StateValue) == 0
		}
		out[iface] = link
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseTransferWindow(src map[string]json.RawMessage) inventory.Transfer {
	if len(src) == 0 {
		return inventory.Transfer{}
	}
	var totalUp, totalDown flexFloat
	var upload, download, conn [][]flexFloat
	_ = json.Unmarshal(src["totalUpload"], &totalUp)
	_ = json.Unmarshal(src["totalDownload"], &totalDown)
	_ = json.Unmarshal(src["upload"], &upload)
	_ = json.Unmarshal(src["download"], &download)
	_ = json.Unmarshal(src["conn"], &conn)
	raw := &rawNewLast24{
		TotalUpload:   totalUp,
		TotalDownload: totalDown,
		Upload:        upload,
		Download:      download,
		Conn:          conn,
	}
	xfer, _, _ := parseNewLast24(raw)
	return xfer
}

func parseNICMetrics(src map[string]rawNICPercentiles) []InitNICMetric {
	if len(src) == 0 {
		return nil
	}
	names := make([]string, 0, len(src))
	for n := range src {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]InitNICMetric, 0, len(names))
	for _, name := range names {
		m := src[name]
		row := InitNICMetric{Name: name}
		if m.Rx != nil {
			row.RxMedian = parseByteString(string(m.Rx.Median))
			row.RxPt90 = parseByteString(string(m.Rx.Pt90))
		}
		if m.Tx != nil {
			row.TxMedian = parseByteString(string(m.Tx.Median))
			row.TxPt90 = parseByteString(string(m.Tx.Pt90))
		}
		out = append(out, row)
	}
	return out
}

func parseByteString(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		f, err2 := strconv.ParseFloat(s, 64)
		if err2 != nil {
			return 0
		}
		return int64(f)
	}
	return n
}

func parseWGPeers(raw []rawWGPeer) []InitWGPeer {
	if len(raw) == 0 {
		return nil
	}
	out := make([]InitWGPeer, 0, len(raw))
	for _, p := range raw {
		out = append(out, InitWGPeer{
			Name:       strings.TrimSpace(p.Name),
			PublicKey:  strings.TrimSpace(p.PublicKey),
			Intf:       strings.TrimSpace(p.Intf),
			RxBytes:    int64(p.RxBytes),
			TxBytes:    int64(p.TxBytes),
			LastActive: float64(p.LastActiveTimestamp),
		})
	}
	return out
}

func parseWGClients(raw []rawWGClient) []InitWGClient {
	if len(raw) == 0 {
		return nil
	}
	out := make([]InitWGClient, 0, len(raw))
	for _, p := range raw {
		out = append(out, InitWGClient{
			ProfileID: strings.TrimSpace(p.ProfileID),
			Status:    strings.TrimSpace(string(p.Status)),
			RemoteIP:  strings.TrimSpace(p.RemoteIP),
			LocalIP:   strings.TrimSpace(p.LocalIP),
		})
	}
	return out
}

func parseVIPs(raw []rawVIP) []InitVIP {
	if len(raw) == 0 {
		return nil
	}
	out := make([]InitVIP, 0, len(raw))
	for _, p := range raw {
		out = append(out, InitVIP{
			UID:  strings.TrimSpace(p.UID),
			Name: strings.TrimSpace(p.Name),
			IP:   strings.TrimSpace(p.IP),
		})
	}
	return out
}

func parseVirtWANs(raw []rawVirtWAN) []InitVirtWAN {
	if len(raw) == 0 {
		return nil
	}
	out := make([]InitVirtWAN, 0, len(raw))
	for _, p := range raw {
		row := InitVirtWAN{
			UUID:      strings.TrimSpace(p.UUID),
			Name:      strings.TrimSpace(p.Name),
			Type:      strings.TrimSpace(p.Type),
			ConnState: virtWANConnSummary(p.ConnState),
		}
		var asStrings []string
		if json.Unmarshal(p.WANs, &asStrings) == nil {
			row.WANs = asStrings
		} else {
			var asObjs []map[string]any
			if json.Unmarshal(p.WANs, &asObjs) == nil {
				for _, o := range asObjs {
					if id, ok := o["profileId"].(string); ok && id != "" {
						row.WANs = append(row.WANs, id)
						continue
					}
					if id, ok := o["uuid"].(string); ok && id != "" {
						row.WANs = append(row.WANs, id)
					}
				}
			}
		}
		out = append(out, row)
	}
	return out
}

func virtWANConnSummary(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var asString string
	if json.Unmarshal(raw, &asString) == nil {
		return strings.TrimSpace(asString)
	}
	var m map[string]map[string]any
	if json.Unmarshal(raw, &m) != nil || len(m) == 0 {
		return ""
	}
	active, enabled := 0, 0
	for _, st := range m {
		if v, ok := st["enabled"].(bool); ok && v {
			enabled++
		}
		if v, ok := st["active"].(bool); ok && v {
			active++
		}
	}
	return strconv.Itoa(active) + "/" + strconv.Itoa(enabled) + " active"
}

func flexIDList(src []flexString) []string {
	if len(src) == 0 {
		return nil
	}
	out := make([]string, 0, len(src))
	for _, t := range src {
		s := strings.TrimSpace(string(t))
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}
