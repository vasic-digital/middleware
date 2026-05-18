// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Milos Vasic

package validation_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"digital.vasic.middleware/pkg/validation"
)

// TestValidation_DefaultTranslator_EmitsMsgIDInBody asserts that
// without an explicit Translator on Config the default NoopTranslator
// emits the namespaced message ID verbatim in the 415 response body.
// Per CONST-035 / Article XI §11.9 the message-ID-as-body is itself
// positive runtime evidence.
func TestValidation_DefaultTranslator_EmitsMsgIDInBody(t *testing.T) {
	cfg := validation.DefaultConfig()
	mw := validation.New(cfg)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream handler must not run on content-type rejection")
	}))

	// POST with text/plain triggers the 415 branch (default config
	// requires application/json).
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader([]byte("hello")))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
	body := strings.TrimSpace(rec.Body.String())
	const want = "middleware_validation_unsupported_media_type"
	if body != want {
		t.Fatalf("body: got %q, want %q (noop fallback must emit msgID verbatim)", body, want)
	}
}
