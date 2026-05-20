// Package recovery provides panic recovery middleware for net/http handlers.
// When a downstream handler panics, the middleware recovers, logs the error,
// and returns an HTTP 500 Internal Server Error response.
package recovery

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime/debug"

	"digital.vasic.middleware/pkg/i18n"
)

// msgInternalServerError is the i18n message ID for the default panic
// response body. Per CONST-046 (no-hardcoded-content) the English text
// lives in the i18n bundle, not as a literal in this source file; a
// Serbian or Japanese end user receives a localised 500 body when the
// consuming project wires a bundle/LLM-backed Translator.
const msgInternalServerError = "middleware_recovery_internal_server_error"

// Config holds configuration for the recovery middleware.
type Config struct {
	// Output is the writer where panic details are logged. Defaults to
	// os.Stderr.
	Output io.Writer

	// PrintStack controls whether the full stack trace is included in the
	// log output. Defaults to true.
	PrintStack bool

	// ResponseBody is the body returned to the client on panic. If nil the
	// configured Translator renders the default localised message at
	// request time (CONST-046). Setting ResponseBody explicitly takes
	// precedence over the Translator and is returned verbatim.
	ResponseBody []byte

	// ResponseContentType is the Content-Type header for the error response.
	// Defaults to "text/plain; charset=utf-8".
	ResponseContentType string

	// Translator renders the panic response body when ResponseBody is nil.
	// Defaults to i18n.NoopTranslator{}, which returns the message ID
	// verbatim so absence-of-bundle is loudly visible in captured HTTP
	// responses. Consumers wire a real bundle-backed or LLM-backed
	// Translator for localised 500 bodies without modifying this package.
	Translator i18n.Translator
}

// DefaultConfig returns a default recovery configuration that logs to stderr
// with stack traces enabled. The response body is rendered at request time
// from the i18n Translator (CONST-046).
func DefaultConfig() *Config {
	return &Config{
		Output:              os.Stderr,
		PrintStack:          true,
		ResponseContentType: "text/plain; charset=utf-8",
		Translator:          i18n.NoopTranslator{},
	}
}

// New creates a panic recovery middleware. If cfg is nil the default
// configuration is used.
func New(cfg *Config) func(http.Handler) http.Handler {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.Output == nil {
		cfg.Output = os.Stderr
	}
	if cfg.ResponseContentType == "" {
		cfg.ResponseContentType = "text/plain; charset=utf-8"
	}
	if cfg.Translator == nil {
		cfg.Translator = i18n.NoopTranslator{}
	}

	logger := log.New(cfg.Output, "", log.LstdFlags)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					if cfg.PrintStack {
						logger.Printf("[RECOVERY] panic recovered: %v\n%s", err, debug.Stack())
					} else {
						logger.Printf("[RECOVERY] panic recovered: %v", err)
					}

					w.Header().Set("Content-Type", cfg.ResponseContentType)
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, responseBody(cfg, r))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// responseBody returns the panic response body. An explicit
// Config.ResponseBody wins verbatim; otherwise the body is rendered from
// the configured Translator using the request context so per-request
// Accept-Language negotiation performed by the consuming project is
// honoured (CONST-046).
func responseBody(cfg *Config, r *http.Request) string {
	if cfg.ResponseBody != nil {
		return string(cfg.ResponseBody)
	}
	return cfg.Translator.T(r.Context(), msgInternalServerError, nil) + "\n"
}
