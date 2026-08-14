package config

import (
	"testing"
	"time"
)

func TestLoadWithoutIngestToken(t *testing.T) {
	t.Setenv("INGEST_TOKEN", "")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr == "" {
		t.Fatal("want default listen addr")
	}
}

func TestGeoUpdateIntervalFloor(t *testing.T) {
	t.Setenv("GEOIP_UPDATE_EVERY", "5m")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GeoIPUpdateEvery != time.Hour {
		t.Fatalf("got %v", cfg.GeoIPUpdateEvery)
	}
}

func TestMaxMindEnv(t *testing.T) {
	t.Setenv("MAXMIND_ACCOUNT_ID", "99")
	t.Setenv("MAXMIND_LICENSE_KEY", "abc")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxMindAccountID != "99" || cfg.MaxMindLicenseKey != "abc" {
		t.Fatalf("%+v", cfg)
	}
}
