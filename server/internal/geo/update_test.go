package geo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func packMMDB(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractMMDBFromTarGz(t *testing.T) {
	raw := packMMDB(t, "GeoLite2-Country_20260101/GeoLite2-Country.mmdb", []byte("mmdb-bytes"))
	dir := t.TempDir()
	dest := filepath.Join(dir, "GeoLite2-Country.mmdb")
	if err := extractMMDB(bytes.NewReader(raw), dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "mmdb-bytes" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractMMDBRejectsEmptyArchive(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.Close()
	_ = gz.Close()
	if err := extractMMDB(bytes.NewReader(buf.Bytes()), filepath.Join(t.TempDir(), "x.mmdb")); err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdaterDownloadsWhenMissing(t *testing.T) {
	raw := packMMDB(t, "GeoLite2-Country.mmdb", []byte("downloaded"))
	var gets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "42" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodHead {
			w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusOK)
			return
		}
		gets++
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(srv.Close)

	dest := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	h := NewHolder(nil)
	u := &Updater{
		AccountID:  "42",
		LicenseKey: "secret",
		Path:       dest,
		Client:     srv.Client(),
		Holder:     h,
		URL:        srv.URL,
	}
	err := u.Once(context.Background())
	if err == nil {
		t.Fatal("fake mmdb should not open")
	}
	if gets != 1 {
		t.Fatalf("gets %d", gets)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "downloaded" {
		t.Fatalf("got %q", got)
	}
}

func TestUpdaterSkipsFreshFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "GeoLite2-Country.mmdb")
	if err := os.WriteFile(dest, []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	_ = os.Chtimes(dest, now, now)
	var gets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets++
			_, _ = io.Copy(io.Discard, r.Body)
		}
		w.Header().Set("Last-Modified", now.Add(-2*time.Hour).UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	u := &Updater{
		AccountID:  "42",
		LicenseKey: "secret",
		Path:       dest,
		Client:     srv.Client(),
		URL:        srv.URL,
	}
	if err := u.Once(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gets != 0 {
		t.Fatalf("unexpected GET %d", gets)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "local" {
		t.Fatalf("rewrote file: %q", got)
	}
}
