package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is fireproxy-server runtime configuration.
type Config struct {
	ListenAddr        string
	StoreSize         int
	DataPath          string
	RetentionDays     int
	GeoIPMMDB         string
	MaxMindAccountID  string
	MaxMindLicenseKey string
	GeoIPUpdateEvery  time.Duration
	ModuleEnable      map[string]bool
	UnifiBaseURL      string
	UnifiAPIKey       string
	AgentBinDir       string
	InstallScriptPath string
}

// LoadFromEnv loads server config.
func LoadFromEnv() (Config, error) {
	cfg := Config{
		ListenAddr:        envOr("LISTEN_ADDR", ":8080"),
		StoreSize:         envInt("STORE_SIZE", 64),
		DataPath:          strings.TrimSpace(os.Getenv("DATA_PATH")),
		RetentionDays:     envInt("RETENTION_DAYS", 90),
		GeoIPMMDB:         strings.TrimSpace(os.Getenv("GEOIP_MMDB")),
		MaxMindAccountID:  strings.TrimSpace(os.Getenv("MAXMIND_ACCOUNT_ID")),
		MaxMindLicenseKey: strings.TrimSpace(os.Getenv("MAXMIND_LICENSE_KEY")),
		GeoIPUpdateEvery:  envDuration("GEOIP_UPDATE_EVERY", 12*time.Hour),
		ModuleEnable:      map[string]bool{},
		UnifiBaseURL:      envOr("UNIFI_BASE_URL", "https://unifi.lan:11443"),
		UnifiAPIKey:       strings.TrimSpace(os.Getenv("UNIFI_API_KEY")),
		AgentBinDir:       envOr("AGENT_BIN_DIR", "/agent"),
		InstallScriptPath: envOr("INSTALL_SCRIPT_PATH", "/agent/install-agent.sh"),
	}
	if cfg.GeoIPUpdateEvery < time.Hour {
		cfg.GeoIPUpdateEvery = time.Hour
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 90
	}
	if cfg.StoreSize < 2 {
		cfg.StoreSize = 2
	}
	// MODULES=unifi-sync,ha-mqtt  (comma-separated enable list)
	for _, name := range strings.Split(os.Getenv("MODULES"), ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			cfg.ModuleEnable[name] = true
		}
	}
	return cfg, nil
}

func envOr(k, fb string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return fb
}

func envDuration(k string, fb time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return fb
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fb
	}
	return d
}

func envInt(k string, fb int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return fb
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fb
	}
	return n
}
