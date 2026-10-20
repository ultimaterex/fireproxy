package observatory

import (
	"context"
	"errors"
	"testing"
	"time"

	"fireproxy/server/internal/fwapp"
)

func TestVPNFromWarmInit(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	initAt := now.Add(-time.Minute)
	deps := Deps{
		Now: now,
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			return fwapp.ObservatorySnapshot{
				WGPeers: []fwapp.InitWGPeer{{Name: "phone"}},
				ClientProfiles: []fwapp.InitVPNClientFamily{
					{Family: "wireguard", Profiles: []fwapp.InitVPNClientProfile{{ProfileID: "A", DisplayName: "A", Status: "disconnected"}}},
					{Family: "openvpn", Profiles: nil},
				},
				WGClients: []fwapp.InitWGClient{{ProfileID: "A", Status: "disconnected"}},
			}, initAt, true
		},
		EnsureInit: func(ctx context.Context) error {
			t.Fatal("warm init should not EnsureInit")
			return nil
		},
	}
	view, prov, ok := VPN(context.Background(), deps)
	if !ok {
		t.Fatal("expected ok")
	}
	if prov.Source != SourceFWAppInit {
		t.Fatalf("source=%q", prov.Source)
	}
	if len(view.WGPeers) != 1 || len(view.ClientProfiles) != 2 {
		t.Fatalf("view=%+v", view)
	}
	if view.ClientProfiles[0].Family != "wireguard" || len(view.ClientProfiles[0].Profiles) != 1 {
		t.Fatalf("client_profiles=%+v", view.ClientProfiles)
	}
}

func TestVPNEmptyWithoutInit(t *testing.T) {
	deps := Deps{
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			return fwapp.ObservatorySnapshot{}, time.Time{}, false
		},
		EnsureInit: func(ctx context.Context) error {
			return errors.New("offline")
		},
	}
	_, prov, ok := VPN(context.Background(), deps)
	if ok {
		t.Fatal("expected !ok")
	}
	if prov.Source != SourceEmpty {
		t.Fatalf("source=%q", prov.Source)
	}
}
