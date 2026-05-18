// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Milos Vasic

package auth_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"digital.vasic.middleware/pkg/auth"
)

// stubValidator always rejects so the unauthenticated 401 path fires
// and we can capture the response body the end user receives.
type stubValidator struct{}

func (stubValidator) ValidateToken(string) (auth.Claims, error) {
	return nil, errInvalid{}
}

type errInvalid struct{}

func (errInvalid) Error() string { return "invalid" }

// TestAuth_DefaultTranslator_EmitsMsgIDInBody asserts that without
// an explicit WithTranslator(...) the default NoopTranslator emits
// the namespaced message ID verbatim in the 401 response body.
// Per CONST-035 / Article XI §11.9 the message-ID-as-body is itself
// positive runtime evidence — operators see exactly which i18n key
// the middleware tried to resolve when no bundle is wired.
func TestAuth_DefaultTranslator_EmitsMsgIDInBody(t *testing.T) {
	mw := auth.Middleware(stubValidator{})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream handler must not run on auth failure")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	body := strings.TrimSpace(rec.Body.String())
	const want = "middleware_auth_unauthorized"
	if body != want {
		t.Fatalf("body: got %q, want %q (noop fallback must emit msgID verbatim)", body, want)
	}
}
