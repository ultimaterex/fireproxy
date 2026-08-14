package auth_test

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"fireproxy/server/internal/auth"
)

func TestCookieSecureFlags(t *testing.T) {
	cfg := auth.Config{}
	r := httptest.NewRequest(http.MethodGet, "http://example/", nil)
	if auth.CookieSecure(r, cfg) {
		t.Fatal("plain HTTP should not be Secure")
	}

	r = httptest.NewRequest(http.MethodGet, "https://example/", nil)
	r.TLS = &tls.ConnectionState{}
	if !auth.CookieSecure(r, cfg) {
		t.Fatal("TLS should be Secure")
	}

	r = httptest.NewRequest(http.MethodGet, "http://example/", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	if auth.CookieSecure(r, cfg) {
		t.Fatal("X-Forwarded-Proto without TrustedProxy must be ignored")
	}

	cfg.TrustedProxy = true
	if !auth.CookieSecure(r, cfg) {
		t.Fatal("TrustedProxy + X-Forwarded-Proto=https should be Secure")
	}
}
