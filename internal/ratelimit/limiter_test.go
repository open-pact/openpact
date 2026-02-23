package ratelimit

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	limiter := New(Config{Rate: 10, Burst: 5})
	if limiter == nil {
		t.Fatal("New returned nil")
	}
}

func TestNewDefaults(t *testing.T) {
	limiter := New(Config{})
	if limiter.rate != 1.0 {
		t.Errorf("rate = %f, want 1.0", limiter.rate)
	}
	if limiter.burst != 1 {
		t.Errorf("burst = %d, want 1", limiter.burst)
	}
}

func TestAllowBurst(t *testing.T) {
	limiter := New(Config{Rate: 100, Burst: 5})

	// Should allow burst of 5
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Errorf("request %d should be allowed", i)
		}
	}

	// 6th request should be denied
	if limiter.Allow() {
		t.Error("request beyond burst should be denied")
	}
}

func TestAllowN(t *testing.T) {
	limiter := New(Config{Rate: 100, Burst: 10})

	// Should allow 5 at once
	if !limiter.AllowN(5) {
		t.Error("AllowN(5) should succeed with burst of 10")
	}

	// Should allow another 5
	if !limiter.AllowN(5) {
		t.Error("AllowN(5) should succeed with 5 remaining")
	}

	// Should deny 1 more
	if limiter.AllowN(1) {
		t.Error("AllowN(1) should fail with 0 remaining")
	}
}

func TestRefill(t *testing.T) {
	limiter := New(Config{Rate: 100, Burst: 10})

	// Exhaust tokens
	limiter.AllowN(10)

	// Wait for refill (100/sec = 1 token per 10ms)
	time.Sleep(50 * time.Millisecond)

	// Should have ~5 tokens now
	tokens := limiter.Tokens()
	if tokens < 3 || tokens > 7 {
		t.Errorf("tokens = %f, expected ~5", tokens)
	}
}

func TestReserve(t *testing.T) {
	limiter := New(Config{Rate: 10, Burst: 5})

	// With full bucket, no wait needed
	wait := limiter.Reserve(1)
	if wait != 0 {
		t.Errorf("wait = %v, expected 0 with full bucket", wait)
	}

	// Exhaust bucket
	limiter.AllowN(5)

	// Should need to wait for 1 token at 10/sec = 100ms
	wait = limiter.Reserve(1)
	if wait < 50*time.Millisecond || wait > 150*time.Millisecond {
		t.Errorf("wait = %v, expected ~100ms", wait)
	}
}

func TestTokens(t *testing.T) {
	limiter := New(Config{Rate: 10, Burst: 5})

	if limiter.Tokens() != 5 {
		t.Errorf("initial tokens = %f, expected 5", limiter.Tokens())
	}

	limiter.Allow()
	tokens := limiter.Tokens()
	// Allow for small refill between calls (floating point)
	if tokens < 3.9 || tokens > 4.1 {
		t.Errorf("after Allow(), tokens = %f, expected ~4", tokens)
	}
}

func TestKeyedLimiter(t *testing.T) {
	kl := NewKeyed(Config{Rate: 100, Burst: 5})

	// Different keys should have independent limits
	for i := 0; i < 5; i++ {
		if !kl.Allow("user1") {
			t.Errorf("user1 request %d should be allowed", i)
		}
		if !kl.Allow("user2") {
			t.Errorf("user2 request %d should be allowed", i)
		}
	}

	// Both should be exhausted now
	if kl.Allow("user1") {
		t.Error("user1 should be rate limited")
	}
	if kl.Allow("user2") {
		t.Error("user2 should be rate limited")
	}
}

func TestKeyedLimiterCount(t *testing.T) {
	kl := NewKeyed(Config{Rate: 10, Burst: 5})

	kl.Allow("a")
	kl.Allow("b")
	kl.Allow("c")

	if kl.Count() != 3 {
		t.Errorf("count = %d, expected 3", kl.Count())
	}
}

func TestKeyedLimiterConcurrent(t *testing.T) {
	kl := NewKeyed(Config{Rate: 1000, Burst: 10})

	done := make(chan struct{})

	// Concurrent access from multiple goroutines
	for i := 0; i < 10; i++ {
		go func(key string) {
			for j := 0; j < 10; j++ {
				kl.Allow(key)
			}
			done <- struct{}{}
		}(string(rune('a' + i)))
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have 10 different limiters
	if kl.Count() != 10 {
		t.Errorf("count = %d, expected 10", kl.Count())
	}
}

func TestTokensCap(t *testing.T) {
	limiter := New(Config{Rate: 1000, Burst: 5})

	// Wait to ensure tokens would exceed burst if not capped
	time.Sleep(10 * time.Millisecond)

	tokens := limiter.Tokens()
	if tokens > 5 {
		t.Errorf("tokens = %f, should not exceed burst of 5", tokens)
	}
}
