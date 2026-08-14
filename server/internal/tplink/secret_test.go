package tplink

import (
	"testing"
)

const testSecretsKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestKeyFromEnvHexOK(t *testing.T) {
	key, err := KeyFromEnv(testSecretsKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 32 {
		t.Fatalf("want 32-byte key, got len=%d", len(key))
	}
}

func TestKeyFromEnvEmpty(t *testing.T) {
	if _, err := KeyFromEnv(""); err != ErrNoSecretsKey {
		t.Fatalf("want ErrNoSecretsKey, got %v", err)
	}
	if _, err := KeyFromEnv("   "); err != ErrNoSecretsKey {
		t.Fatalf("want ErrNoSecretsKey for whitespace, got %v", err)
	}
}

func TestKeyFromEnvBad(t *testing.T) {
	cases := []string{
		"test-secret-key",
		"short",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde",   // 63 hex
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0", // 65 hex
		"gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg",  // 64 non-hex
	}
	for _, in := range cases {
		if _, err := KeyFromEnv(in); err != ErrBadSecretsKey {
			t.Fatalf("input %q: want ErrBadSecretsKey, got %v", in, err)
		}
	}
}

func TestSecretRoundTrip(t *testing.T) {
	key, err := KeyFromEnv(testSecretsKey)
	if err != nil {
		t.Fatal(err)
	}
	ct, err := Encrypt(key, "ripjaws")
	if err != nil {
		t.Fatal(err)
	}
	if ct == "" || ct == "ripjaws" {
		t.Fatalf("ciphertext looks wrong: %q", ct)
	}
	pt, err := Decrypt(key, ct)
	if err != nil {
		t.Fatal(err)
	}
	if pt != "ripjaws" {
		t.Fatalf("got %q", pt)
	}
}

func TestSecretEmptyKey(t *testing.T) {
	if _, err := KeyFromEnv(""); err != ErrNoSecretsKey {
		t.Fatalf("want ErrNoSecretsKey, got %v", err)
	}
}

func TestSecretTamper(t *testing.T) {
	key, err := KeyFromEnv(testSecretsKey)
	if err != nil {
		t.Fatal(err)
	}
	ct, err := Encrypt(key, "secret")
	if err != nil {
		t.Fatal(err)
	}
	tampered := ct[:len(ct)-1] + "A"
	if _, err := Decrypt(key, tampered); err == nil {
		t.Fatal("expected error")
	}
}

func TestSecretHexKey(t *testing.T) {
	key, err := KeyFromEnv(testSecretsKey)
	if err != nil || len(key) != 32 {
		t.Fatalf("hex key: %v len=%d", err, len(key))
	}
}
