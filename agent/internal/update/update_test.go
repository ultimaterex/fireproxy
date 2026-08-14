package update

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"fireproxy/pkg/agentws"
)

func TestAllowedGate(t *testing.T) {
	upd := &agentws.AgentUpdate{URL: "http://x", SHA256: "abc"}
	if Allowed(false, upd) {
		t.Fatal("flag off should ignore")
	}
	if Allowed(true, nil) || Allowed(true, &agentws.AgentUpdate{}) {
		t.Fatal("incomplete payload")
	}
	if !Allowed(true, upd) {
		t.Fatal("want allow")
	}
}

func TestApplyDownloadVerify(t *testing.T) {
	body := []byte("agent-bytes")
	sum := sha256.Sum256(body)
	sha := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	oldClient, oldRestart := HTTPClient, Restart
	HTTPClient = srv.Client()
	Restart = func() error { return nil }
	t.Cleanup(func() {
		HTTPClient = oldClient
		Restart = oldRestart
	})

	dest := filepath.Join(t.TempDir(), "fireproxy-agent")
	if err := Apply(&agentws.AgentUpdate{URL: srv.URL, SHA256: sha, Version: "1.0.1"}, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("dest %q", got)
	}

	if err := Apply(&agentws.AgentUpdate{URL: srv.URL, SHA256: "deadbeef", Version: "1.0.2"}, dest+".2"); err == nil {
		t.Fatal("want sha mismatch")
	}
	if _, err := os.Stat(dest + ".2"); !os.IsNotExist(err) {
		t.Fatalf("mismatch should not leave dest: %v", err)
	}
}
