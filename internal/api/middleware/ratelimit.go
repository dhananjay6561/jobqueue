// Package middleware — ratelimit.go implements per-IP token-bucket rate limiting
// using golang.org/x/time/rate. Each unique remote IP gets its own Limiter.
// Old limiters are evicted after a configurable TTL to prevent unbounded growth.
package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ipLimiter pairs a rate.Limiter with the last-seen timestamp for eviction.
type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter stores per-IP limiters and evicts stale entries.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
	rps      rate.Limit
	burst    int
	ttl      time.Duration
}

// NewRateLimiter creates a RateLimiter and starts a background goroutine that
// evicts stale entries every ttl duration.
//
// rps is the sustained request rate per IP; burst is the token bucket capacity.
func NewRateLimiter(rps, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*ipLimiter),
		rps:      rate.Limit(rps),
		burst:    burst,
		ttl:      5 * time.Minute,
	}

	go rl.evictLoop()
	return rl
}

// getLimiter returns the rate.Limiter for the given IP, creating one if needed.
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, ok := rl.limiters[ip]
	if !ok {
		entry = &ipLimiter{
			limiter: rate.NewLimiter(rl.rps, rl.burst),
		}
		rl.limiters[ip] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

// evictLoop removes limiters that have not been used within the TTL window.
func (rl *RateLimiter) evictLoop() {
	ticker := time.NewTicker(rl.ttl)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-rl.ttl)

		rl.mu.Lock()
		for ip, entry := range rl.limiters {
			if entry.lastSeen.Before(cutoff) {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware returns an http.Handler that enforces per-IP rate limits.
// Requests that exceed the limit receive a 429 Too Many Requests response.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			// Fallback: use the full RemoteAddr if it cannot be parsed.
			ip = r.RemoteAddr
		}

		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"rate limit exceeded","data":null}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
