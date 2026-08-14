package auth

import (
	"strings"
	"testing"
)

func TestHashTokenVerifyRoundTrip(t *testing.T) {
	plain, err := NewToken("fp_")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(plain, "fp_") {
		t.Fatalf("prefix missing: %q", plain)
	}
	if len(plain) <= len("fp_") {
		t.Fatalf("token too short: %q", plain)
	}

	hash := HashToken(plain)
	if hash == "" {
		t.Fatal("empty hash")
	}
	if !VerifyToken(plain, hash) {
		t.Fatal("VerifyToken failed for matching token")
	}
	if VerifyToken("wrong-token", hash) {
		t.Fatal("VerifyToken succeeded for wrong token")
	}
	if VerifyToken(plain, "deadbeef") {
		t.Fatal("VerifyToken succeeded for wrong hash")
	}
}
