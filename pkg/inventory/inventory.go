// Package inventory defines FireProxy catalog payloads (devices, network, policies, tags).
package inventory

import (
	"encoding/json"
	"strconv"

	"fireproxy/pkg/device"
)

// NetworkIface is a LAN/WAN/VPN interface from sys:network:info.
type NetworkIface struct {
	Name   string   `json:"name"`           // e.g. br0, eth1, bond0.10
	Desc   string   `json:"desc,omitempty"` // friendly label e.g. Core, Unified
	Type   string   `json:"type,omitempty"` // lan, wan, …
	UUID   string   `json:"uuid,omitempty"`
	IP     string   `json:"ip,omitempty"`
	Subnet string   `json:"subnet,omitempty"`
	VID    int      `json:"vid,omitempty"`
	Intfs  []string `json:"intfs,omitempty"`
	DHCP   bool     `json:"dhcp,omitempty"`
	Ready  *bool    `json:"wan_ready,omitempty"`
	Active *bool    `json:"wan_active,omitempty"`
}

// IsLogical is a Firewalla network (LAN/WAN/VPN), not a raw NIC/VLAN stub.
func (n NetworkIface) IsLogical() bool {
	return n.Desc != "" || n.Subnet != "" || n.IP != ""
}

// Policy is a condensed Firewalla rule.
type Policy struct {
	ID       string `json:"id"`
	Action   string `json:"action,omitempty"`
	Type     string `json:"type,omitempty"`
	Target   string `json:"target,omitempty"`
	HitCount int64  `json:"hit_count"`
	Disabled bool   `json:"disabled"`
	Notes    string `json:"notes,omitempty"` // _name if present
}

// Tag is a Firewalla group/user/device/ssid tag.
// Type is one of: group, user, device, ssid.
// AffiliatedTag is set on user tags that own a native device group (and vice versa lookup).
type Tag struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type,omitempty"`
	AffiliatedTag string `json:"affiliated_tag,omitempty"`
}

// Device extends the base device with LAN uuid for topology joins.
type Device struct {
	device.Device
	IntfUUID     string       `json:"intf_uuid,omitempty"`
	TagIDs       []string     `json:"tag_ids,omitempty"`        // group tags (policy "tags")
	DeviceTagIDs []string     `json:"device_tag_ids,omitempty"` // device tags
	UserTagIDs   []string     `json:"user_tag_ids,omitempty"`   // user tags
	Upload       int64        `json:"upload,omitempty"`         // last 24h, timedTraffic upload:<mac>
	Download     int64        `json:"download,omitempty"`       // last 24h, timedTraffic download:<mac>
	TopDests     []RankedFlow `json:"top_dests,omitempty"`      // sumflow sample destinations
	DAP          *DAP         `json:"dap,omitempty"`            // policy:mac dap JSON
	Hostname     string       `json:"hostname,omitempty"`       // UniFi sta overlay
	OS           string       `json:"os,omitempty"`
	TxKbps       int          `json:"tx_kbps,omitempty"`
	RxKbps       int          `json:"rx_kbps,omitempty"`
	SSID         string       `json:"ssid,omitempty"`
	Band         string       `json:"band,omitempty"`
	APMAC        string       `json:"ap_mac,omitempty"`
	APName       string       `json:"ap_name,omitempty"`
}

// DAP is Device Active Protect state from policy:mac:<MAC> field "dap".
type DAP struct {
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Learned int64  `json:"learned"`
	Trusted int64  `json:"trusted"`
	Blocked int64  `json:"blocked"`
}

// Catalog is one inventory push from the agent.
type Catalog struct {
	TS        int64          `json:"ts"`
	Host      string         `json:"host"`
	Devices   []Device       `json:"devices"`
	Network   []NetworkIface `json:"network"`
	Policies  []Policy       `json:"policies"`
	Tags      []Tag          `json:"tags"`
	Dashboard *Dashboard     `json:"dashboard,omitempty"`
	Box       *Box           `json:"box,omitempty"`
	Switches  []Switch       `json:"switches,omitempty"`
	Topo      []TopoNode     `json:"topo,omitempty"`
	Wifi      []WifiRadio    `json:"wifi,omitempty"`
}

