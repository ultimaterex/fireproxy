package modules

import "testing"

func TestRegistryEnablesKnownModules(t *testing.T) {
	r := NewRegistry(map[string]bool{
		"unifi-sync": true,
		"nope":       true,
		"ha-mqtt":    false,
	}, DefaultFactories())
	names := r.EnabledNames()
	if len(names) != 1 || names[0] != "unifi-sync" {
		t.Fatalf("names: %v", names)
	}
	r.StartAll()
	r.StopAll()
}

func TestRegistrySetAndApply(t *testing.T) {
	r := NewRegistry(nil, DefaultFactories())
	var saved map[string]bool
	r.SetPersist(func(m map[string]bool) { saved = m })
	if err := r.Set("unifi-sync", true); err != nil {
		t.Fatal(err)
	}
	if !saved["unifi-sync"] {
		t.Fatalf("persist: %v", saved)
	}
	list := r.List()
	if len(list) != 3 || list[0].ID != "unifi-sync" || !list[0].Enabled {
		t.Fatalf("list: %+v", list)
	}
	r2 := NewRegistry(nil, DefaultFactories())
	r2.Apply(saved)
	r2.StartAll()
	if got := r2.EnabledNames(); len(got) != 1 || got[0] != "unifi-sync" {
		t.Fatalf("apply: %v", got)
	}
	if err := r.Set("nope", true); err == nil {
		t.Fatal("want unknown")
	}
}

type fakeRep struct {
	Stub
	status, detail, site string
}

func (f *fakeRep) Report() (string, string, string) {
	return f.status, f.detail, f.site
}

func TestListCopiesReporter(t *testing.T) {
	r := NewRegistry(nil, map[string]func() Module{
		"unifi-sync": func() Module {
			return &fakeRep{Stub: Stub{ModuleName: "unifi-sync"}, status: "ok", detail: "10.5.67", site: "Default"}
		},
		"ha-mqtt": func() Module { return &Stub{ModuleName: "ha-mqtt"} },
	})
	if err := r.Set("unifi-sync", true); err != nil {
		t.Fatal(err)
	}
	list := r.List()
	if list[0].Status != "ok" || list[0].Detail != "10.5.67" || list[0].Site != "Default" {
		t.Fatalf("%+v", list[0])
	}
	if r.Running("unifi-sync") == nil {
		t.Fatal("running")
	}
}
