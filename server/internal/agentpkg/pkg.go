package agentpkg

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultArch is served when a request omits an arch (legacy installs, and the
// arch the Firewalla Gold SE historically shipped).
const DefaultArch = "arm64"

// Binary is one architecture's agent binary.
type Binary struct {
	Arch   string
	Path   string
	SHA256 string
}

// Package describes the on-disk agent binaries served for install/update.
// It holds one Binary per architecture (arm64, amd64). Path/SHA256 mirror the
// default arch for callers that are not arch-aware.
type Package struct {
	Dir     string
	Version string
	Bins    map[string]Binary // normalized arch -> binary
	SHA256  string            // default arch (back-compat)
	Path    string            // default arch (back-compat)
	missing bool
}

// NormalizeArch maps uname/GOARCH spellings to canonical arm64 / amd64.
func NormalizeArch(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "arm64", "aarch64":
		return "arm64"
	case "amd64", "x86_64", "x64":
		return "amd64"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

// Open scans dir for per-arch agent binaries (fireproxy-agent-<arch>) plus an
// optional VERSION file. A legacy unsuffixed fireproxy-agent is treated as the
// default arch so older layouts keep working.
func Open(dir string) (*Package, error) {
	p := &Package{Dir: dir, Version: "0.0.0", Bins: map[string]Binary{}}
	if dir == "" {
		p.missing = true
		return p, nil
	}
	add := func(arch, path string) error {
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		sum := sha256.Sum256(b)
		p.Bins[arch] = Binary{Arch: arch, Path: path, SHA256: hex.EncodeToString(sum[:])}
		return nil
	}
	for _, arch := range []string{"arm64", "amd64"} {
		if err := add(arch, filepath.Join(dir, "fireproxy-agent-"+arch)); err != nil {
			return nil, err
		}
	}
	// Legacy unsuffixed binary -> default arch (only if not already present).
	if _, ok := p.Bins[DefaultArch]; !ok {
		if err := add(DefaultArch, filepath.Join(dir, "fireproxy-agent")); err != nil {
			return nil, err
		}
	}
	if len(p.Bins) == 0 {
		p.missing = true
		return p, nil
	}
	if vb, err := os.ReadFile(filepath.Join(dir, "VERSION")); err == nil {
		if v := strings.TrimSpace(string(vb)); v != "" {
			p.Version = v
		}
	}
	if def, ok := p.Bin(DefaultArch); ok {
		p.Path = def.Path
		p.SHA256 = def.SHA256
	}
	// Bare "dev" collides under semver compare; stamp with binary digest so rebuilds are detectable.
	if p.Version == "dev" && p.SHA256 != "" {
		p.Version = "dev-" + p.SHA256[:12]
	}
	return p, nil
}

// Bin returns the binary for an arch (any spelling). An empty arch resolves to
// the default arch; if that is absent, any available binary is returned.
func (p *Package) Bin(arch string) (Binary, bool) {
	if p == nil {
		return Binary{}, false
	}
	if a := NormalizeArch(arch); a != "" {
		if b, ok := p.Bins[a]; ok {
			return b, true
		}
		if arch != "" {
			return Binary{}, false // caller asked for a specific arch we don't have
		}
	}
	if b, ok := p.Bins[DefaultArch]; ok {
		return b, true
	}
	// Deterministic fallback: the lowest arch name by sort order, so the
	// choice is stable across runs (map iteration order is randomized).
	for _, a := range p.Arches() {
		return p.Bins[a], true
	}
	return Binary{}, false
}

// Arches lists the architectures this package can serve.
func (p *Package) Arches() []string {
	if p == nil {
		return nil
	}
	out := make([]string, 0, len(p.Bins))
	for a := range p.Bins {
		out = append(out, a)
	}
	// Sorted so /agent/version output is stable across runs and diffs.
	sort.Strings(out)
	return out
}

func (p *Package) Missing() bool {
	return p == nil || p.missing || len(p.Bins) == 0
}

func (p *Package) Info() map[string]string {
	if p.Missing() {
		return map[string]string{"version": "", "sha256": "", "missing": "true"}
	}
	return map[string]string{
		"version": p.Version,
		"sha256":  p.SHA256,
		"missing": "false",
	}
}

// Newer reports whether packaged version is semver-greater than agentVer.
func Newer(packaged, agentVer string) bool {
	return cmpSemver(packaged, agentVer) > 0
}

// NeedsUpdate reports whether the packaged agent should replace the running one.
// Prefer SHA mismatch when the agent reports a digest (covers equal-version rebuilds).
// Else semver Newer, else raw version string inequality (covers VERSION=dev stamps).
func NeedsUpdate(packagedVer, agentVer, packagedSHA, agentSHA string) bool {
	packagedSHA = strings.TrimSpace(strings.ToLower(packagedSHA))
	agentSHA = strings.TrimSpace(strings.ToLower(agentSHA))
	if packagedSHA != "" && agentSHA != "" {
		return packagedSHA != agentSHA
	}
	pv := strings.TrimSpace(packagedVer)
	av := strings.TrimSpace(agentVer)
	if Newer(pv, av) {
		return true
	}
	if pv != "" && av != "" && pv != av {
		return true
	}
	return false
}

func cmpSemver(a, b string) int {
	pa := parseVer(a)
	pb := parseVer(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func parseVer(s string) [3]int {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	var out [3]int
	parts := strings.Split(s, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		n := 0
		fmt.Sscanf(parts[i], "%d", &n)
		out[i] = n
	}
	return out
}
