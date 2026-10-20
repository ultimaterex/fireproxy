package update

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"fireproxy/pkg/agentws"
)

// HTTPClient is used for binary downloads (overridable in tests).
var HTTPClient = &http.Client{Timeout: 2 * time.Minute}

// Restart reloads the agent unit after a successful binary replace (overridable in tests).
var Restart = func() error {
	cmd := exec.Command("systemctl", "restart", "fireproxy-agent")
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("sudo", "systemctl", "restart", "fireproxy-agent")
		if err2 := cmd.Run(); err2 != nil {
			return fmt.Errorf("restart: %v / %v", err, err2)
		}
	}
	return nil
}

// Allowed reports whether a gated agent should apply an update payload.
func Allowed(selfUpdate bool, upd *agentws.AgentUpdate) bool {
	return selfUpdate && upd != nil && upd.URL != "" && upd.SHA256 != ""
}

// Apply downloads, verifies, replaces the running binary, and restarts the unit.
func Apply(upd *agentws.AgentUpdate, destPath string) error {
	if !Allowed(true, upd) {
		return fmt.Errorf("invalid update")
	}
	res, err := HTTPClient.Get(upd.URL)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("download status %d", res.StatusCode)
	}
	tmp := destPath + ".new"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), res.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	_ = f.Close()
	sum := hex.EncodeToString(h.Sum(nil))
	if sum != upd.SHA256 {
		_ = os.Remove(tmp)
		return fmt.Errorf("sha256 mismatch")
	}
	if err := os.Rename(tmp, destPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Chmod(destPath, 0o755)
	return Restart()
}

// SelfPath returns the running executable path.
func SelfPath() (string, error) {
	return os.Executable()
}

// FileSHA256 returns the hex SHA-256 of path.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SelfSHA256 hashes the running executable (empty string on failure).
func SelfSHA256() string {
	path, err := SelfPath()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	sum, err := FileSHA256(path)
	if err != nil {
		return ""
	}
	return sum
}

// DestFromEnv prefers installed path.
func DestFromEnv() string {
	if p := os.Getenv("AGENT_BIN_PATH"); p != "" {
		return p
	}
	home := "/home/pi"
	return filepath.Join(home, "fireproxy", "bin", "fireproxy-agent")
}
