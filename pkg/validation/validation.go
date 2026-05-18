// Package validation provides input validation and sanitization middleware
// for net/http.
//
// It enforces request size limits, content-type requirements, and provides
// common sanitization helpers for preventing injection attacks.
//
// Design pattern: Chain of Responsibility (validation checks run in sequence).
package validation

import (
	"net/http"
	"strings"

	"digital.vasic.middleware/pkg/i18n"
)

// Config holds validation middleware configuration.
type Config struct {
	// MaxBodySize is the maximum allowed request body size in bytes.
	// 0 means no limit.
	MaxBodySize int64
	// RequireContentType enforces that requests with a body must have
	// one of the specified Content-Type values.
	RequireContentType []string
	// Methods that require content-type validation.
	// Defaults to POST, PUT, PATCH.
	BodyMethods []string
	// Translator sources the 415 response body per CONST-046.
	// Defaults to i18n.NoopTranslator{} (returns message ID verbatim,
	// preserving anti-bluff visibility in captured wire traffic).
	Translator i18n.Translator
}

// DefaultConfig returns a default validation configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxBodySize:        10 * 1024 * 1024, // 10 MB
		RequireContentType: []string{"application/json"},
		BodyMethods:        []string{"POST", "PUT", "PATCH"},
	}
}

// New creates a validation middleware with the given configuration.
func New(cfg *Config) func(http.Handler) http.Handler {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if len(cfg.BodyMethods) == 0 {
		cfg.BodyMethods = []string{"POST", "PUT", "PATCH"}
	}
	if cfg.Translator == nil {
		cfg.Translator = i18n.NoopTranslator{}
	}

	bodyMethodSet := make(map[string]bool, len(cfg.BodyMethods))
	for _, m := range cfg.BodyMethods {
		bodyMethodSet[m] = true
	}
	translator := cfg.Translator

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.MaxBodySize > 0 && bodyMethodSet[r.Method] {
				r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxBodySize)
			}

			if len(cfg.RequireContentType) > 0 && bodyMethodSet[r.Method] {
				ct := r.Header.Get("Content-Type")
				if ct == "" || !matchesContentType(ct, cfg.RequireContentType) {
					body := translator.T(r.Context(), "middleware_validation_unsupported_media_type", nil)
					http.Error(w, body, http.StatusUnsupportedMediaType)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func matchesContentType(ct string, allowed []string) bool {
	// Normalize: "application/json; charset=utf-8" -> "application/json"
	ct = strings.TrimSpace(strings.SplitN(ct, ";", 2)[0])
	ct = strings.ToLower(ct)
	for _, a := range allowed {
		if strings.ToLower(a) == ct {
			return true
		}
	}
	return false
}

// SanitizeString removes null bytes and trims whitespace from a string.
func SanitizeString(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	return strings.TrimSpace(s)
}

// SanitizeHeader removes newlines from header values to prevent header injection.
func SanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}
