package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadFromEnvDefaults(t *testing.T) {
	t.Setenv("PUSH_URL", "http://127.0.0.1:8080/v1/ingest")
	t.Setenv("INTERVAL", "")
	t.Setenv("IFACES", "eth1,br2")
	t.Setenv("HOSTNAME", "Firewalla")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interval != DefaultInterval {
		t.Fatalf("interval: %v", cfg.Interval)
	}
	if len(cfg.Ifaces) != 2 || cfg.Ifaces[0] != "eth1" {
		t.Fatalf("ifaces: %v", cfg.Ifaces)
	}
	if !cfg.CollectCPU {
		t.Fatal("COLLECT_CPU should default on")
	}
	if cfg.FwapcURL != "http://127.0.0.1:8841/v1" {
		t.Fatalf("fwapc url: %s", cfg.FwapcURL)
	}
	if cfg.FireRouterURL != "http://127.0.0.1:8837/v1" {
		t.Fatalf("firerouter url: %s", cfg.FireRouterURL)
	}
	if cfg.WSURL != "ws://127.0.0.1:8080/v1/agent/ws" {
		t.Fatalf("ws url: %s", cfg.WSURL)
	}
	if cfg.LogSyncInterval != DefaultLogSyncInterval {
		t.Fatalf("log sync: %v", cfg.LogSyncInterval)
	}
}

func TestLoadFromEnvFloorInterval(t *testing.T) {
	t.Setenv("PUSH_URL", "http://example/v1/ingest")
	t.Setenv("INTERVAL", "5s")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interval != MinInterval {
		t.Fatalf("want floor %v got %v", MinInterval, cfg.Interval)
	}
	if cfg.CatalogURL != "http://example/v1/catalog" {
		t.Fatalf("catalog url: %s", cfg.CatalogURL)
	}
}

func TestLoadFromEnvRequiresPushURL(t *testing.T) {
	_ = os.Unsetenv("PUSH_URL")
	t.Setenv("PUSH_URL", "")
	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseCustomTimeout(t *testing.T) {
	t.Setenv("PUSH_URL", "http://example/v1/ingest")
	t.Setenv("HTTP_TIMEOUT", "3s")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPTimeout != 3*time.Second {
		t.Fatalf("timeout: %v", cfg.HTTPTimeout)
	}
}
