// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Milos Vasic

package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"digital.vasic.middleware/pkg/ratelimit"
)

// TestRatelimit_DefaultTranslator_EmitsMsgIDInBody asserts that
// without an explicit Translator on Config the default NoopTranslator
// emits the namespaced message ID verbatim in the 429 response body.
// Per CONST-035 / Article XI §11.9 the message-ID-as-body is itself
// positive runtime evidence.
func TestRatelimit_DefaultTranslator_EmitsMsgIDInBody(t *testing.T) {
	cfg := &ratelimit.Config{
		Rate:   1,
		Window: time.Minute,
		KeyFunc: func(r *http.Request) string {
			return "fixed-key"
		},
	}
	mw := ratelimit.New(cfg)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request consumes the token.
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/x", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first request status: got %d, want 200", first.Code)
	}

	// Second request hits the limit and triggers the i18n-wired body.
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/x", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status: got %d, want %d", second.Code, http.StatusTooManyRequests)
	}
	body := strings.TrimSpace(second.Body.String())
	const want = "middleware_ratelimit_too_many_requests"
	if body != want {
		t.Fatalf("body: got %q, want %q (noop fallback must emit msgID verbatim)", body, want)
	}
}
