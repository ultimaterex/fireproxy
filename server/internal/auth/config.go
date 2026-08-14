package auth

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config is auth-related runtime configuration from the environment.
type Config struct {
	Disabled      bool
	Password      string // AUTH_PASSWORD
	DevAgentToken string
	TrustedProxy  bool
	SessionAbs    time.Duration // default 7d
	SessionIdle   time.Duration // default 24h
	PublicOrigin  string        // optional
}

// AuthDisabled reports whether auth gating is opted out.
func (c Config) AuthDisabled() bool {
	return c.Disabled
}

// LoadFromEnv loads auth config.
// When auth is enabled (default), AUTH_PASSWORD must be non-empty.
func LoadFromEnv() (Config, error) {
	cfg := Config{
		Disabled:      envTruthy("AUTH_DISABLED"),
		Password:      strings.TrimSpace(os.Getenv("AUTH_PASSWORD")),
		DevAgentToken: strings.TrimSpace(os.Getenv("AUTH_DEV_AGENT_TOKEN")),
		TrustedProxy:  envTruthy("AUTH_TRUSTED_PROXY"),
		SessionAbs:    envDuration("AUTH_SESSION_ABS", 7*24*time.Hour),
		SessionIdle:   envDuration("AUTH_SESSION_IDLE", 24*time.Hour),
		PublicOrigin:  strings.TrimSpace(os.Getenv("AUTH_PUBLIC_ORIGIN")),
	}
	if !cfg.Disabled {
		cfg.DevAgentToken = "" // AUTH_DEV_AGENT_TOKEN only when AUTH_DISABLED
		if cfg.Password == "" {
			return cfg, fmt.Errorf("AUTH_PASSWORD is required when auth is enabled")
		}
	}
	return cfg, nil
}

func envTruthy(k string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(k)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
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
