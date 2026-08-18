package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"fireproxy/server/internal/api"
	"fireproxy/server/internal/store"
)

func TestControlHistoryListAndPagination(t *testing.T) {
	p, err := store.OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })

	now := time.Now().UnixMilli()
	if err := p.InsertControlEvent(store.ControlEvent{
		TS: now, Scheme: "firewalla", Action: "host.rename", ActorKind: "user", Actor: "admin",
		Target: "aa:bb", Summary: "renamed", Result: "ok",
		BeforeJSON: `{"name":"old"}`, AfterJSON: `{"name":"new"}`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := p.InsertControlEvent(store.ControlEvent{
		TS: now + 1, Scheme: "firewalla", Action: "host.dns", ActorKind: "user", Actor: "admin",
		Target: "aa:bb", Summary: "dns set", Result: "ok",
		BeforeJSON: `{"hostname":"a"}`, AfterJSON: `{"hostname":"b"}`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := p.InsertControlEvent(store.ControlEvent{
		TS: now + 2, Scheme: "unifi", Action: "client.rename", ActorKind: "system", Actor: "name-sync",
		Target: "cc:dd", Result: "ok",
	}); err != nil {
		t.Fatal(err)
	}

	s := &api.Server{Persist: p}
	mux := http.NewServeMux()
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/history?scheme=firewalla&limit=1", nil))
	if rr.Code != 200 {
		t.Fatalf("history %d %s", rr.Code, rr.Body.String())
	}
	var page1 struct {
		Events []struct {
			ID        int64          `json:"id"`
			Scheme    string         `json:"scheme"`
			Action    string         `json:"action"`
			ActorKind string         `json:"actor_kind"`
			Actor     string         `json:"actor"`
			Target    string         `json:"target"`
			Summary   string         `json:"summary"`
			Result    string         `json:"result"`
			Before    map[string]any `json:"before"`
			After     map[string]any `json:"after"`
		} `json:"events"`
		Actions map[string][]string `json:"actions"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &page1); err != nil {
		t.Fatal(err)
	}
	if len(page1.Events) != 1 {
		t.Fatalf("want 1 event, got %+v", page1.Events)
	}
	ev := page1.Events[0]
	if ev.Scheme != "firewalla" || ev.Action != "host.dns" {
		t.Fatalf("newest firewalla row: %+v", ev)
	}
	if ev.Before["hostname"] != "a" || ev.After["hostname"] != "b" {
		t.Fatalf("before/after: %+v %+v", ev.Before, ev.After)
	}
	fwActions := page1.Actions["firewalla"]
	if len(fwActions) != 9 || fwActions[0] != "host.rename" || fwActions[4] != "rule.create" {
		t.Fatalf("actions firewalla: %+v", page1.Actions)
	}
	if len(page1.Actions["unifi"]) != 1 || page1.Actions["unifi"][0] != "client.rename" {
		t.Fatalf("actions unifi: %+v", page1.Actions)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/history?scheme=firewalla&limit=1&before_id="+strconv.FormatInt(ev.ID, 10), nil))
	if rr.Code != 200 {
		t.Fatalf("page2 %d %s", rr.Code, rr.Body.String())
	}
	var page2 struct {
		Events []struct {
			ID     int64          `json:"id"`
			Action string         `json:"action"`
			Before map[string]any `json:"before"`
			After  map[string]any `json:"after"`
		} `json:"events"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &page2); err != nil {
		t.Fatal(err)
	}
	if len(page2.Events) != 1 || page2.Events[0].Action != "host.rename" {
		t.Fatalf("page2: %+v", page2.Events)
	}
	if page2.Events[0].ID >= ev.ID {
		t.Fatalf("before_id pagination: got id %d want < %d", page2.Events[0].ID, ev.ID)
	}
	if page2.Events[0].Before["name"] != "old" || page2.Events[0].After["name"] != "new" {
		t.Fatalf("rename snapshots: %+v", page2.Events[0])
	}
}

func TestControlHistorySettingsRetention(t *testing.T) {
	p, err := store.OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })

	s := &api.Server{Persist: p}
	mux := http.NewServeMux()
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/settings/history", nil))
	if rr.Code != 200 {
		t.Fatalf("get %d %s", rr.Code, rr.Body.String())
	}
	var getBody struct {
		RetentionDays int `json:"retention_days"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &getBody); err != nil {
		t.Fatal(err)
	}
	if getBody.RetentionDays != 365 {
		t.Fatalf("default retention: %+v", getBody)
	}

	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/settings/history", strings.NewReader(`{"retention_days":90}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("put %d %s", rr.Code, rr.Body.String())
	}
	var putBody struct {
		RetentionDays int `json:"retention_days"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &putBody); err != nil {
		t.Fatal(err)
	}
	if putBody.RetentionDays != 90 {
		t.Fatalf("put body: %+v", putBody)
	}
	if p.ControlHistoryRetentionDays() != 90 {
		t.Fatalf("persist retention %d", p.ControlHistoryRetentionDays())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/settings/history", nil))
	_ = json.Unmarshal(rr.Body.Bytes(), &getBody)
	if getBody.RetentionDays != 90 {
		t.Fatalf("get after put: %+v", getBody)
	}
}
