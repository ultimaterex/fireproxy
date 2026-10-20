package store

import (
	"testing"

	"fireproxy/pkg/inventory"
)

func TestMergeSpeedtest(t *testing.T) {
	cs := NewCatalogStore()
	cs.Set(inventory.Catalog{
		Dashboard: &inventory.Dashboard{
			Speedtest: []inventory.SpeedtestWAN{{
				UUID: "wan-a",
				Name: "Telesur",
				Down: 600,
				Up:   400,
				Points: []inventory.SpeedtestPoint{
					{TS: 100, Down: 600, Up: 400, Ping: 12},
				},
			}},
		},
	})
	cs.MergeSpeedtest([]inventory.SpeedtestPoint{
		{TS: 100, Down: 600, Up: 400, Ping: 12}, // dedupe
		{TS: 200, Down: 619, Up: 410, Ping: 11},
	}, "wan-a")
	cat, ok := cs.Get()
	if !ok {
		t.Fatal("no catalog")
	}
	row := cat.Dashboard.Speedtest[0]
	if len(row.Points) != 2 {
		t.Fatalf("points %+v", row.Points)
	}
	if row.Down != 619 || row.Up != 410 || row.Ping != 11 {
		t.Fatalf("latest %+v", row)
	}
	if row.Name != "Telesur" {
		t.Fatalf("name %q", row.Name)
	}
}

func TestMergeSpeedtestPreservesServer(t *testing.T) {
	cs := NewCatalogStore()
	cs.MergeSpeedtest([]inventory.SpeedtestPoint{
		{TS: 100, Down: 10, Up: 2, ServerID: "1", Server: "Telesur", Location: "Balona"},
	}, "wan-a")
	cs.MergeSpeedtest([]inventory.SpeedtestPoint{
		{TS: 100, Down: 11, Up: 3}, // same ts, no server — keep prior labels
	}, "wan-a")
	cat, _ := cs.Get()
	p := cat.Dashboard.Speedtest[0].Points[0]
	if p.Down != 11 || p.Server != "Telesur" || p.ServerID != "1" {
		t.Fatalf("%+v", p)
	}
}
