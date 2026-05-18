// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Milos Vasic

package i18n_test

import (
	"context"
	"testing"

	"digital.vasic.middleware/pkg/i18n"
)

// TestNoopTranslator_T_ReturnsMsgIDVerbatim asserts that the
// stripped-down fallback Translator emits the message ID unchanged.
// Per CONST-035 / Article XI §11.9 this verbatim-fallback is itself
// positive runtime evidence — operators see exactly which key was
// resolved without a bundle.
func TestNoopTranslator_T_ReturnsMsgIDVerbatim(t *testing.T) {
	tr := i18n.NoopTranslator{}
	got := tr.T(context.Background(), "middleware_auth_unauthorized", map[string]any{
		"realm": "test",
	})
	const want = "middleware_auth_unauthorized"
	if got != want {
		t.Fatalf("NoopTranslator.T mismatch:\n got = %q\nwant = %q", got, want)
	}
}

// TestNoopTranslator_TPlural_ReturnsMsgIDVerbatim mirrors the T
// assertion for plural-form lookups.
func TestNoopTranslator_TPlural_ReturnsMsgIDVerbatim(t *testing.T) {
	tr := i18n.NoopTranslator{}
	got := tr.TPlural(context.Background(), "middleware_ratelimit_too_many_requests", 3, nil)
	const want = "middleware_ratelimit_too_many_requests"
	if got != want {
		t.Fatalf("NoopTranslator.TPlural mismatch:\n got = %q\nwant = %q", got, want)
	}
}

// TestNoopTranslator_T_NilArgs_ReturnsMsgIDVerbatim ensures the noop
// implementation tolerates nil arg maps without panic — important for
// call-sites that have no template substitutions (e.g. the
// validation middleware's 415 body).
func TestNoopTranslator_T_NilArgs_ReturnsMsgIDVerbatim(t *testing.T) {
	tr := i18n.NoopTranslator{}
	got := tr.T(context.Background(), "middleware_validation_unsupported_media_type", nil)
	const want = "middleware_validation_unsupported_media_type"
	if got != want {
		t.Fatalf("NoopTranslator.T(nil args) mismatch:\n got = %q\nwant = %q", got, want)
	}
}
