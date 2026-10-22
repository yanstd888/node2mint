package rate

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Allow: rate limit fixed-window. Atomic via Lua script (INCR + EXPIRE).
func Allow(ctx context.Context, rdb *redis.Client, key string, limit int, window time.Duration) (bool, error) {
	script := redis.NewScript(`
		local c = redis.call('INCR', KEYS[1])
		if c == 1 then redis.call('EXPIRE', KEYS[1], ARGV[2]) end
		if c > tonumber(ARGV[1]) then return 0 end
		return 1
	`)
	res, err := script.Run(ctx, rdb, []string{key}, limit, int(window.Seconds())).Result()
	if err != nil {
		return false, err
	}
	return res == int64(1), nil
}

func APIKeyRateKey(a string) string { return fmt.Sprintf("rl:apikey:%s", a) }
func UserRateKey(s string) string   { return fmt.Sprintf("rl:user:%s", s) }
func IPRateKey(ip string) string    { return fmt.Sprintf("rl:ip:%s", ip) }

// SetRateLimitHeaders sets the standard X-RateLimit-* + Retry-After headers on a
// 429 response. For fixed-window Redis limits, remaining is computed as
// limit-1 (this request consumed one token) and reset is the window end.
func SetRateLimitHeaders(w http.ResponseWriter, limit int, window time.Duration) {
	now := time.Now()
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, limit-1)))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", now.Add(window).Unix()))
	w.Header().Set("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
}

// TokenBucket: stdlib-only in-memory token bucket (1 r/s default).
// Thread-safe via sync.Mutex. Global per-instance lock — sufficient for a
// small fixed set of hot-path public endpoints fronted by Redis for the rest.
// ponytail: global lock per bucket; if per-IP fan-out scales, split into
// per-key buckets with a cleanup sweeper.
type TokenBucket struct {
	mu     sync.Mutex
	rate   float64 // tokens/sec
	burst  int     // max capacity
	tokens float64
	last   time.Time
}

// NewTokenBucket creates a bucket refilling at rate/sec with burst tokens.
func NewTokenBucket(ratePerSec float64, burst int) *TokenBucket {
	if ratePerSec <= 0 {
		ratePerSec = 1
	}
	if burst <= 0 {
		burst = 1
	}
	return &TokenBucket{rate: ratePerSec, burst: burst, tokens: float64(burst), last: time.Now()}
}

// Allow returns true if a token is available, else false.
func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now
	b.tokens += elapsed * b.rate
	if b.tokens > float64(b.burst) {
		b.tokens = float64(b.burst)
	}
	if b.tokens >= 1 {
		b.tokens -= 1
		return true
	}
	return false
}
