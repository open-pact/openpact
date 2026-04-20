package admin

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/open-pact/openpact/internal/ratelimit"
)

// LoginRateLimiter returns a middleware that per-remote-IP rate-limits
// requests to the handler it wraps. Intended for the brute-force
// targets — /api/auth/login and /api/setup — where a slow attacker
// otherwise gets unlimited password guesses.
//
// The limiter's rate/burst come from advanced_settings.rate_limit.* in
// op_kv. Defaults (10 rps, 20 burst) let a human at a real keyboard
// through comfortably while a scripted attacker gets 429'd.
//
// Cleanup of per-IP buckets runs on the KeyedLimiter's own hourly
// schedule — the middleware doesn't have to manage that.
func LoginRateLimiter(cfg ratelimit.Config) func(http.Handler) http.Handler {
	limiter := ratelimit.NewKeyed(cfg)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := remoteIP(r)
			if !limiter.Allow(ip) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":   "rate_limited",
					"message": "Too many attempts. Wait a moment and try again.",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// remoteIP extracts the client IP for rate-limiting purposes.
// X-Forwarded-For first entry wins when the admin UI sits behind a
// reverse proxy; otherwise r.RemoteAddr's host. A malicious client
// can spoof XFF, but the worst they can do is get themselves
// rate-limited under a fake key — not the real abuse vector.
func remoteIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i != -1 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