// WifiRadio is one onboard AP (Gold Wi‑Fi SD, Purple, etc). Never includes PSK.
type WifiRadio struct {
	Iface   string `json:"iface"`
	MAC     string `json:"mac,omitempty"`
	SSID    string `json:"ssid,omitempty"`
	Band    string `json:"band,omitempty"`
	Channel int    `json:"channel,omitempty"`
	Enabled bool   `json:"enabled,omitempty"`
	LAN     string `json:"lan,omitempty"`
	LANUUID string `json:"lan_uuid,omitempty"`
	IP      string `json:"ip,omitempty"`
}

// BoxMAC is a labeled hardware address (Ethernet Port N / Wi-Fi / Bluetooth).
type BoxMAC struct {
	Name string `json:"name"`
	MAC  string `json:"mac"`
}

// BoxMACList accepts both [{name,mac}] and legacy ["aa:bb:..."] catalog JSON.
type BoxMACList []BoxMAC

func (l *BoxMACList) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	var objs []BoxMAC
	if err := json.Unmarshal(b, &objs); err == nil {
		*l = objs
		return nil
	}
	var strs []string
	if err := json.Unmarshal(b, &strs); err != nil {
		return err
	}
	out := make([]BoxMAC, 0, len(strs))
	for i, s := range strs {
		out = append(out, BoxMAC{Name: "Address " + strconv.Itoa(i+1), MAC: s})
	}
	*l = out
	return nil
}

// Box is on-box identity (no MSP / license secrets).
type Box struct {
	Name              string            `json:"name"`
	PublicIP          string            `json:"public_ip,omitempty"`
	PublicIPs         map[string]string `json:"public_ips,omitempty"` // iface → IP (init fallback)
	DDNS              string            `json:"ddns,omitempty"`
	MACs              BoxMACList        `json:"macs,omitempty"`
	Version           string            `json:"version,omitempty"`
	Model             string            `json:"model,omitempty"`
	License           string            `json:"license,omitempty"`
	EID               string            `json:"eid,omitempty"`
	Mode              string            `json:"mode,omitempty"`
	Timezone          string            `json:"timezone,omitempty"`
	Country           string            `json:"country,omitempty"`
	Region            string            `json:"region,omitempty"`
	WanType           string            `json:"wan_type,omitempty"`            // failover | load_balance | single
	LocalDomainSuffix string            `json:"local_domain_suffix,omitempty"` // Redis local:domain:suffix when set
	UptimeSec         *int64            `json:"uptime_sec,omitempty"`          // Firewalla process uptime
	OsUptimeSec       *int64            `json:"os_uptime_sec,omitempty"`
	CloudConnected    *bool             `json:"cloud_connected,omitempty"`
}

// Switch is a managed PoE switch attached to the box.
type Switch struct {
	MAC        string `json:"mac"`
	Name       string `json:"name,omitempty"`
	IP         string `json:"ip,omitempty"`
	Model      string `json:"model,omitempty"`
	Version    string `json:"version,omitempty"`
	Firmware   string `json:"firmware,omitempty"`
	Source     string `json:"source,omitempty"` // "unifi"; omit for fwapc
	Kind       string `json:"kind,omitempty"`   // "ap"; omit for switch
	Healthy    bool   `json:"healthy"`
	UplinkPort string `json:"uplink_port,omitempty"`
	// UplinkStatic is true when uplink_port came from operator config (not discovery).
	UplinkStatic bool                `json:"uplink_static,omitempty"`
	ParentNIC    string              `json:"parent_nic,omitempty"`
	ParentMAC    string              `json:"parent_mac,omitempty"`  // parent switch MAC
	ParentPort   string              `json:"parent_port,omitempty"` // port on that parent
	Ports        []SwitchPort        `json:"ports,omitempty"`
	Lags         [][]string          `json:"lags,omitempty"` // port-id groups
	Radios       []Radio             `json:"radios,omitempty"`
	PoE          *SwitchPoE          `json:"poe,omitempty"`
	ACL          *SwitchACL          `json:"acl,omitempty"`
	Clients      []string            `json:"clients,omitempty"`
	ClientVLANs  map[string][]int    `json:"client_vlans,omitempty"` // uppercase MAC -> vlanIds from Switch X
	Capabilities *SwitchCapabilities `json:"capabilities,omitempty"`
}

