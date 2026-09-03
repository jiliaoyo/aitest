package auth

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestClientIPOnlyTrustsConfiguredProxy(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/login", nil)
	r.RemoteAddr = "192.0.2.10:1234"
	r.Header.Set("X-Forwarded-For", "198.51.100.20, 198.51.100.21")

	if got := clientIP(r, nil); got != r.RemoteAddr {
		t.Fatalf("untrusted proxy should use RemoteAddr, got %q", got)
	}
	trusted := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	if got := clientIP(r, trusted); got != "198.51.100.20" {
		t.Fatalf("trusted proxy should use first forwarded IP, got %q", got)
	}
	r.Header.Set("X-Forwarded-For", "not-an-ip, 198.51.100.22")
	if got := clientIP(r, trusted); got != "198.51.100.22" {
		t.Fatalf("should skip invalid forwarded IP, got %q", got)
	}
	r.RemoteAddr = "203.0.113.10:1234"
	if got := clientIP(r, trusted); got != r.RemoteAddr {
		t.Fatalf("unmatched proxy should use RemoteAddr, got %q", got)
	}
}

func TestValidatePasswordRejectsBcryptTooLongPassword(t *testing.T) {
	if err := validatePassword(strings.Repeat("a", 73)); err == nil || err.Code != "validation_failed" {
		t.Fatal("expected field validation for a password over 72 bytes")
	}
	if err := validatePassword("short"); err == nil || err.Code != "validation_failed" {
		t.Fatal("expected field validation for a short password")
	}
}
