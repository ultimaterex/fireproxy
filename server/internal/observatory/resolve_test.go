package observatory

import (
	"testing"
	"time"
)

func TestPick(t *testing.T) {
	initAt := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	ttl := time.Hour

	tests := []struct {
		name        string
		agentOnline bool
		agentAge    time.Duration
		agentTTL    time.Duration
		initOK      bool
		initAt      time.Time
		wantSource  string
		wantUseInit bool
		wantStale   bool
		wantFetched time.Time
	}{
		{
			name:        "online_fresh_uses_agent",
			agentOnline: true,
			agentAge:    10 * time.Minute,
			agentTTL:    ttl,
			initOK:      true,
			initAt:      initAt,
			wantSource:  SourceAgent,
			wantUseInit: false,
			wantStale:   false,
		},
		{
			name:        "online_at_ttl_boundary_uses_agent",
			agentOnline: true,
			agentAge:    ttl - time.Nanosecond,
			agentTTL:    ttl,
			initOK:      true,
			initAt:      initAt,
			wantSource:  SourceAgent,
			wantUseInit: false,
			wantStale:   false,
		},
		{
			name:        "online_stale_warm_init_uses_init",
			agentOnline: true,
			agentAge:    ttl,
			agentTTL:    ttl,
			initOK:      true,
			initAt:      initAt,
			wantSource:  SourceFWAppInit,
			wantUseInit: true,
			wantStale:   false,
			wantFetched: initAt,
		},
		{
			name:        "online_stale_cold_init_empty",
			agentOnline: true,
			agentAge:    2 * ttl,
			agentTTL:    ttl,
			initOK:      false,
			wantSource:  SourceEmpty,
			wantUseInit: false,
			wantStale:   false,
		},
		{
			name:        "offline_warm_init_even_if_catalog_recent",
			agentOnline: false,
			agentAge:    time.Minute, // recent catalog must not win while offline
			agentTTL:    ttl,
			initOK:      true,
			initAt:      initAt,
			wantSource:  SourceFWAppInit,
			wantUseInit: true,
			wantStale:   false,
			wantFetched: initAt,
		},
		{
			name:        "offline_cold_init_empty",
			agentOnline: false,
			agentAge:    time.Minute,
			agentTTL:    ttl,
			initOK:      false,
			wantSource:  SourceEmpty,
			wantUseInit: false,
			wantStale:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, useInit := Pick(tt.agentOnline, tt.agentAge, tt.agentTTL, tt.initOK, tt.initAt)
			if p.Source != tt.wantSource {
				t.Fatalf("source=%q want %q", p.Source, tt.wantSource)
			}
			if useInit != tt.wantUseInit {
				t.Fatalf("useInit=%v want %v", useInit, tt.wantUseInit)
			}
			if p.Stale != tt.wantStale {
				t.Fatalf("stale=%v want %v", p.Stale, tt.wantStale)
			}
			if !p.FetchedAt.Equal(tt.wantFetched) {
				t.Fatalf("fetched_at=%v want %v", p.FetchedAt, tt.wantFetched)
			}
		})
	}
}
