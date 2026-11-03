package auth

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCallLimiterStandardAccount(t *testing.T) {
	limiter := NewCallLimiter()
	release, ok := limiter.Acquire("acct-1", false)
	if !ok {
		t.Fatal("first call should acquire")
	}
	if !limiter.Active("acct-1") {
		t.Fatal("call should be active")
	}
	if _, ok := limiter.Acquire("acct-1", false); ok {
		t.Fatal("second call should be rejected")
	}
	release()
	release()
	if limiter.Active("acct-1") {
		t.Fatal("release should clear the slot")
	}
	if _, ok := limiter.Acquire("acct-1", false); !ok {
		t.Fatal("slot should be reusable")
	}
}

func TestCallLimiterOwnerBypass(t *testing.T) {
	limiter := NewCallLimiter()
	var releases []func()
	for i := 0; i < 5; i++ {
		release, ok := limiter.Acquire("owner", true)
		if !ok {
			t.Fatal("owner should not be limited")
		}
		releases = append(releases, release)
	}
	for _, release := range releases {
		release()
	}
	if limiter.Active("owner") {
		t.Fatal("owner calls should be released")
	}
}

func TestCallLimiterRace(t *testing.T) {
	limiter := NewCallLimiter()
	start := make(chan struct{})
	var wg sync.WaitGroup
	var acquired atomic.Int32
	var release func()
	var mu sync.Mutex
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r, ok := limiter.Acquire("acct", false)
			if ok {
				acquired.Add(1)
				mu.Lock()
				release = r
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()
	if acquired.Load() != 1 {
		t.Fatalf("expected exactly one acquire, got %d", acquired.Load())
	}
	mu.Lock()
	r := release
	mu.Unlock()
	r()
	if limiter.Active("acct") {
		t.Fatal("release should clear the slot")
	}
}

func TestAttemptLimiter(t *testing.T) {
	limiter := NewAttemptLimiter(2, time.Minute)
	now := time.Unix(1000, 0)
	limiter.now = func() time.Time { return now }
	if !limiter.Allow("k") {
		t.Fatal("initial attempt should be allowed")
	}
	limiter.Fail("k")
	limiter.Fail("k")
	if limiter.Allow("k") {
		t.Fatal("third attempt should be blocked")
	}
	now = now.Add(2 * time.Minute)
	if !limiter.Allow("k") {
		t.Fatal("attempt should be allowed after window")
	}
	limiter.Fail("k")
	limiter.Reset("k")
	if !limiter.Allow("k") {
		t.Fatal("reset should clear attempts")
	}
}