// SwitchCapabilities marks features the data source can actually provide.
// Omitted means unknown/legacy (treat as available). Explicit false = unsupported.
type SwitchCapabilities struct {
	Clients     bool `json:"clients"`      // node-level client / MAC list
	PortClients bool `json:"port_clients"` // per-port MAC table
}

// Radio is one AP radio (UniFi).
type Radio struct {
	Band     string `json:"band"` // "2.4" | "5" | "6"
	Channel  int    `json:"channel,omitempty"`
	WidthMHz int    `json:"width_mhz,omitempty"`
}

// SwitchPort is one physical port on a managed switch.
type SwitchPort struct {
	ID          string   `json:"id"`
	Up          bool     `json:"up"`
	SpeedMbps   int      `json:"speed_mbps,omitempty"`
	PoEStatus   string   `json:"poe_status,omitempty"`
	PoEW        float64  `json:"poe_w,omitempty"`
	PoEMode     string   `json:"poe_mode,omitempty"`
	SFP         bool     `json:"sfp,omitempty"`
	Uplink      bool     `json:"uplink,omitempty"`
	Clients     []string `json:"clients,omitempty"`
	RxBytes     int64    `json:"rx_bytes,omitempty"`
	TxBytes     int64    `json:"tx_bytes,omitempty"`
	RxUnicast   int64    `json:"rx_unicast,omitempty"`
	TxUnicast   int64    `json:"tx_unicast,omitempty"`
	RxBroadcast int64    `json:"rx_broadcast,omitempty"`
	TxBroadcast int64    `json:"tx_broadcast,omitempty"`
	RxMulticast int64    `json:"rx_multicast,omitempty"`
	TxMulticast int64    `json:"tx_multicast,omitempty"`
	RxDiscard   int64    `json:"rx_discard,omitempty"`
	TxDiscard   int64    `json:"tx_discard,omitempty"`
	RxError     int64    `json:"rx_error,omitempty"`
	TxError     int64    `json:"tx_error,omitempty"`
}

// SwitchPoE is aggregate PoE draw for a switch.
type SwitchPoE struct {
	DrawW       float64 `json:"draw_w"`
	BudgetW     float64 `json:"budget_w"`
	ActivePorts int     `json:"active_ports"`
	PeakPort    string  `json:"peak_port,omitempty"`
}

// SwitchACL is ACL slot usage on a managed switch.
type SwitchACL struct {
	Used     int `json:"used"`
	Max      int `json:"max"`
	Control  int `json:"control"`
	Tracking int `json:"tracking"`
}

// TopoNode is one node in the box/switch topology tree.
type TopoNode struct {
	MAC         string              `json:"mac"`
	Name        string              `json:"name,omitempty"`
	IP          string              `json:"ip,omitempty"`
	Type        string              `json:"type,omitempty"` // box|switch
	ParentPort  string              `json:"parent_port,omitempty"`
	ChildPort   string              `json:"child_port,omitempty"`
	Clients     []string            `json:"clients,omitempty"`
	PortClients map[string][]string `json:"-"`
	Children    []TopoNode          `json:"children,omitempty"`
}

