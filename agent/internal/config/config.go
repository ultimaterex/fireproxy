package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultInterval          = 60 * time.Second
	MinInterval              = 30 * time.Second
	DefaultHTTPTimeout       = 5 * time.Second
	DefaultRedisAddr         = "127.0.0.1:6379"
	DefaultACLLog            = "/alog/dnsmasq-acl.log"
	DefaultInventoryInterval = time.Hour
	DefaultFwapcURL          = "http://127.0.0.1:8841/v1"
	DefaultFireRouterURL     = "http://127.0.0.1:8837/v1"
)

// Config is fireproxy-agent runtime configuration (env-driven).
type Config struct {
	PushURL           string
	CatalogURL        string
	WSURL             string
	Interval          time.Duration
	InventoryInterval time.Duration
	LogSyncInterval   time.Duration
	LogSyncStagger    time.Duration
	LogCursorPath     string
	Ifaces            []string
	RedisAddr         string
	ACLLogPath        string
	Hostname          string
	HTTPTimeout       time.Duration
	AgentToken        string
	SysfsRoot         string
	ProcLoadPath      string
	CPUOnlinePath     string
	ProcStatPath      string
	CollectCPU        bool
	FwapcURL          string
	FireRouterURL     string
	SelfUpdate        bool
	Version           string
}

const (
	DefaultLogSyncInterval = time.Hour
	DefaultLogSyncStagger  = 15 * time.Minute
	DefaultLogCursorPath   = "/home/pi/.fireproxy/log-cursors.json"
)

// LoadFromEnv reads agent config from environment variables.
func LoadFromEnv() (Config, error) {
	cfg := Config{
		PushURL:       strings.TrimSpace(os.Getenv("PUSH_URL")),
		RedisAddr:     envOr("REDIS_ADDR", DefaultRedisAddr),
		ACLLogPath:    envOr("DNSMASQ_ACL_LOG", DefaultACLLog),
		Hostname:      strings.TrimSpace(os.Getenv("HOSTNAME")),
		SysfsRoot:     envOr("SYSFS_ROOT", "/sys"),
		ProcLoadPath:  envOr("PROC_LOADAVG", "/proc/loadavg"),
		CPUOnlinePath: envOr("CPU_ONLINE", "/sys/devices/system/cpu/online"),
		ProcStatPath:  envOr("PROC_STAT", "/proc/stat"),
		CollectCPU:    envBool("COLLECT_CPU", true),
		AgentToken:    strings.TrimSpace(os.Getenv("AGENT_TOKEN")),
		FwapcURL:      envOr("FWAPC_URL", DefaultFwapcURL),
		FireRouterURL: envOr("FIREROUTER_URL", DefaultFireRouterURL),
		SelfUpdate:    envBool("AGENT_SELF_UPDATE", false),
	}

	if cfg.PushURL == "" {
		return cfg, fmt.Errorf("PUSH_URL is required")
	}

	cfg.CatalogURL = strings.TrimSpace(os.Getenv("CATALOG_URL"))
	if cfg.CatalogURL == "" {
		if strings.HasSuffix(cfg.PushURL, "/v1/ingest") {
			cfg.CatalogURL = strings.TrimSuffix(cfg.PushURL, "/v1/ingest") + "/v1/catalog"
		} else {
			return cfg, fmt.Errorf("CATALOG_URL is required when PUSH_URL does not end with /v1/ingest")
		}
	}

	cfg.WSURL = strings.TrimSpace(os.Getenv("AGENT_WS_URL"))
	if cfg.WSURL == "" {
		if strings.HasSuffix(cfg.PushURL, "/v1/ingest") {
			base := strings.TrimSuffix(cfg.PushURL, "/v1/ingest")
			cfg.WSURL = httpToWS(base + "/v1/agent/ws")
		} else {
			return cfg, fmt.Errorf("AGENT_WS_URL is required when PUSH_URL does not end with /v1/ingest")
		}
	} else {
		cfg.WSURL = httpToWS(cfg.WSURL)
	}

	cfg.LogCursorPath = envOr("LOG_CURSOR_PATH", DefaultLogCursorPath)
	cfg.LogSyncInterval = DefaultLogSyncInterval
	if v := strings.TrimSpace(os.Getenv("LOG_SYNC_INTERVAL")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("LOG_SYNC_INTERVAL: %w", err)
		}
		cfg.LogSyncInterval = d
	}
	if cfg.LogSyncInterval < time.Minute {
		cfg.LogSyncInterval = time.Minute
	}
	cfg.LogSyncStagger = DefaultLogSyncStagger
	if v := strings.TrimSpace(os.Getenv("LOG_SYNC_STAGGER")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("LOG_SYNC_STAGGER: %w", err)
		}
		cfg.LogSyncStagger = d
	}

	interval := DefaultInterval
	if v := strings.TrimSpace(os.Getenv("INTERVAL")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("INTERVAL: %w", err)
		}
		interval = d
	}
	if interval < MinInterval {
		interval = MinInterval
	}
	cfg.Interval = interval

	invInterval := DefaultInventoryInterval
	if v := strings.TrimSpace(os.Getenv("INVENTORY_INTERVAL")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("INVENTORY_INTERVAL: %w", err)
		}
		invInterval = d
	}
	if invInterval < time.Minute {
		invInterval = time.Minute
	}
	cfg.InventoryInterval = invInterval

	timeout := DefaultHTTPTimeout
	if v := strings.TrimSpace(os.Getenv("HTTP_TIMEOUT")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("HTTP_TIMEOUT: %w", err)
		}
		timeout = d
	}
	cfg.HTTPTimeout = timeout

	ifaces := envOr("IFACES", "eth1,eth2,eth3,br0,br2,br3,br4,wg0,wg_ap")
	for _, p := range strings.Split(ifaces, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			cfg.Ifaces = append(cfg.Ifaces, p)
		}
	}
	if len(cfg.Ifaces) == 0 {
		return cfg, fmt.Errorf("IFACES is empty")
	}

	if cfg.Hostname == "" {
		if h, err := os.Hostname(); err == nil {
			cfg.Hostname = h
		} else {
			cfg.Hostname = "firewalla"
		}
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func httpToWS(u string) string {
	switch {
	case strings.HasPrefix(u, "https://"):
		return "wss://" + strings.TrimPrefix(u, "https://")
	case strings.HasPrefix(u, "http://"):
		return "ws://" + strings.TrimPrefix(u, "http://")
	default:
		return u
	}
}
