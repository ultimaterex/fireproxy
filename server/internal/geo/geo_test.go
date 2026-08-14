package geo

import (
	"testing"

	"fireproxy/pkg/inventory"
)

type mapLookup map[string]string

func (m mapLookup) Country(ip string) string { return m[ip] }

func TestEnrichBuildsRegionDevicesFromDestFlows(t *testing.T) {
	d := &inventory.Dashboard{
		DestFlows: []inventory.RankedFlow{
			{
				ID: "9.9.9.9", Name: "hf.co", DestIP: "9.9.9.9", Download: 500, Bytes: 500,
				Devices: []inventory.RankedFlow{{ID: "AA:BB:CC:DD:EE:01", Name: "aurora", Type: "nas", Download: 500, Bytes: 500}},
			},
			{
				ID: "1.2.3.4", Name: "1.2.3.4", DestIP: "1.2.3.4", Country: "NL", Upload: 1000, Bytes: 1000,
				Devices: []inventory.RankedFlow{{ID: "AA:BB:CC:DD:EE:02", Name: "laptop", Upload: 1000, Bytes: 1000}},
			},
		},
	}
	Enrich(d, mapLookup{"9.9.9.9": "US"})
	var us, nl *inventory.RankedFlow
	for i := range d.TopRegions {
		switch d.TopRegions[i].ID {
		case "US":
			us = &d.TopRegions[i]
		case "NL":
			nl = &d.TopRegions[i]
		}
	}
	if us == nil || len(us.Devices) != 1 || us.Devices[0].Name != "aurora" {
		t.Fatalf("US devices %+v", us)
	}
	if len(us.Targets) != 1 || us.Targets[0].Name != "hf.co" {
		t.Fatalf("US targets %+v", us)
	}
	if nl == nil || len(nl.Devices) != 1 || nl.Devices[0].Name != "laptop" {
		t.Fatalf("NL devices %+v", nl)
	}
}

func TestEnrichPreservesRegionDevicesAndTargets(t *testing.T) {
	d := &inventory.Dashboard{
		DestFlows: []inventory.RankedFlow{
			{ID: "1.2.3.4", Name: "1.2.3.4", DestIP: "1.2.3.4", Country: "NL", Upload: 1000, Bytes: 1000},
		},
		TopRegions: []inventory.RankedFlow{{
			ID: "NL", Name: "Netherlands", Country: "NL", Bytes: 1000,
			Devices: []inventory.RankedFlow{{ID: "AA:BB:CC:DD:EE:01", Name: "aurora", Upload: 1000, Bytes: 1000}},
			Targets: []inventory.RankedFlow{{ID: "1.2.3.4", Name: "1.2.3.4", Upload: 1000, Bytes: 1000}},
		}},
	}
	Enrich(d, nil)
	if len(d.TopRegions) != 1 || len(d.TopRegions[0].Devices) != 1 || d.TopRegions[0].Devices[0].Name != "aurora" {
		t.Fatalf("devices lost: %+v", d.TopRegions)
	}
	if len(d.TopRegions[0].Targets) != 1 || d.TopRegions[0].Targets[0].Name != "1.2.3.4" {
		t.Fatalf("targets lost: %+v", d.TopRegions[0].Targets)
	}
}

func TestEnrichFillsFromMMDBAndReranks(t *testing.T) {
	d := &inventory.Dashboard{
		DestFlows: []inventory.RankedFlow{
			{ID: "hf.co", Name: "hf.co", DestIP: "9.9.9.9", Upload: 10, Download: 500, Bytes: 510},
			{ID: "1.2.3.4", Name: "1.2.3.4", DestIP: "1.2.3.4", Country: "NL", Upload: 1000, Download: 0, Bytes: 1000},
		},
		TopRegions: []inventory.RankedFlow{{ID: "NL", Name: "Netherlands", Bytes: 1000}},
	}
	Enrich(d, mapLookup{"9.9.9.9": "US", "1.2.3.4": "DE"})
	if d.TopDestUpload[0].Country != "NL" || d.TopDestUpload[0].Region != "Netherlands" {
		t.Fatalf("intel wins: %+v", d.TopDestUpload)
	}
	if len(d.TopDestDownload) == 0 || d.TopDestDownload[0].Country != "US" || d.TopDestDownload[0].Region != "United States" {
		t.Fatalf("mmdb dest: %+v", d.TopDestDownload)
	}
	if len(d.TopRegions) < 2 || d.TopRegions[0].ID != "NL" || d.TopRegions[1].ID != "US" {
		t.Fatalf("regions %+v", d.TopRegions)
	}
}

func TestEnrichJunkCountryFallsBackToMMDB(t *testing.T) {
	d := &inventory.Dashboard{
		DestFlows: []inventory.RankedFlow{
			{ID: "1.1.1.1", Name: "1.1.1.1", DestIP: "1.1.1.1", Country: "[]", Upload: 1, Download: 9, Bytes: 10},
		},
	}
	Enrich(d, mapLookup{"1.1.1.1": "SR"})
	if len(d.TopRegions) != 1 || d.TopRegions[0].ID != "SR" || d.TopRegions[0].Name != "Suriname" {
		t.Fatalf("regions %+v", d.TopRegions)
	}
	if d.TopDestDownload[0].Country != "SR" {
		t.Fatalf("dest %+v", d.TopDestDownload)
	}
}

func TestEnrichLegacyTopsWithoutDestFlows(t *testing.T) {
	d := &inventory.Dashboard{
		TopDestDownload: []inventory.RankedFlow{
			{ID: "8.8.8.8", Name: "8.8.8.8", Bytes: 1, Download: 1},
		},
	}
	Enrich(d, mapLookup{"8.8.8.8": "US"})
	if d.TopDestDownload[0].Country != "US" || d.TopDestDownload[0].Region != "United States" {
		t.Fatalf("%+v", d.TopDestDownload)
	}
}
