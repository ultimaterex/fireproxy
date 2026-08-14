package svclogs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestACLFileReaderNotable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "acl.log")
	content := "noise query ok\n" +
		"foo [ACL][Blocked] bad.example\n" +
		"another info line\n" +
		"bar [ACL][Blocked] worse.example\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &ACLFileReader{Path: path}
	lines, off, err := r.ReadNotable(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("lines=%d %+v", len(lines), lines)
	}
	if lines[0].Source != "dnsmasq" {
		t.Fatalf("source %q", lines[0].Source)
	}
	if off != int64(len(content)) {
		t.Fatalf("offset %d want %d", off, len(content))
	}
	// Second read: no new lines
	r.Offset = off
	lines2, off2, err := r.ReadNotable(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines2) != 0 || off2 != off {
		t.Fatalf("second read %+v off=%d", lines2, off2)
	}
}

func TestACLFileReaderRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "acl.log")
	if err := os.WriteFile(path, []byte("old [ACL][Blocked] a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &ACLFileReader{Path: path, Offset: 1000} // past EOF → treat as rotate
	lines, _, err := r.ReadNotable(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 {
		t.Fatalf("after rotate got %d", len(lines))
	}
}
