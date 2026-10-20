package fwapp

import (
	"testing"
	"time"
)

func TestVPNCacheIsolatesProfiles(t *testing.T) {
	snap, err := ParseInitObservatory([]byte(`{"wgvpnClientProfiles":[{"profileId":"test","status":true,"settings":{"displayName":"Test VPN"}}],"ovpnClientProfiles":[{"profileId":"other","status":false}]}`))
	if err != nil {
		t.Fatal(err)
	}
	var cache ObservatoryCache
	cache.Set(snap, time.Now())
	snap.ClientProfiles[0].Profiles[0].DisplayName = "changed input"
	got, _, _ := cache.Get()
	if got.ClientProfiles[0].Profiles[0].DisplayName != "Test VPN" || got.ClientProfiles[0].Profiles[0].Status != "connected" || got.ClientProfiles[2].Profiles[0].Status != "disconnected" {
		t.Fatalf("profiles = %+v", got.ClientProfiles)
	}
	got.ClientProfiles[0].Profiles[0].DisplayName = "changed output"
	got.WGClients[0].DisplayName = "changed legacy"
	again, _, _ := cache.Get()
	if again.ClientProfiles[0].Profiles[0].DisplayName != "Test VPN" || again.WGClients[0].DisplayName != "Test VPN" {
		t.Fatal("caller mutated cached VPN inventory")
	}
}