// Dashboard is MSP-style rollups read from on-box Redis (timedTraffic / sumflow / alarms).
type Dashboard struct {
	AlarmCount      int64        `json:"alarm_count"`
	Transfer24h     Transfer     `json:"transfer_24h"`
	MonthlyWANs     []WANUsage   `json:"monthly_wans,omitempty"`
	Blocked         BlockedMix   `json:"blocked"`
	TopUpload       []RankedFlow `json:"top_upload,omitempty"`
	TopDownload     []RankedFlow `json:"top_download,omitempty"`
	TopDestUpload   []RankedFlow `json:"top_dest_upload,omitempty"`
	TopDestDownload []RankedFlow `json:"top_dest_download,omitempty"`
	TopRegions      []RankedFlow `json:"top_regions,omitempty"`
	// DestFlows is the unranked dest pool (dest_ip + intel country). Server applies MMDB.
	DestFlows []RankedFlow   `json:"dest_flows,omitempty"`
	Speedtest []SpeedtestWAN `json:"speedtest,omitempty"`
	DNS       *DNSHealth     `json:"dns,omitempty"`
}

// SpeedtestWAN is last + history internet speedtest for one ISP WAN.
type SpeedtestWAN struct {
	UUID     string           `json:"uuid"`
	Name     string           `json:"name"`
	Active   bool             `json:"active,omitempty"`
	PlanDown float64          `json:"plan_down,omitempty"` // Mbps
	PlanUp   float64          `json:"plan_up,omitempty"`
	Down     float64          `json:"down,omitempty"`
	Up       float64          `json:"up,omitempty"`
	Ping     float64          `json:"ping,omitempty"`
	Jitter   float64          `json:"jitter,omitempty"`
	ServerID string           `json:"server_id,omitempty"`
	Server   string           `json:"server,omitempty"`
	Location string           `json:"location,omitempty"`
	Points   []SpeedtestPoint `json:"points,omitempty"`
}

// SpeedtestPoint is one scheduled test.
type SpeedtestPoint struct {
	TS       int64   `json:"ts"`
	Down     float64 `json:"down"`
	Up       float64 `json:"up"`
	Ping     float64 `json:"ping,omitempty"`
	ServerID string  `json:"server_id,omitempty"`
	Server   string  `json:"server,omitempty"` // sponsor label
	Location string  `json:"location,omitempty"`
}

// DNSHealth is upstream resolver probes + query volume.
type DNSHealth struct {
	Resolvers []DNSResolver `json:"resolvers,omitempty"`
	Queries   []DNSQuery    `json:"queries,omitempty"`
}

// DNSResolver is one upstream nameserver probe from event:state:cache.
type DNSResolver struct {
	Server string `json:"server"`
	WAN    string `json:"wan,omitempty"`
	OK     bool   `json:"ok"`
}

// DNSQuery is one timeseries slot of DNS lookups.
type DNSQuery struct {
	TS    int64 `json:"ts"`
	Count int64 `json:"count"`
}

// Transfer is upload/download totals plus timeseries points.
type Transfer struct {
	Upload   int64       `json:"upload"`
	Download int64       `json:"download"`
	Points   []BytePoint `json:"points,omitempty"`
}

// BytePoint is one timeseries slot (bytes + connection count).
type BytePoint struct {
	TS       int64 `json:"ts"`
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Conn     int64 `json:"conn,omitempty"`
}

// WANUsage is monthly (billing-cycle) volume for one WAN.
type WANUsage struct {
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}

// BlockedMix is connection vs block counts over the last 24h.
type BlockedMix struct {
	Blocked int64 `json:"blocked"`
	Allowed int64 `json:"allowed"`
}

// RankedFlow is a top device, destination, or region by bytes.
type RankedFlow struct {
	ID       string       `json:"id,omitempty"`
	Name     string       `json:"name"`
	Type     string       `json:"type,omitempty"` // device detect.type
	DestIP   string       `json:"dest_ip,omitempty"`
	Country  string       `json:"country,omitempty"` // ISO, from intel or server MMDB
	Region   string       `json:"region,omitempty"`
	Upload   int64        `json:"upload,omitempty"`
	Download int64        `json:"download,omitempty"`
	Bytes    int64        `json:"bytes"`
	Devices  []RankedFlow `json:"devices,omitempty"` // region → contributing devices
	Targets  []RankedFlow `json:"targets,omitempty"` // region → destinations
}
