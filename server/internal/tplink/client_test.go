package tplink

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"fireproxy/server/internal/store"
)

func TestClientScrape(t *testing.T) {
	sys := testdata("system_info.html")
	ports := testdata("port_statistics.html")
	poe := testdata("poe_config.html")
	loginPage := `<script>var logonInfo = new Array(1,0,0);</script><form><input name="username"></form>`
	requireSSID := func(w http.ResponseWriter, r *http.Request, body string) {
		c, err := r.Cookie("H_P_SSID")
		if err != nil || c.Value == "" {
			_, _ = w.Write([]byte(loginPage))
			return
		}
		_, _ = w.Write([]byte(body))
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/logon.cgi", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "H_P_SSID", Value: "tplink_test", Path: "/"})
		_, _ = w.Write([]byte("<script>var logonInfo = new Array(\n0,\n0,0);</script>"))
	})
	mux.HandleFunc("/SystemInfoRpm.htm", func(w http.ResponseWriter, r *http.Request) { requireSSID(w, r, sys) })
	mux.HandleFunc("/PortStatisticsRpm.htm", func(w http.ResponseWriter, r *http.Request) { requireSSID(w, r, ports) })
	mux.HandleFunc("/PoeConfigRpm.htm", func(w http.ResponseWriter, r *http.Request) { requireSSID(w, r, poe) })
	loggedOut := false
	mux.HandleFunc("/Logout.htm", func(w http.ResponseWriter, r *http.Request) {
		loggedOut = true
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, User: "admin", Pass: "x", HTTP: srv.Client()}
	res, err := c.Scrape()
	if err != nil {
		t.Fatal(err)
	}
	if res.Info.Model != "TL-SG1016PE" || len(res.Ports) != 16 || res.Poe == nil {
		t.Fatalf("%+v", res)
	}
	if !loggedOut {
		t.Fatal("expected logout")
	}
}

func TestLoginFailed(t *testing.T) {
	if loginFailed(`<script>var logonInfo = new Array(0,0,0);</script>`) {
		t.Fatal("success should not fail")
	}
	if !loginFailed(`<script>var logonInfo = new Array(1,0,0);</script>`) {
		t.Fatal("bad creds should fail")
	}
	if loginFailed("no logonInfo here") {
		t.Fatal("missing logonInfo is not a login failure signal")
	}
}

func TestStoreEncryptRoundTrip(t *testing.T) {
	p, err := store.OpenPersist(filepath.Join(t.TempDir(), "t.db"), 7)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	st := NewStore(p, testSecretsKey)
	pass := "ripjaws"
	en := true
	c, err := st.Create(UpsertInput{IP: "203.0.113.10", Username: "admin", Password: &pass, Enabled: &en})
	if err != nil {
		t.Fatal(err)
	}
	if c.PasswordCiphertext == "" || c.PasswordCiphertext == pass {
		t.Fatal("password not encrypted")
	}
	got, err := st.DecryptPassword(c)
	if err != nil || got != pass {
		t.Fatalf("decrypt %q %v", got, err)
	}
	if !c.PublicView().HasPassword {
		t.Fatal("has_password")
	}
	_, err = st.Create(UpsertInput{IP: "1.2.3.4", Password: &pass})
	st2 := NewStore(p, "")
	_, err = st2.Create(UpsertInput{IP: "1.2.3.5", Password: &pass})
	if err != ErrNoSecretsKey {
		t.Fatalf("want ErrNoSecretsKey got %v", err)
	}
}
