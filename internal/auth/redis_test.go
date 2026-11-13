package auth

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	server := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: server.Addr()})
}

func TestRedisCallLimiterIsShared(t *testing.T) {
	client := newTestRedis(t)
	first := NewRedisCallLimiter(client, time.Minute)
	second := NewRedisCallLimiter(client, time.Minute)

	release, ok := first.Acquire("account-1", false)
	if !ok {
		t.Fatal("first acquire failed")
	}
	if _, ok := second.Acquire("account-1", false); ok {
		t.Fatal("second process acquired the same account")
	}
	release()
	if _, ok := second.Acquire("account-1", false); !ok {
		t.Fatal("account was not released")
	}
	// Owners are not limited.
	if _, ok := first.Acquire("owner", true); !ok {
		t.Fatal("owner acquire failed")
	}
	if _, ok := first.Acquire("owner", true); !ok {
		t.Fatal("owner should allow concurrent calls")
	}
}

func TestRedisAttemptLimiter(t *testing.T) {
	client := newTestRedis(t)
	limiter := NewRedisAttemptLimiter(client, 3, time.Minute)
	if !limiter.Allow("ip|email") {
		t.Fatal("first attempt should be allowed")
	}
	for i := 0; i < 3; i++ {
		limiter.Fail("ip|email")
	}
	if limiter.Allow("ip|email") {
		t.Fatal("attempt limit was not enforced")
	}
	limiter.Reset("ip|email")
	if !limiter.Allow("ip|email") {
		t.Fatal("reset did not clear attempts")
	}
}
