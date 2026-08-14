package inventory

import "strings"

// GoldPort maps a Gold/FireRouter NIC name to physical port 1–4.
// VLAN suffixes (eth0.254) set tagged; wlan and unknown names return ok false.
func GoldPort(name string) (n int, tagged bool, ok bool) {
	base := name
	if i := strings.IndexByte(name, '.'); i >= 0 {
		tagged = true
		base = name[:i]
	}
	switch base {
	case "eth3":
		return 1, tagged, true
	case "eth2":
		return 2, tagged, true
	case "eth1":
		return 3, tagged, true
	case "eth0":
		return 4, tagged, true
	default:
		return 0, false, false
	}
}

// ManagerKind is the network-manager tab bucket: lan, wan, or hide.
func ManagerKind(n NetworkIface) string {
	switch {
	case strings.HasPrefix(n.Name, "wg_ap"),
		strings.HasPrefix(n.Name, "tun_fwvpn"):
		return "hide"
	case n.Type == "wan":
		return "wan"
	default:
		return "lan"
	}
}
