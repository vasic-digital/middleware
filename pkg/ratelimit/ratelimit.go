// Package ratelimit provides token-bucket rate limiting middleware for net/http.
//
// It limits requests per client (by IP or custom key) using an in-memory
// token bucket. Requests exceeding the limit receive a 429 Too Many Requests
// response with a Retry-After header.
//
// Design pattern: Strategy (key extraction is injected).
package ratelimit

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"digital.vasic.middleware/pkg/i18n"
)

// KeyFunc extracts a rate-limiting key from the request (e.g., client IP).
type KeyFunc func(r *http.Request) string

// Config holds rate limiter configuration.
type Config struct {
	// Rate is the number of requests allowed per window.
	Rate int
	// Window is the time window for the rate limit.
	Window time.Duration
	// KeyFunc extracts the rate-limiting key from the request.
	// Defaults to client IP from RemoteAddr.
	KeyFunc KeyFunc
	// Translator sources the 429 response body per CONST-046.
	// Defaults to i18n.NoopTranslator{} (returns message ID verbatim,
	// preserving anti-bluff visibility in captured wire traffic).
	Translator i18n.Translator
}

// DefaultConfig returns a default rate limiter configuration
// allowing 100 requests per minute per IP.
func DefaultConfig() *Config {
	return &Config{
		Rate:   100,
		Window: time.Minute,
		KeyFunc: func(r *http.Request) string {
			return r.RemoteAddr
		},
	}
}

type bucket struct {
	tokens    int
	lastReset time.Time
}

type limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    int
	window  time.Duration
	keyFunc KeyFunc
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[key]
	if !ok || now.Sub(b.lastReset) >= l.window {
		l.buckets[key] = &bucket{tokens: l.rate - 1, lastReset: now}
		return true
	}

	if b.tokens > 0 {
		b.tokens--
		return true
	}
	return false
}

func (l *limiter) retryAfter(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		return 0
	}
	remaining := l.window - time.Since(b.lastReset)
	if remaining <= 0 {
		return 0
	}
	return int(remaining.Seconds()) + 1
}

// New creates a rate limiting middleware with the given configuration.
func New(cfg *Config) func(http.Handler) http.Handler {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.Rate <= 0 {
		cfg.Rate = 100
	}
	if cfg.Window <= 0 {
		cfg.Window = time.Minute
	}
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = func(r *http.Request) string {
			return r.RemoteAddr
		}
	}
	if cfg.Translator == nil {
		cfg.Translator = i18n.NoopTranslator{}
	}

	l := &limiter{
		buckets: make(map[string]*bucket),
		rate:    cfg.Rate,
		window:  cfg.Window,
		keyFunc: cfg.KeyFunc,
	}
	translator := cfg.Translator

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := l.keyFunc(r)
			if !l.allow(key) {
				retry := l.retryAfter(key)
				w.Header().Set("Retry-After", strconv.Itoa(retry))
				body := translator.T(r.Context(), "middleware_ratelimit_too_many_requests", nil)
				http.Error(w, body, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
