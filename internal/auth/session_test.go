package auth

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPIgnoresSpoofedForwardedFor(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.9:54321"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := clientIP(req); got != "203.0.113.9" {
		t.Fatalf("clientIP = %q", got)
	}
}

func TestClientIPTrustsConfiguredProxy(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.1")
	if got := clientIP(req); got != "1.2.3.4" {
		t.Fatalf("clientIP = %q", got)
	}
}
