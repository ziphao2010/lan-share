package token

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateValidRoundTrip(t *testing.T) {
	exp := time.Now().Add(time.Hour)
	tok := Generate("secret", "/big.iso", exp)
	if len(tok) != 64 {
		t.Fatalf("token length = %d, want 64 (sha256 hex)", len(tok))
	}
	if !Valid("secret", "/big.iso", tok, exp) {
		t.Error("valid token rejected")
	}
}

func TestValidRejectsWrongSecret(t *testing.T) {
	exp := time.Now().Add(time.Hour)
	tok := Generate("secret-a", "/f", exp)
	if Valid("secret-b", "/f", tok, exp) {
		t.Error("token from different secret must fail")
	}
}

func TestValidRejectsExpired(t *testing.T) {
	exp := time.Now().Add(-time.Minute)
	tok := Generate("s", "/f", exp)
	if Valid("s", "/f", tok, exp) {
		t.Error("expired token must fail")
	}
}

func TestValidRejectsTampered(t *testing.T) {
	exp := time.Now().Add(time.Hour)
	tok := Generate("s", "/f", exp)
	if Valid("s", "/g", tok, exp) { // path tampered
		t.Error("tampered path must fail")
	}
	if Valid("s", "/f", strings.Repeat("0", 64), exp) {
		t.Error("garbage token must fail")
	}
}
