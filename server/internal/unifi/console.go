package unifi

import (
	"encoding/json"
	"net"
	"strconv"
	"strings"
)

// Console is UniFi controller identity for Inventory.
type Console struct {
	Name           string `json:"name,omitempty"`
	Hardware       string `json:"hardware,omitempty"`
	IP             string `json:"ip,omitempty"`
	Inform         string `json:"inform,omitempty"`
	UptimeSec      int64  `json:"uptime_sec,omitempty"`
	NetworkVersion string `json:"network_version,omitempty"`
	NetworkUpdate  bool   `json:"network_update,omitempty"`
	Site           string `json:"site,omitempty"`
	Cloud          bool   `json:"cloud,omitempty"`
	APs            int    `json:"aps,omitempty"`
	Switches       int    `json:"switches,omitempty"`
	Devices        int    `json:"devices,omitempty"`
	Clients        int    `json:"clients,omitempty"`
}

type sysinfoPage struct {
	Data []sysinfoRow `json:"data"`
}

type sysinfoRow struct {
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	UpdateAvailable bool     `json:"update_available"`
	IPAddrs         []string `json:"ip_addrs"`
	InformPort      int      `json:"inform_port"`
	Uptime          int64    `json:"uptime"`
}

type healthPage struct {
	Data []healthRow `json:"data"`
}

type healthRow struct {
	Subsystem       string `json:"subsystem"`
	NumAP           int    `json:"num_ap"`
	NumSW           int    `json:"num_sw"`
	NumAdopted      int    `json:"num_adopted"`
	NumUser         int    `json:"num_user"`
	NumGuest        int    `json:"num_guest"`
	NumDisconnected int    `json:"num_disconnected"`
}

type osSystemRow struct {
	Name     string `json:"name"`
	Hardware struct {
		Shortname string `json:"shortname"`
	} `json:"hardware"`
	CloudConnected bool `json:"cloudConnected"`
}

// ApplySysinfo copies classic /stat/sysinfo into Console.
func ApplySysinfo(c *Console, raw []byte) {
	if c == nil || len(raw) == 0 {
		return
	}
	var p sysinfoPage
	if json.Unmarshal(raw, &p) != nil || len(p.Data) == 0 {
		return
	}
	s := p.Data[0]
	if s.Name != "" {
		c.Name = s.Name
	}
	if s.Version != "" {
		c.NetworkVersion = s.Version
	}
	c.NetworkUpdate = s.UpdateAvailable
	c.UptimeSec = s.Uptime
	ip := firstPublicIP(s.IPAddrs)
	if ip != "" {
		c.IP = ip
		if s.InformPort > 0 {
			c.Inform = net.JoinHostPort(ip, strconv.Itoa(s.InformPort))
		}
	}
}

// ApplyHealth copies classic /stat/health counts into Console.
func ApplyHealth(c *Console, raw []byte) {
	if c == nil || len(raw) == 0 {
		return
	}
	var p healthPage
	if json.Unmarshal(raw, &p) != nil {
		return
	}
	clients := 0
	for _, h := range p.Data {
		switch h.Subsystem {
		case "wlan":
			c.APs = h.NumAP
			clients += h.NumUser + h.NumGuest
		case "lan":
			c.Switches = h.NumSW
			c.Devices = h.NumAdopted
			clients += h.NumUser + h.NumGuest
		}
	}
	c.Clients = clients
	if c.Devices == 0 {
		c.Devices = c.APs + c.Switches
	}
}

// ApplyOSSystem copies UniFi OS /api/system into Console.
func ApplyOSSystem(c *Console, raw []byte) {
	if c == nil || len(raw) == 0 {
		return
	}
	var s osSystemRow
	if json.Unmarshal(raw, &s) != nil {
		return
	}
	if c.Name == "" && s.Name != "" {
		c.Name = s.Name
	}
	c.Cloud = s.CloudConnected
	c.Hardware = hardwareLabel(s.Hardware.Shortname)
}

func hardwareLabel(short string) string {
	switch strings.ToUpper(strings.TrimSpace(short)) {
	case "UOSSERVER":
		return "UniFi OS Server"
	case "":
		return ""
	default:
		return short
	}
}

func firstPublicIP(addrs []string) string {
	for _, a := range addrs {
		ip := net.ParseIP(strings.TrimSpace(a))
		if ip == nil || ip.IsLoopback() {
			continue
		}
		return ip.String()
	}
	return ""
}
