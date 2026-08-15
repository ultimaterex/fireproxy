package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fireproxy/server/internal/api"
	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/tplink"
)

func TestFWAppRoutes(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("11", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &fwapp.CredentialVault{Store: fwapp.NewMemStore(), Key: key}
	lan := fwapp.NewLANClient()
	lan.HTTP = &http.Client{Transport: fwAppOKTransport{sym: strings.Repeat("s", 32)}}
	svc := fwapp.NewServiceWithVault(v, lan)
	svc.SetPairFn(func(ctx context.Context, req fwapp.PairRequest) (fwapp.Creds, error) {
		return fwapp.Creds{
			PairedAt: time.Now().UTC(),
			BoxIP:    req.BoxIP,
			Gid:      "g1",
			Eid:      "e1",
			SymKey:   strings.Repeat("s", 32),
			Email:    req.Email,
		}, nil
	})

	mux := http.NewServeMux()
	s := &api.Server{FWApp: svc}
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/fw-app/status", nil))
	if rr.Code != 200 {
		t.Fatalf("status code %d", rr.Code)
	}
	var st fwapp.Status
	if err := json.NewDecoder(rr.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if st.Paired || !st.SecretsReady {
		t.Fatalf("%+v", st)
	}

	body := `{"qr_json":"{\"gid\":\"g\",\"seed\":\"s\",\"license\":\"lic12345\",\"ek\":\"x\"}","box_ip":"127.0.0.1","email":"a@b.co"}`
	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/pair", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("pair %d %s", rr.Code, rr.Body.String())
	}
	if err := json.NewDecoder(rr.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if st.State != "lan-ok" {
		t.Fatalf("%+v", st)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/fw-app/ping", nil))
	if rr.Code != 200 {
		t.Fatalf("ping %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/fw-app/wol", strings.NewReader(`{"mac":"aa-bb-cc-dd-ee-ff"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("wol %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/fw-app/wol", strings.NewReader(`{"mac":"nope"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("wol bad mac %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/fw-app/hosts/rename", strings.NewReader(`{"mac":"aa-bb-cc-dd-ee-ff","name":"Lab Host"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("rename %d %s", rr.Code, rr.Body.String())
	}
	var renameBody struct {
		OK   bool   `json:"ok"`
		Name string `json:"name"`
		MAC  string `json:"mac"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&renameBody); err != nil {
		t.Fatal(err)
	}
	if !renameBody.OK || renameBody.Name != "Lab Host" || renameBody.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("%+v", renameBody)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/fw-app/hosts/rename", strings.NewReader(`{"mac":"aa-bb-cc-dd-ee-ff","name":"  "}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("rename empty %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/fw-app/speedtest", strings.NewReader(`{"wan_uuid":"wan-abc"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("speedtest %d %s", rr.Code, rr.Body.String())
	}
	var startBody struct {
		Job fwapp.SpeedtestJob `json:"job"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&startBody); err != nil {
		t.Fatal(err)
	}
	if startBody.Job.ID == "" {
		t.Fatal("missing job id")
	}
	var got fwapp.SpeedtestJob
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rr = httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/fw-app/speedtest/"+startBody.Job.ID, nil))
		if rr.Code != 200 {
			t.Fatalf("job get %d", rr.Code)
		}
		var poll struct {
			Job fwapp.SpeedtestJob `json:"job"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&poll); err != nil {
			t.Fatal(err)
		}
		got = poll.Job
		if got.State == "done" || got.State == "error" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got.State != "done" || got.Result == nil || got.Result.Down != 111 {
		t.Fatalf("%+v", got)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/v1/fw-app/pair", nil))
	if rr.Code != 200 {
		t.Fatalf("unpair %d", rr.Code)
	}
}

type fwAppOKTransport struct{ sym string }

func (t fwAppOKTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	reply, _ := fwapp.AESEncryptLegacy(t.sym, `{"code":200,"data":{"result":{"success":true,"timestamp":1700000000,"result":{"download":111,"upload":22,"latency":9,"jitter":1}}}}`)
	raw, _ := json.Marshal(map[string]any{"message": reply})
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(string(raw))),
		Header:     make(http.Header),
	}, nil
}
