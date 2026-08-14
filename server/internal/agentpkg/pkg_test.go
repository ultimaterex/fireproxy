package agentpkg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenMissing(t *testing.T) {
	p, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !p.Missing() {
		t.Fatal("expected missing")
	}
}

func TestOpenPresent(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fireproxy-agent")
	if err := os.WriteFile(bin, []byte("fake-agent-bytes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir)
	if err != nil || p.Missing() {
		t.Fatalf("%v missing=%v", err, p.Missing())
	}
	if p.Version != "1.2.3" || p.SHA256 == "" {
		t.Fatalf("%+v", p)
	}
	if !Newer("1.2.4", "1.2.3") {
		t.Fatal("1.2.4 should be newer")
	}
	if Newer("1.2.3", "1.2.3") {
		t.Fatal("equal not newer")
	}
	if Newer("1.2.2", "1.2.3") {
		t.Fatal("older not newer")
	}
	// Legacy unsuffixed binary resolves as the default arch.
	if b, ok := p.Bin(""); !ok || b.Arch != DefaultArch {
		t.Fatalf("legacy default arch: %+v ok=%v", b, ok)
	}
}

func TestBinFallbackDeterministicWhenDefaultAbsent(t *testing.T) {
	// Two non-default arches, the default arch (arm64) absent. Empty-arch Bin()
	// must resolve to the sorted-first arch deterministically rather than a
	// random map entry.
	p := &Package{Bins: map[string]Binary{
		"amd64":   {Arch: "amd64", SHA256: "a"},
		"riscv64": {Arch: "riscv64", SHA256: "r"},
	}}
	for i := 0; i < 50; i++ {
		b, ok := p.Bin("")
		if !ok || b.Arch != "amd64" {
			t.Fatalf("empty-arch fallback must be deterministic amd64: %+v ok=%v", b, ok)
		}
	}
}

func TestArchesSorted(t *testing.T) {
	p := &Package{Bins: map[string]Binary{
		"arm64": {Arch: "arm64"},
		"amd64": {Arch: "amd64"},
	}}
	got := p.Arches()
	want := []string{"amd64", "arm64"}
	if len(got) != len(want) {
		t.Fatalf("arches %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arches %v want %v", got, want)
		}
	}
}

func TestOpenMultiArch(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("fireproxy-agent-arm64", "arm-bytes")
	write("fireproxy-agent-amd64", "x86-bytes")
	write("VERSION", "2.0.0\n")

	p, err := Open(dir)
	if err != nil || p.Missing() {
		t.Fatalf("%v missing=%v", err, p.Missing())
	}
	if p.Version != "2.0.0" || len(p.Arches()) != 2 {
		t.Fatalf("version=%q arches=%v", p.Version, p.Arches())
	}
	arm, _ := p.Bin("arm64")
	amd, _ := p.Bin("amd64")
	if arm.SHA256 == amd.SHA256 || arm.SHA256 == "" {
		t.Fatalf("expected distinct per-arch sha: %s / %s", arm.SHA256, amd.SHA256)
	}
	// Alias spellings normalize.
	if b, ok := p.Bin("aarch64"); !ok || b.Arch != "arm64" {
		t.Fatalf("aarch64 -> %+v ok=%v", b, ok)
	}
	if b, ok := p.Bin("x86_64"); !ok || b.Arch != "amd64" {
		t.Fatalf("x86_64 -> %+v ok=%v", b, ok)
	}
	// Empty resolves to the default arch; unknown arch is a miss.
	if b, ok := p.Bin(""); !ok || b.Arch != DefaultArch {
		t.Fatalf("default -> %+v ok=%v", b, ok)
	}
	if _, ok := p.Bin("mips"); ok {
		t.Fatal("mips should not resolve")
	}
	// Back-compat mirror points at the default arch.
	if p.SHA256 != arm.SHA256 {
		t.Fatalf("default mirror sha %s != arm %s", p.SHA256, arm.SHA256)
	}
}
