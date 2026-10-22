package rate

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTokenBucketAllow(t *testing.T) {
	b := NewTokenBucket(1.0, 3)
	// burst capacity = 3, first 3 allowed
	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	// 4th immediately should be denied
	if b.Allow() {
		t.Fatal("4th request should be rate limited")
	}
}

func TestSetRateLimitHeaders(t *testing.T) {
	rr := httptest.NewRecorder()
	SetRateLimitHeaders(rr, 100, 3600*time.Second)
	h := rr.Header()
	if h.Get("X-RateLimit-Limit") != "100" {
		t.Fatalf("limit header = %q", h.Get("X-RateLimit-Limit"))
	}
	if h.Get("X-RateLimit-Remaining") != "99" {
		t.Fatalf("remaining header = %q", h.Get("X-RateLimit-Remaining"))
	}
	if h.Get("Retry-After") != "3600" {
		t.Fatalf("retry-after header = %q", h.Get("Retry-After"))
	}
	if h.Get("X-RateLimit-Reset") == "" {
		t.Fatal("reset header empty")
	}
}

// TestRateKeyFunctionsIsolated: each key function must produce a distinct
// Redis bucket so per-user and per-key limits are independent.
// Regression for APIKeyRateKey returning a shared constant.
func TestRateKeyFunctionsIsolated(t *testing.T) {
	if got := APIKeyRateKey("key-123"); !strings.Contains(got, "key-123") || !strings.HasPrefix(got, "rl:apikey:") {
		t.Fatalf("APIKeyRateKey must embed the key id, got %q", got)
	}
	if got := UserRateKey("user-abc"); got != "rl:user:user-abc" {
		t.Fatalf("UserRateKey = %q", got)
	}
	if got := IPRateKey("10.0.0.1"); got != "rl:ip:10.0.0.1" {
		t.Fatalf("IPRateKey = %q", got)
	}
	if APIKeyRateKey("a") == APIKeyRateKey("b") {
		t.Fatal("APIKeyRateKey must be unique per key")
	}
	if APIKeyRateKey("a") == UserRateKey("a") {
		t.Fatal("apikey and user buckets must not collide")
	}
}
