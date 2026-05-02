package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRetryAfter_UnknownKey covers the retryAfter method when the key is not
// found in the buckets map (the !ok branch returning 0).
func TestRetryAfter_UnknownKey(t *testing.T) {
	l := &limiter{
		buckets: make(map[string]*bucket),
		rate:    5,
		window:  time.Minute,
		keyFunc: func(r *http.Request) string { return r.RemoteAddr },
	}

	result := l.retryAfter("nonexistent-key")
	assert.Equal(t, 0, result)
}

// TestRetryAfter_ExpiredWindow covers the retryAfter method when the bucket
// exists but the window has expired (remaining <= 0 branch).
func TestRetryAfter_ExpiredWindow(t *testing.T) {
	l := &limiter{
		buckets: make(map[string]*bucket),
		rate:    1,
		window:  10 * time.Millisecond,
		keyFunc: func(r *http.Request) string { return r.RemoteAddr },
	}

	// Create a bucket with a past lastReset so the window has expired.
	l.buckets["expired-key"] = &bucket{
		tokens:    0,
		lastReset: time.Now().Add(-time.Second),
	}

	result := l.retryAfter("expired-key")
	assert.Equal(t, 0, result)
}

// TestRetryAfter_ActiveWindow covers the retryAfter method when the bucket
// exists and the window is still active, returning a positive retry-after.
func TestRetryAfter_ActiveWindow(t *testing.T) {
	l := &limiter{
		buckets: make(map[string]*bucket),
		rate:    1,
		window:  time.Minute,
		keyFunc: func(r *http.Request) string { return r.RemoteAddr },
	}

	l.buckets["active-key"] = &bucket{
		tokens:    0,
		lastReset: time.Now(),
	}

	result := l.retryAfter("active-key")
	assert.Greater(t, result, 0)
}

// TestNew_NegativeRate covers the cfg.Rate <= 0 branch, defaulting to 100.
func TestNew_NegativeRate(t *testing.T) {
	mw := New(&Config{Rate: -5, Window: time.Minute})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestNew_ZeroRate covers the cfg.Rate <= 0 branch with zero value.
func TestNew_ZeroRate(t *testing.T) {
	mw := New(&Config{Rate: 0, Window: time.Minute})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestNew_NegativeWindow covers the cfg.Window <= 0 branch, defaulting to
// time.Minute.
func TestNew_NegativeWindow(t *testing.T) {
	mw := New(&Config{Rate: 5, Window: -time.Second})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.3:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestNew_ZeroWindow covers the cfg.Window <= 0 branch with zero duration.
func TestNew_ZeroWindow(t *testing.T) {
	mw := New(&Config{Rate: 5, Window: 0})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.4:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestNew_NilKeyFunc covers the cfg.KeyFunc == nil branch, defaulting to
// RemoteAddr extraction.
func TestNew_NilKeyFunc(t *testing.T) {
	mw := New(&Config{Rate: 2, Window: time.Minute, KeyFunc: nil})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Two requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.0.1:9999"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	}

	// Third request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.0.1:9999"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

// TestAllow_BucketReset covers the bucket reset path in allow() when the
// window has expired since lastReset.
func TestAllow_BucketReset(t *testing.T) {
	l := &limiter{
		buckets: make(map[string]*bucket),
		rate:    1,
		window:  10 * time.Millisecond,
		keyFunc: func(r *http.Request) string { return r.RemoteAddr },
	}

	// First request creates the bucket.
	assert.True(t, l.allow("key1"))
	// Second request: bucket exists but no tokens.
	assert.False(t, l.allow("key1"))
	// Wait for window to expire.
	time.Sleep(15 * time.Millisecond)
	// Should succeed again after reset.
	assert.True(t, l.allow("key1"))
}

// TestAllow_TokenDecrement covers the token decrement path in allow().
func TestAllow_TokenDecrement(t *testing.T) {
	l := &limiter{
		buckets: make(map[string]*bucket),
		rate:    3,
		window:  time.Minute,
		keyFunc: func(r *http.Request) string { return r.RemoteAddr },
	}

	// All 3 requests should succeed (rate=3, first creates with tokens=2).
	assert.True(t, l.allow("key2"))
	assert.True(t, l.allow("key2"))
	assert.True(t, l.allow("key2"))
	// Fourth should fail.
	assert.False(t, l.allow("key2"))
}

// TestNew_ConcurrentAccess verifies that the rate limiter is safe for
// concurrent access.
// TestRetryAfter_HeaderValue verifies the actual Retry-After header value
// returned when a request is rate limited.
func TestRetryAfter_HeaderValue(t *testing.T) {
	mw := New(&Config{Rate: 1, Window: 30 * time.Second})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.5:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Second request: rate limited
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	assert.Equal(t, http.StatusTooManyRequests, rec2.Code)

	retryAfter := rec2.Header().Get("Retry-After")
	require.NotEmpty(t, retryAfter)
}

// TestDefaultConfig_KeyFunc verifies that DefaultConfig's KeyFunc extracts
// RemoteAddr.
func TestDefaultConfig_KeyFunc_ExtractsRemoteAddr(t *testing.T) {
	cfg := DefaultConfig()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	key := cfg.KeyFunc(req)
	assert.Equal(t, "1.2.3.4:5678", key)
}
