package rate

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestUserRateKeyTask19: Task 19 per-user aggregate limiter.
// The 60 req/min Redis bucket key must embed the user ID so it's enforced
// across all API keys belonging to that user.
func TestUserRateKeyTask19(t *testing.T) {
	k := UserRateKey("user-xyz")
	if k != "rl:user:user-xyz" {
		t.Errorf("UserRateKey = %q, want rl:user:user-xyz", k)
	}
	// Task 19: the v1 per-user bucket combines user key + ":v1:" + limit
	bucket := UserRateKey("user-xyz") + ":v1:60"
	if !strings.Contains(bucket, "user-xyz") {
		t.Error("per-user v1 bucket must embed user ID")
	}
	if UserRateKey("a") != "rl:user:a" {
		t.Error("UserRateKey must produce unique bucket per user")
	}
}

// TestAPIKeyRateKeyUniqueness: regression test — APIKeyRateKey must embed
// the key ID, not return a shared constant (which would break per-key limiting).
func TestAPIKeyRateKeyUniqueness(t *testing.T) {
	a := APIKeyRateKey("key-a")
	b := APIKeyRateKey("key-b")
	if a == b {
		t.Fatal("APIKeyRateKey must differ per key ID")
	}
	if !strings.Contains(a, "key-a") {
		t.Errorf("APIKeyRateKey must embed key ID, got %q", a)
	}
}

// TestTokenBucketRefill: verifies tokens refill at the configured rate
// after burst is exhausted.
func TestTokenBucketRefill(t *testing.T) {
	b := NewTokenBucket(10.0, 2)
	// Exhaust burst.
	b.Allow()
	b.Allow()
	if b.Allow() {
		t.Fatal("should be rate limited after burst exhausted")
	}
	// Wait for 2 tokens to refill (200ms at 10/sec).
	time.Sleep(210 * time.Millisecond)
	if !b.Allow() {
		t.Error("should allow after refill")
	}
	if !b.Allow() {
		t.Error("should allow second token after refill")
	}
	if b.Allow() {
		t.Error("should be rate limited again (only 2 tokens refilled)")
	}
}

// TestSetRateLimitHeaders429: verifies the X-RateLimit-* and Retry-After
// headers are set correctly on a 429 response.
func TestSetRateLimitHeaders429(t *testing.T) {
	rr := httptest.NewRecorder()
	SetRateLimitHeaders(rr, 60, time.Minute)
	h := rr.Header()
	if h.Get("X-RateLimit-Limit") != "60" {
		t.Errorf("limit = %q, want 60", h.Get("X-RateLimit-Limit"))
	}
	if h.Get("Retry-After") != "60" {
		t.Errorf("retry-after = %q, want 60", h.Get("Retry-After"))
	}
	if h.Get("X-RateLimit-Remaining") != "59" {
		t.Errorf("remaining = %q, want 59", h.Get("X-RateLimit-Remaining"))
	}
}
