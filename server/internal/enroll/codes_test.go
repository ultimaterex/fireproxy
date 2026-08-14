package enroll

import (
	"testing"
	"time"
)

func TestMintExchangeOneUse(t *testing.T) {
	s := &CodeStore{}
	now := time.Unix(1_700_000_000, 0)
	code, exp, err := s.Mint("http://fp.lan:8081", now, time.Minute)
	if err != nil || code == "" || exp.Before(now) {
		t.Fatalf("%q %v %v", code, exp, err)
	}
	base, err := s.Exchange(code, now.Add(30*time.Second))
	if err != nil || base != "http://fp.lan:8081" {
		t.Fatalf("%q %v", base, err)
	}
	if _, err := s.Exchange(code, now.Add(30*time.Second)); err == nil {
		t.Fatal("expected second exchange fail")
	}
}

func TestMintInvalidatesPrior(t *testing.T) {
	s := &CodeStore{}
	now := time.Now()
	c1, _, err := s.Mint("http://a", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	c2, _, err := s.Mint("http://b", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Exchange(c1, now); err == nil {
		t.Fatal("old code should be invalid")
	}
	base, err := s.Exchange(c2, now)
	if err != nil || base != "http://b" {
		t.Fatalf("%q %v", base, err)
	}
}

func TestExchangeExpired(t *testing.T) {
	s := &CodeStore{}
	now := time.Unix(100, 0)
	code, _, err := s.Mint("http://x", now, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Exchange(code, now.Add(2*time.Second)); err == nil {
		t.Fatal("expected expired")
	}
}

func TestMintRequiresURL(t *testing.T) {
	s := &CodeStore{}
	if _, _, err := s.Mint("  ", time.Now(), 0); err == nil {
		t.Fatal("expected error")
	}
}
