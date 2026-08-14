package unifi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestApplyNamesPutsClassicUser(t *testing.T) {
	var puts atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method %s", r.Method)
		}
		if r.Header.Get("X-API-KEY") != "k" {
			t.Fatalf("key")
		}
		if r.URL.Path != "/proxy/network/api/s/default/rest/user/64aaaaaaaaaaaaaaaaaaaa02" {
			t.Fatalf("path %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		_ = json.Unmarshal(body, &payload)
		if payload["_id"] != "64aaaaaaaaaaaaaaaaaaaa02" || payload["name"] != "NAS" {
			t.Fatalf("%s", body)
		}
		if _, ok := payload["id"]; ok {
			t.Fatal("must not send Integration UUID")
		}
		puts.Add(1)
		_, _ = w.Write([]byte(`{"meta":{"rc":"ok"}}`))
	}))
	defer srv.Close()
	users := []User{{ID: "64aaaaaaaaaaaaaaaaaaaa02", MAC: "02:00:00:00:00:02"}}
	rows := []NameRow{{MAC: "02:00:00:00:00:02", Firewalla: "NAS", Status: "conflict"}}
	got := ApplyNames(srv.Client(), srv.URL, "k", "default", users, rows)
	if puts.Load() != 1 || len(got) != 1 || !got[0].OK {
		t.Fatalf("%+v puts=%d", got, puts.Load())
	}
}

func TestApplyNamesContinuesAfterError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/proxy/network/api/s/default/rest/user/bad" {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`nope`))
			return
		}
		_, _ = w.Write([]byte(`{"meta":{"rc":"ok"}}`))
	}))
	defer srv.Close()
	users := []User{
		{ID: "bad", MAC: "02:00:00:00:00:02"},
		{ID: "ok", MAC: "02:00:00:00:00:03"},
	}
	rows := []NameRow{
		{MAC: "02:00:00:00:00:02", Firewalla: "NAS", Status: "conflict"},
		{MAC: "02:00:00:00:00:03", Firewalla: "phone", Status: "empty"},
	}
	got := ApplyNames(srv.Client(), srv.URL, "k", "default", users, rows)
	if len(got) != 2 || got[0].OK || !got[1].OK {
		t.Fatalf("%+v", got)
	}
}

func TestApplyNamesAbortsOn401(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`no`))
	}))
	defer srv.Close()
	users := []User{
		{ID: "a", MAC: "02:00:00:00:00:02"},
		{ID: "b", MAC: "02:00:00:00:00:03"},
	}
	rows := []NameRow{
		{MAC: "02:00:00:00:00:02", Firewalla: "NAS", Status: "conflict"},
		{MAC: "02:00:00:00:00:03", Firewalla: "phone", Status: "empty"},
	}
	got := ApplyNames(srv.Client(), srv.URL, "k", "default", users, rows)
	if n.Load() != 1 || len(got) != 2 || got[0].OK || got[1].OK || got[1].Error == "" {
		t.Fatalf("n=%d %+v", n.Load(), got)
	}
}

func TestApplyNamesRetries429Once(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.WriteHeader(429)
			return
		}
		_, _ = w.Write([]byte(`{"meta":{"rc":"ok"}}`))
	}))
	defer srv.Close()
	users := []User{{ID: "u", MAC: "02:00:00:00:00:02"}}
	rows := []NameRow{{MAC: "02:00:00:00:00:02", Firewalla: "NAS", Status: "conflict"}}
	got := ApplyNames(srv.Client(), srv.URL, "k", "default", users, rows)
	if n.Load() != 2 || !got[0].OK {
		t.Fatalf("n=%d %+v", n.Load(), got)
	}
}

func TestSelectRows(t *testing.T) {
	all := []NameRow{
		{MAC: "02:00:00:00:00:02", Status: "conflict", Firewalla: "NAS"},
		{MAC: "02:00:00:00:00:03", Status: "empty", Firewalla: "phone"},
	}
	if got := SelectRows(all, nil, false); len(got) != 2 {
		t.Fatalf("%+v", got)
	}
	if got := SelectRows(all, nil, true); len(got) != 1 || got[0].Status != "empty" {
		t.Fatalf("emptyOnly: %+v", got)
	}
	if got := SelectRows(all, []string{"02:00:00:00:00:02"}, false); len(got) != 1 || got[0].MAC != "02:00:00:00:00:02" {
		t.Fatalf("macs: %+v", got)
	}
	if got := SelectRows(all, []string{"02:00:00:00:00:99"}, false); len(got) != 0 {
		t.Fatalf("unknown: %+v", got)
	}
}
