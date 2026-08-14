package tplink

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func testdata(name string) string {
	_, file, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "testdata", name))
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestParseSystemInfo(t *testing.T) {
	info, err := ParseSystemInfo(testdata("system_info.html"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Model != "TL-SG1016PE" || info.MAC != "AA:BB:CC:DD:EE:10" || info.IP != "203.0.113.10" {
		t.Fatalf("%+v", info)
	}
}

func TestParseSystemInfoOpaqueDescri(t *testing.T) {
	html := `<script>var info_ds = {
descriStr:["` + "\xbf\x01\x9ct" + `"],
macStr:["AA:BB:CC:DD:EE:10"],
ipStr:["203.0.113.10"],
firmwareStr:["1.0.0 Build 20230219 Rel.70930"],
hardwareStr:["TL-SG1016PE 3.0"]
};</script>`
	info, err := ParseSystemInfo(html)
	if err != nil {
		t.Fatal(err)
	}
	if info.Model != "TL-SG1016PE 3.0" {
		t.Fatalf("want hardware fallback, got %+v", info)
	}
}

func TestParsePortStatistics(t *testing.T) {
	ports, err := ParsePortStatistics(testdata("port_statistics.html"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 16 {
		t.Fatalf("ports %d", len(ports))
	}
	if !ports[0].Up || ports[0].SpeedMbps != 1000 {
		t.Fatalf("port1 %+v", ports[0])
	}
	if ports[1].Up {
		t.Fatalf("port2 should be down")
	}
	if !ports[15].Up || ports[15].SpeedMbps != 1000 {
		t.Fatalf("port16 %+v", ports[15])
	}
}

func TestParsePoeConfig(t *testing.T) {
	poe, err := ParsePoeConfig(testdata("poe_config.html"))
	if err != nil {
		t.Fatal(err)
	}
	if poe.BudgetW != 150 || poe.DrawW != 28.9 {
		t.Fatalf("budget/draw %+v", poe)
	}
	if len(poe.Ports) != 8 {
		t.Fatalf("poe ports %d", len(poe.Ports))
	}
	if !poe.Ports[0].Delivering || poe.Ports[0].PowerW != 5.7 {
		t.Fatalf("poe1 %+v", poe.Ports[0])
	}
}
