// Package ratelimit provides token bucket rate limiting.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter implements a token bucket rate limiter
type Limiter struct {
	mu         sync.Mutex
	rate       float64   // tokens per second
	burst      int       // max tokens
	tokens     float64   // current tokens
	lastUpdate time.Time // last token update
}

// Config configures a rate limiter
type Config struct {
	Rate  float64 // requests per second
	Burst int     // maximum burst size
}

// New creates a new rate limiter
func New(cfg Config) *Limiter {
	if cfg.Rate <= 0 {
		cfg.Rate = 1.0
	}
	if cfg.Burst <= 0 {
		cfg.Burst = 1
	}

	return &Limiter{
		rate:       cfg.Rate,
		burst:      cfg.Burst,
		tokens:     float64(cfg.Burst),
		lastUpdate: time.Now(),
	}
}

// Allow checks if a request is allowed and consumes a token if so
func (l *Limiter) Allow() bool {
	return l.AllowN(1)
}

// AllowN checks if n requests are allowed and consumes n tokens if so
func (l *Limiter) AllowN(n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.refill(now)

	if l.tokens >= float64(n) {
		l.tokens -= float64(n)
		return true
	}

	return false
}

// refill adds tokens based on elapsed time (must hold lock)
func (l *Limiter) refill(now time.Time) {
	elapsed := now.Sub(l.lastUpdate).Seconds()
	l.tokens += elapsed * l.rate

	// Cap at burst limit
	if l.tokens > float64(l.burst) {
		l.tokens = float64(l.burst)
	}

	l.lastUpdate = now
}

// Wait blocks until a token is available or context expires
func (l *Limiter) Wait() {
	for !l.Allow() {
		time.Sleep(time.Millisecond * 10)
	}
}

// Reserve returns how long to wait for n tokens (does not consume)
func (l *Limiter) Reserve(n int) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.refill(now)

	if l.tokens >= float64(n) {
		return 0
	}

	// Calculate wait time for needed tokens
	needed := float64(n) - l.tokens
	waitSeconds := needed / l.rate

	return time.Duration(waitSeconds * float64(time.Second))
}

// Tokens returns current available tokens
func (l *Limiter) Tokens() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill(time.Now())
	return l.tokens
}

// KeyedLimiter provides per-key rate limiting
type KeyedLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*Limiter
	config   Config
	cleanup  time.Duration
}

// NewKeyed creates a keyed rate limiter (e.g., per-user)
func NewKeyed(cfg Config) *KeyedLimiter {
	kl := &KeyedLimiter{
		limiters: make(map[string]*Limiter),
		config:   cfg,
		cleanup:  time.Hour, // cleanup old limiters hourly
	}

	go kl.cleanupLoop()

	return kl
}

// Allow checks if a request for the given key is allowed
func (kl *KeyedLimiter) Allow(key string) bool {
	return kl.AllowN(key, 1)
}

// AllowN checks if n requests for the given key are allowed
func (kl *KeyedLimiter) AllowN(key string, n int) bool {
	limiter := kl.getLimiter(key)
	return limiter.AllowN(n)
}

// getLimiter gets or creates a limiter for a key
func (kl *KeyedLimiter) getLimiter(key string) *Limiter {
	kl.mu.RLock()
	limiter, exists := kl.limiters[key]
	kl.mu.RUnlock()

	if exists {
		return limiter
	}

	kl.mu.Lock()
	defer kl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists = kl.limiters[key]; exists {
		return limiter
	}

	limiter = New(kl.config)
	kl.limiters[key] = limiter
	return limiter
}

// cleanupLoop periodically removes inactive limiters
func (kl *KeyedLimiter) cleanupLoop() {
	ticker := time.NewTicker(kl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		kl.mu.Lock()
		// Remove limiters that are at full capacity (inactive)
		for key, limiter := range kl.limiters {
			if limiter.Tokens() >= float64(kl.config.Burst) {
				delete(kl.limiters, key)
			}
		}
		kl.mu.Unlock()
	}
}

// Count returns the number of active limiters
func (kl *KeyedLimiter) Count() int {
	kl.mu.RLock()
	defer kl.mu.RUnlock()
	return len(kl.limiters)
}
