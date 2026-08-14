package collect

import (
	"encoding/json"
	"strings"
	"testing"

	"fireproxy/pkg/inventory"
)

func TestParseWifiHostapdStripsSecrets(t *testing.T) {
	active := []byte(`{
	  "hostapd": {
	    "wlan1": {
	      "bridge": "br1",
	      "params": {
	        "ssid": "VEGA",
	        "channel": 6,
	        "hw_mode": "g",
	        "wpa_passphrase": "hunter2",
	        "wpa_psk": "deadbeef",
	        "sae_password": "nope"
	      }
	    }
	  }
	}`)
	ifaces := []byte(`{
	  "wlan1": {
	    "state": {
	      "mac": "02:00:00:00:00:50",
	      "essid": "VEGA",
	      "channel": 6,
	      "ip4": "203.0.113.1/24"
	    }
	  }
	}`)
	net := []inventory.NetworkIface{{
		Name: "br1", Desc: "VEGA LAN", UUID: "vega-uuid", Intfs: []string{"wlan1"},
		IP: "203.0.113.1",
	}}
	got := ParseWifi(ifaces, active, net, "")
	if len(got) != 1 {
		t.Fatalf("radios: %+v", got)
	}
	r := got[0]
	if r.Iface != "wlan1" || r.SSID != "VEGA" || r.Channel != 6 || r.Band != "2.4" {
		t.Fatalf("radio: %+v", r)
	}
	if r.MAC != "02:00:00:00:00:50" || r.LAN != "VEGA LAN" || r.LANUUID != "vega-uuid" {
		t.Fatalf("join: %+v", r)
	}
	blob, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(blob), "hunter") || strings.Contains(string(blob), "deadbeef") || strings.Contains(string(blob), "nope") {
		t.Fatalf("leaked secret: %s", blob)
	}
}

func TestParseWifiSkipsSTAAndScan(t *testing.T) {
	ifaces := []byte(`{
	  "wlan0": { "state": { "mac": "aa:aa:aa:aa:aa:aa" } },
	  "wlan_ap_scan": { "state": { "essid": "x", "channel": 1 } }
	}`)
	active := []byte(`{"hostapd":{}}`)
	if got := ParseWifi(ifaces, active, nil, ""); len(got) != 0 {
		t.Fatalf("want empty got %+v", got)
	}
}
