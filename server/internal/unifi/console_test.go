package unifi

import "testing"

func TestApplySysinfoAndHealth(t *testing.T) {
	c := Console{Site: "Default", NetworkVersion: "old"}
	ApplySysinfo(&c, []byte(`{"data":[{
		"name":"Unifi",
		"version":"10.5.67",
		"update_available":false,
		"ip_addrs":["192.0.2.4","127.0.0.1"],
		"inform_port":8080,
		"uptime":3048995
	}]}`))
	if c.Name != "Unifi" || c.NetworkVersion != "10.5.67" || c.NetworkUpdate || c.IP != "192.0.2.4" || c.Inform != "192.0.2.4:8080" || c.UptimeSec != 3048995 {
		t.Fatalf("sysinfo: %+v", c)
	}
	ApplyHealth(&c, []byte(`{"data":[
		{"subsystem":"wlan","num_user":37,"num_guest":0,"num_ap":3,"num_adopted":3},
		{"subsystem":"lan","num_user":29,"num_sw":4,"num_adopted":5}
	]}`))
	if c.APs != 3 || c.Switches != 4 || c.Devices != 5 || c.Clients != 66 {
		t.Fatalf("health: %+v", c)
	}
	ApplyOSSystem(&c, []byte(`{"name":"Unifi","hardware":{"shortname":"UOSSERVER"},"cloudConnected":true}`))
	if c.Hardware != "UniFi OS Server" || !c.Cloud {
		t.Fatalf("os: %+v", c)
	}
}
