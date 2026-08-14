package geo

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	permalinkCountry = "https://download.maxmind.com/geoip/databases/GeoLite2-Country/download?suffix=tar.gz"
	legacyDownload   = "https://download.maxmind.com/app/geoip_download"
	userAgent        = "fireproxy-server"
)

// Updater downloads GeoLite2-Country.mmdb via MaxMind permalink + license key.
type Updater struct {
	AccountID  string
	LicenseKey string
	Path       string
	Interval   time.Duration
	Client     *http.Client
	Holder     *Holder
	URL        string // test override
}

func (u *Updater) client() *http.Client {
	if u.Client != nil {
		return u.Client
	}
	return &http.Client{Timeout: 2 * time.Minute}
}

func (u *Updater) downloadURL() string {
	if strings.TrimSpace(u.URL) != "" {
		return u.URL
	}
	if strings.TrimSpace(u.AccountID) != "" {
		return permalinkCountry
	}
	q := url.Values{}
	q.Set("edition_id", "GeoLite2-Country")
	q.Set("license_key", u.LicenseKey)
	q.Set("suffix", "tar.gz")
	return legacyDownload + "?" + q.Encode()
}

func (u *Updater) auth(req *http.Request) {
	if id := strings.TrimSpace(u.AccountID); id != "" {
		req.SetBasicAuth(id, u.LicenseKey)
	}
	req.Header.Set("User-Agent", userAgent)
}

// Loop checks on an interval until ctx is cancelled.
func (u *Updater) Loop(ctx context.Context) {
	if u.Interval <= 0 {
		u.Interval = 12 * time.Hour
	}
	t := time.NewTicker(u.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := u.Once(ctx); err != nil {
				log.Printf("geo: %v", err)
			}
		}
	}
}

// Once HEAD-checks Last-Modified, downloads if newer/missing, then reloads the MMDB.
func (u *Updater) Once(ctx context.Context) error {
	return u.sync(ctx)
}

func (u *Updater) sync(ctx context.Context) error {
	if strings.TrimSpace(u.LicenseKey) == "" {
		return fmt.Errorf("MAXMIND_LICENSE_KEY is empty")
	}
	if strings.TrimSpace(u.Path) == "" {
		return fmt.Errorf("GEOIP_MMDB is empty")
	}
	need, err := u.stale(ctx)
	if err != nil {
		return err
	}
	if !need {
		if u.Holder != nil && !u.Holder.Ready() {
			if look := OpenMMDB(u.Path); look != nil {
				u.Holder.Swap(look)
			}
		}
		return nil
	}
	if err := u.fetch(ctx); err != nil {
		return err
	}
	look := OpenMMDB(u.Path)
	if look == nil {
		return fmt.Errorf("downloaded mmdb unreadable: %s", u.Path)
	}
	if u.Holder != nil {
		u.Holder.Swap(look)
	} else {
		closeLookup(look)
	}
	return nil
}

func (u *Updater) stale(ctx context.Context) (bool, error) {
	st, err := os.Stat(u.Path)
	if err != nil {
		return true, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u.downloadURL(), nil)
	if err != nil {
		return false, err
	}
	u.auth(req)
	resp, err := u.client().Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return false, fmt.Errorf("maxmind HEAD %s", resp.Status)
	}
	lm := resp.Header.Get("Last-Modified")
	if lm == "" {
		return true, nil
	}
	mod, err := http.ParseTime(lm)
	if err != nil {
		return true, nil
	}
	return mod.After(st.ModTime().Add(time.Minute)), nil
}

func (u *Updater) fetch(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.downloadURL(), nil)
	if err != nil {
		return err
	}
	u.auth(req)
	resp, err := u.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("maxmind GET %s", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(u.Path), 0o755); err != nil {
		return err
	}
	return extractMMDB(resp.Body, u.Path)
}

func extractMMDB(r io.Reader, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("archive has no .mmdb")
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != 0 {
			continue
		}
		name := filepath.ToSlash(hdr.Name)
		if strings.Contains(name, "..") || !strings.HasSuffix(strings.ToLower(name), ".mmdb") {
			continue
		}
		tmp := dest + ".new"
		f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(f, tr)
		closeErr := f.Close()
		if copyErr != nil {
			_ = os.Remove(tmp)
			return copyErr
		}
		if closeErr != nil {
			_ = os.Remove(tmp)
			return closeErr
		}
		if err := os.Rename(tmp, dest); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		return nil
	}
}
