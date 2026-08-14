package tplink

import (
	"fmt"
	"strconv"
	"strings"

	"fireproxy/pkg/inventory"
)

// ToSwitch maps scraped pages into an inventory.Switch.
func ToSwitch(info SystemInfo, ports []PortStat, poe *PoeInfo) inventory.Switch {
	sw := inventory.Switch{
		MAC:      info.MAC,
		Name:     info.Model,
		IP:       info.IP,
		Model:    info.Model,
		Firmware: info.Firmware,
		Source:   "tplink",
		Healthy:  true,
		// Node-level clients filled at merge from UniFi parent-port FDB.
		// Per-port MAC table does not exist on Easy Smart HTTP.
		Capabilities: &inventory.SwitchCapabilities{Clients: true, PortClients: false},
	}
	sw.Ports = make([]inventory.SwitchPort, 0, len(ports))
	activePoE := 0
	var peakW float64
	var peakID string
	for _, p := range ports {
		sp := inventory.SwitchPort{
			ID:        strconv.Itoa(p.ID),
			Up:        p.Up,
			SpeedMbps: p.SpeedMbps,
			TxUnicast: p.TxGoodPkt,
			RxUnicast: p.RxGoodPkt,
			TxError:   p.TxBadPkt,
			RxError:   p.RxBadPkt,
		}
		if poe != nil {
			for _, pp := range poe.Ports {
				if pp.ID == p.ID {
					sp.PoEW = pp.PowerW
					if pp.Delivering {
						sp.PoEStatus = "delivering"
						activePoE++
						if pp.PowerW > peakW {
							peakW = pp.PowerW
							peakID = sp.ID
						}
					}
					break
				}
			}
		}
		sw.Ports = append(sw.Ports, sp)
	}
	if poe != nil {
		sw.PoE = &inventory.SwitchPoE{
			DrawW:       poe.DrawW,
			BudgetW:     poe.BudgetW,
			ActivePorts: activePoE,
			PeakPort:    peakID,
		}
	}
	if sw.Name == "" {
		sw.Name = fmt.Sprintf("TP-Link %s", sw.MAC)
	}
	return sw
}

// ApplyUplink sets the local jack facing upstream from persisted config.
func ApplyUplink(sw *inventory.Switch, port string) {
	port = strings.TrimSpace(port)
	if sw == nil || port == "" {
		return
	}
	sw.UplinkPort = port
	sw.UplinkStatic = true
	for i := range sw.Ports {
		sw.Ports[i].Uplink = sw.Ports[i].ID == port
	}
}
