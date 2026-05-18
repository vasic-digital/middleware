// Package auth provides generic JWT authentication middleware for net/http.
//
// It defines a TokenValidator interface that consumers implement with their
// JWT library of choice. The middleware extracts the Bearer token from the
// Authorization header and validates it via the injected validator.
//
// Design pattern: Strategy (token validation strategy is injected).
package auth

import (
	"context"
	"net/http"
	"strings"

	"digital.vasic.middleware/pkg/i18n"
)

type contextKey string

const claimsKey contextKey = "auth_claims"

// Claims holds validated token claims.
type Claims map[string]interface{}

// Get retrieves a claim value by key.
func (c Claims) Get(key string) interface{} {
	return c[key]
}

// GetString retrieves a claim value as string.
func (c Claims) GetString(key string) string {
	v, ok := c[key].(string)
	if !ok {
		return ""
	}
	return v
}

// TokenValidator validates JWT tokens and returns claims.
type TokenValidator interface {
	ValidateToken(tokenString string) (Claims, error)
}

// ErrorResponder writes authentication error responses.
type ErrorResponder interface {
	RespondUnauthorized(w http.ResponseWriter, r *http.Request, err error)
}

// defaultErrorResponder writes a plain-text 401 response whose body
// is sourced from the configured Translator (CONST-046). The default
// Translator is i18n.NoopTranslator{}, which returns the msgID
// verbatim — callers wiring a real bundle/LLM-backed Translator get
// localised bodies for free without modifying this middleware.
type defaultErrorResponder struct {
	translator i18n.Translator
}

func (d defaultErrorResponder) RespondUnauthorized(w http.ResponseWriter, r *http.Request, err error) {
	body := d.translator.T(r.Context(), "middleware_auth_unauthorized", nil)
	http.Error(w, body, http.StatusUnauthorized)
}

// Middleware returns HTTP middleware that validates JWT tokens.
// Requests without a valid token receive a 401 response.
// Valid claims are stored in the request context.
func Middleware(validator TokenValidator, opts ...Option) func(http.Handler) http.Handler {
	cfg := config{
		responder:  defaultErrorResponder{translator: i18n.NoopTranslator{}},
		skipPaths:  make(map[string]bool),
		translator: i18n.NoopTranslator{},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	// If the caller wired a Translator but kept the default responder,
	// re-bind the responder to use the wired Translator. Custom
	// responders supplied via WithErrorResponder are left untouched.
	if _, ok := cfg.responder.(defaultErrorResponder); ok {
		cfg.responder = defaultErrorResponder{translator: cfg.translator}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.skipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			token := extractToken(r)
			if token == "" {
				cfg.responder.RespondUnauthorized(w, r, nil)
				return
			}

			claims, err := validator.ValidateToken(token)
			if err != nil {
				cfg.responder.RespondUnauthorized(w, r, err)
				return
			}

			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext extracts claims from the request context.
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(claimsKey).(Claims)
	return claims, ok
}

func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// Option configures the auth middleware.
type Option func(*config)

type config struct {
	responder  ErrorResponder
	skipPaths  map[string]bool
	translator i18n.Translator
}

// WithErrorResponder sets a custom error responder.
func WithErrorResponder(r ErrorResponder) Option {
	return func(c *config) { c.responder = r }
}

// WithSkipPaths sets paths that bypass authentication.
func WithSkipPaths(paths ...string) Option {
	return func(c *config) {
		for _, p := range paths {
			c.skipPaths[p] = true
		}
	}
}

// WithTranslator wires a Translator used by the default error
// responder when emitting the 401 body. Consuming projects pass a
// real i18n.Translator (bundle-backed or LLM-backed) per CONST-046;
// the default is i18n.NoopTranslator{} which returns the message ID
// verbatim so anti-bluff evidence remains visible in captured wire
// traffic.
func WithTranslator(tr i18n.Translator) Option {
	return func(c *config) {
		if tr != nil {
			c.translator = tr
		}
	}
}
