package unifi

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestProbeOK(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-KEY") != "k" {
			t.Fatalf("key %q", r.Header.Get("X-API-KEY"))
		}
		switch {
		case r.URL.Path == "/proxy/network/integration/v1/info":
			_, _ = w.Write([]byte(`{"applicationVersion":"10.5.67"}`))
		case r.URL.Path == "/proxy/network/integration/v1/sites":
			_, _ = w.Write([]byte(`{"data":[{"id":"22222222-2222-2222-2222-222222222222","internalReference":"default","name":"Default"}]}`))
		case r.URL.Path == "/proxy/network/integration/v1/sites/22222222-2222-2222-2222-222222222222/devices":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.URL.Path == "/proxy/network/integration/v1/sites/22222222-2222-2222-2222-222222222222/clients":
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	m := New(Config{BaseURL: srv.URL, APIKey: "k", HTTP: srv.Client(), Interval: time.Hour})
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()
	if !m.waitReady(3 * time.Second) {
		t.Fatal("ready")
	}
	st, detail, site := m.Report()
	if st != "ok" || detail != "10.5.67" || site != "Default" {
		t.Fatalf("%s %s %s", st, detail, site)
	}
	if _, ok := m.Snapshot(); !ok {
		t.Fatal("snap")
	}
	w, wok := m.Wireless()
	if !wok {
		t.Fatal("wireless")
	}
	if w.Meta.NetworksSupported {
		t.Fatal("networks_supported want false on broadcasts 404")
	}
}

