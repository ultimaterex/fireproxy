package auth

import (
	"testing"
	"time"
)

func TestAuthDisabledTrueForTrueAndOne(t *testing.T) {
	for _, v := range []string{"true", "1"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv("AUTH_DISABLED", v)
			t.Setenv("AUTH_PASSWORD", "")
			cfg, err := LoadFromEnv()
			if err != nil {
				t.Fatal(err)
			}
			if !cfg.AuthDisabled() {
				t.Fatalf("AuthDisabled() = false for AUTH_DISABLED=%q", v)
			}
			if !cfg.Disabled {
				t.Fatalf("Disabled = false for AUTH_DISABLED=%q", v)
			}
		})
	}
}

func TestLoadRequiresPasswordWhenAuthEnabled(t *testing.T) {
	t.Setenv("AUTH_DISABLED", "")
	t.Setenv("AUTH_PASSWORD", "")
	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected error when auth enabled and AUTH_PASSWORD empty")
	}
}

func TestLoadSucceedsWithPassword(t *testing.T) {
	t.Setenv("AUTH_DISABLED", "")
	t.Setenv("AUTH_PASSWORD", "secret")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthDisabled() {
		t.Fatal("expected auth enabled")
	}
	if cfg.Password != "secret" {
		t.Fatalf("Password = %q", cfg.Password)
	}
	if cfg.SessionAbs != 7*24*time.Hour {
		t.Fatalf("SessionAbs = %v", cfg.SessionAbs)
	}
	if cfg.SessionIdle != 24*time.Hour {
		t.Fatalf("SessionIdle = %v", cfg.SessionIdle)
	}
}

func TestLoadIgnoresDevAgentTokenWhenAuthEnabled(t *testing.T) {
	t.Setenv("AUTH_DISABLED", "")
	t.Setenv("AUTH_PASSWORD", "secret")
	t.Setenv("AUTH_DEV_AGENT_TOKEN", "devtok")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DevAgentToken != "" {
		t.Fatalf("DevAgentToken = %q, want empty when auth enabled", cfg.DevAgentToken)
	}
}

func TestLoadOptionalEnv(t *testing.T) {
	t.Setenv("AUTH_DISABLED", "1")
	t.Setenv("AUTH_PASSWORD", "")
	t.Setenv("AUTH_DEV_AGENT_TOKEN", "devtok")
	t.Setenv("AUTH_TRUSTED_PROXY", "true")
	t.Setenv("AUTH_SESSION_ABS", "48h")
	t.Setenv("AUTH_SESSION_IDLE", "2h")
	t.Setenv("AUTH_PUBLIC_ORIGIN", "https://fp.example")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DevAgentToken != "devtok" {
		t.Fatalf("DevAgentToken = %q", cfg.DevAgentToken)
	}
	if !cfg.TrustedProxy {
		t.Fatal("TrustedProxy = false")
	}
	if cfg.SessionAbs != 48*time.Hour {
		t.Fatalf("SessionAbs = %v", cfg.SessionAbs)
	}
	if cfg.SessionIdle != 2*time.Hour {
		t.Fatalf("SessionIdle = %v", cfg.SessionIdle)
	}
	if cfg.PublicOrigin != "https://fp.example" {
		t.Fatalf("PublicOrigin = %q", cfg.PublicOrigin)
	}
}