func TestWirelessRetainedWhenPollDown(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Load() >= 1 && r.URL.Path == "/proxy/network/integration/v1/info" {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`fail`))
			return
		}
		switch {
		case r.URL.Path == "/proxy/network/integration/v1/info":
			_, _ = w.Write([]byte(`{"applicationVersion":"10.5.67"}`))
		case r.URL.Path == "/proxy/network/integration/v1/sites":
			_, _ = w.Write([]byte(`{"data":[{"id":"22222222-2222-2222-2222-222222222222","internalReference":"default","name":"Default"}]}`))
		case r.URL.Path == "/proxy/network/integration/v1/sites/22222222-2222-2222-2222-222222222222/devices":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.URL.Path == "/proxy/network/integration/v1/sites/22222222-2222-2222-2222-222222222222/clients":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.URL.Path == "/proxy/network/integration/v1/sites/22222222-2222-2222-2222-222222222222/wifi/broadcasts":
			n.Add(1)
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	m := New(Config{BaseURL: srv.URL, APIKey: "k", HTTP: srv.Client(), Interval: 40 * time.Millisecond})
	_ = m.Start()
	defer m.Stop()
	if !m.waitReady(3 * time.Second) {
		t.Fatal("ready")
	}
	st, _, _ := m.Report()
	if st != "ok" {
		t.Fatalf("first %s", st)
	}
	w0, ok0 := m.Wireless()
	if !ok0 {
		t.Fatal("wireless first")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		st, detail, _ := m.Report()
		_, sok := m.Snapshot()
		w, wok := m.Wireless()
		if st == "down" && !sok && wok {
			if w.Meta.Error == "" || w.Meta.Error != detail {
				t.Fatalf("meta.error %q detail %q", w.Meta.Error, detail)
			}
			if w.Meta.Site != w0.Meta.Site {
				t.Fatalf("site %q want %q", w.Meta.Site, w0.Meta.Site)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	st, _, _ = m.Report()
	_, sok := m.Snapshot()
	_, wok := m.Wireless()
	t.Fatalf("still %s snap=%v wireless=%v", st, sok, wok)
}

func TestWirelessBroadcasts404StillOK(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/proxy/network/integration/v1/info":
			_, _ = w.Write([]byte(`{"applicationVersion":"10.5.67"}`))
		case r.URL.Path == "/proxy/network/integration/v1/sites":
			_, _ = w.Write([]byte(`{"data":[{"id":"22222222-2222-2222-2222-222222222222","internalReference":"default","name":"Default"}]}`))
		case r.URL.Path == "/proxy/network/integration/v1/sites/22222222-2222-2222-2222-222222222222/devices":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.URL.Path == "/proxy/network/integration/v1/sites/22222222-2222-2222-2222-222222222222/clients":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.URL.Path == "/proxy/network/integration/v1/sites/22222222-2222-2222-2222-222222222222/wifi/broadcasts":
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	m := New(Config{BaseURL: srv.URL, APIKey: "k", HTTP: srv.Client(), Interval: time.Hour})
	_ = m.Start()
	defer m.Stop()
	if !m.waitReady(3 * time.Second) {
		t.Fatal("ready")
	}
	st, _, _ := m.Report()
	if st != "ok" {
		t.Fatalf("%s", st)
	}
	w, ok := m.Wireless()
	if !ok {
		t.Fatal("wireless")
	}
	if w.Meta.NetworksSupported {
		t.Fatal("networks_supported")
	}
}

func TestProbe401(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"code":401,"message":"Unauthorized"}}`))
	}))
	defer srv.Close()
	m := New(Config{BaseURL: srv.URL, APIKey: "bad", HTTP: srv.Client(), Interval: time.Hour})
	_ = m.Start()
	defer m.Stop()
	if !m.waitReady(3 * time.Second) {
		t.Fatal("ready")
	}
	st, detail, _ := m.Report()
	if st != "down" || detail == "" {
		t.Fatalf("%s %s", st, detail)
	}
}

func TestMissingKey(t *testing.T) {
	m := New(Config{BaseURL: "https://unifi.lan:11443"})
	_ = m.Start()
	defer m.Stop()
	if !m.waitReady(3 * time.Second) {
		t.Fatal("ready")
	}
	st, detail, _ := m.Report()
	if st != "down" || detail == "" {
		t.Fatalf("%s %s", st, detail)
	}
}

func TestPollDownClearsOK(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Load() >= 1 {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`fail`))
			return
		}
		switch {
		case r.URL.Path == "/proxy/network/integration/v1/info":
			_, _ = w.Write([]byte(`{"applicationVersion":"10.5.67"}`))
		case r.URL.Path == "/proxy/network/integration/v1/sites":
			_, _ = w.Write([]byte(`{"data":[{"id":"22222222-2222-2222-2222-222222222222","internalReference":"default","name":"Default"}]}`))
		case r.URL.Path == "/proxy/network/integration/v1/sites/22222222-2222-2222-2222-222222222222/devices":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.URL.Path == "/proxy/network/integration/v1/sites/22222222-2222-2222-2222-222222222222/clients":
			n.Add(1)
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	m := New(Config{BaseURL: srv.URL, APIKey: "k", HTTP: srv.Client(), Interval: 40 * time.Millisecond})
	_ = m.Start()
	defer m.Stop()
	if !m.waitReady(3 * time.Second) {
		t.Fatal("ready")
	}
	st, _, _ := m.Report()
	if st != "ok" {
		t.Fatalf("first %s", st)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		st, _, _ = m.Report()
		_, ok := m.Snapshot()
		if st == "down" && !ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("still %s ok=%v", st, func() bool { _, ok := m.Snapshot(); return ok }())
}

func TestProbeTimeout(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(400 * time.Millisecond)
	}))
	defer srv.Close()
	cli := srv.Client()
	cli.Timeout = 50 * time.Millisecond
	m := New(Config{BaseURL: srv.URL, APIKey: "k", HTTP: cli, Interval: time.Hour})
	_ = m.Start()
	defer m.Stop()
	if !m.waitReady(3 * time.Second) {
		t.Fatal("ready")
	}
	st, detail, _ := m.Report()
	if st != "down" || detail == "" {
		t.Fatalf("%s %s", st, detail)
	}
}
