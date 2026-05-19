# digital.vasic.middleware

Reusable HTTP middleware for Go's `net/http` standard library. Round-251 deep-doc + paired-mutation challenge enrichment.

**Module**: `digital.vasic.middleware`
**Go**: 1.25+
**License**: See LICENSE

---

## Features

- **Single contract** — every middleware honours
  `func(http.Handler) http.Handler`, so any router or framework that
  composes `net/http` can mount them unchanged.
- **Zero framework dependencies** — built on `net/http`. The optional
  `pkg/gin` adapter is a thin bridge; the core packages never reach for
  Gin, Echo, or Chi.
- **Composable** — `pkg/chain` collapses an ordered list of
  middleware into one wrapping function. `requestid -> logging ->
  recovery -> cors` is the canonical baseline.
- **UTF-8 safe** — bilingual round-251 fixtures exercise non-ASCII URL
  paths and UTF-8 request/response bodies through a real socket; the
  runner asserts byte-equality round-trip with Latinic Serbian
  diacritics (`čšđžć`).
- **Production-shaped panic recovery** — `pkg/recovery` catches panics
  inside the wrapped chain, logs the stack (configurable), returns a
  500 with a customisable body and content-type, and never lets the
  panic escape.
- **Request-ID propagation** — `pkg/requestid` injects an X-Request-ID
  header on every response, reuses an inbound value when present,
  generates a UUIDv4 when absent, and exposes the ID through the
  request context for downstream handlers.
- **CORS done right** — `pkg/cors` short-circuits OPTIONS preflights
  with 204, honours wildcard + specific-origin allow lists, attaches
  `Vary: Origin` when needed, and respects credentials / expose /
  max-age configuration.

## Packages

| Package | Import path | Description |
|---------|-------------|-------------|
| `chain` | `digital.vasic.middleware/pkg/chain` | Compose multiple middleware into one |
| `cors` | `digital.vasic.middleware/pkg/cors` | Cross-Origin Resource Sharing |
| `logging` | `digital.vasic.middleware/pkg/logging` | Request logging with status + duration |
| `recovery` | `digital.vasic.middleware/pkg/recovery` | Panic recovery -> 500 |
| `requestid` | `digital.vasic.middleware/pkg/requestid` | X-Request-ID propagation |
| `auth` | `digital.vasic.middleware/pkg/auth` | Authentication helpers (i18n) |
| `validation` | `digital.vasic.middleware/pkg/validation` | Request validation (i18n) |
| `ratelimit` | `digital.vasic.middleware/pkg/ratelimit` | Rate limiting (i18n) |
| `brotli` | `digital.vasic.middleware/pkg/brotli` | Brotli compression |
| `cache` | `digital.vasic.middleware/pkg/cache` | Cache-Control headers |
| `altsvc` | `digital.vasic.middleware/pkg/altsvc` | Alt-Svc header injection |
| `gin` | `digital.vasic.middleware/pkg/gin` | Gin framework adapters |
| `i18n` | `digital.vasic.middleware/pkg/i18n` | Translator for i18n messages |

## Installation

```bash
go get digital.vasic.middleware
```

## Quick Start

```go
package main

import (
    "net/http"

    "digital.vasic.middleware/pkg/chain"
    "digital.vasic.middleware/pkg/cors"
    "digital.vasic.middleware/pkg/logging"
    "digital.vasic.middleware/pkg/recovery"
    "digital.vasic.middleware/pkg/requestid"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        // Pull the request-id out of the context for log correlation.
        id := requestid.FromRequest(r)
        w.Header().Set("X-Trace-ID", id)
        _, _ = w.Write([]byte("Hello, world!"))
    })

    middleware := chain.Chain(
        requestid.New(),
        logging.New(nil),
        recovery.New(nil),
        cors.New(cors.DefaultConfig()),
    )

    server := &http.Server{Addr: ":8080", Handler: middleware(mux)}
    _ = server.ListenAndServe()
}
```

## Per-Middleware Configuration

```go
// CORS
cors.New(&cors.Config{
    AllowOrigins:     []string{"https://example.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Authorization", "Content-Type"},
    ExposeHeaders:    []string{"X-Request-ID"},
    AllowCredentials: true,
    MaxAge:           3600,
})

// Logging
logging.New(&logging.Config{
    Output:    os.Stdout,
    SkipPaths: map[string]struct{}{"/health": {}},
})

// Recovery
recovery.New(&recovery.Config{
    Output:              os.Stderr,
    PrintStack:          true,
    ResponseBody:        []byte(`{"error":"internal_server_error"}`),
    ResponseContentType: "application/json",
})

// Request ID — already context-aware
handler := requestid.New()(myHandler)
id := requestid.FromRequest(r) // inside the handler
```

## Testing

```bash
# Unit + coverage suites (every package, race-tested)
go test -count=1 -race ./...

# Verbose
go test -count=1 -race -v ./...

# Per-package
go test -count=1 -race -v ./pkg/recovery/

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Round-251 Challenge

```bash
# Clean-mode (must exit 0 — proves docs, fixtures, runner, real-net all green)
bash challenges/middleware_describe_challenge.sh

# Paired-mutation mode (must exit 99 — proves gate actually catches drift)
bash challenges/middleware_describe_challenge.sh --anti-bluff-mutate

# Run the bilingual runner directly
go run ./challenges/runner -fixtures challenges/fixtures
```

The Challenge validates: deep-doc ledger present + version-tagged + carries
the verbatim Article XI §11.9 mandate; every exported symbol from
`pkg/{chain,cors,logging,recovery,requestid}` is cross-referenced in the
ledger; bilingual fixtures parse and cover at least 2 locales; runner
builds and round-trips both ASCII and UTF-8 (diacritic) request bodies
through the real `net/http` middleware chain over a real socket; README
declares the round-251 anti-bluff guarantees section.

## Anti-bluff guarantees (round-251)

- `go test -count=1 -race ./...` runs every package's unit + coverage suite —
  zero `t.Skip()` without a `SKIP-OK: #<ticket>` marker.
- `challenges/middleware_describe_challenge.sh` is paired-mutation aware
  (`--anti-bluff-mutate` exits 99 to prove the gate actually detects a
  planted ledger-vs-source rename).
- Bilingual fixtures (`challenges/fixtures/{en,sr-Latn}.yaml`) exercise
  non-ASCII URL paths + UTF-8 request bodies; the runner asserts
  byte-equality round-trip through the real `httptest.Server` socket,
  cross-checks the `X-Request-ID` header against the terminal handler's
  `X-Trace-ID` echo, verifies CORS preflight returns 204, and confirms the
  recovery middleware caught the forced panic with a 500.
- `docs/test-coverage.md` enumerates every public symbol with its test
  sources — drift between the file and `go test -cover` is treated as a
  CONST-035 / Article XI §11.9 bluff at the documentation-truth layer.
- The runner exercises the real net/http stack — real `net.Listener`,
  real transport, real client — not in-memory `ResponseRecorder` shortcuts;
  the interface-contract section additionally restates the
  `New() != nil` + wrap-doesn't-panic invariants against `ResponseRecorder`.

## Constitutional anchors

Middleware inherits Article XI §11.9 (anti-bluff), CONST-035 (zero-bluff),
CONST-047 (recursive submodule application), CONST-048 (full-automation-
coverage), CONST-050 (no-fakes-beyond-unit-tests + 100%-test-type-coverage),
CONST-051 (submodules-as-equal-codebase + decoupling), CONST-053 (.gitignore /
no-versioned-build-artifacts) from the constitution submodule. See
`CONSTITUTION.md`, `CLAUDE.md`, `AGENTS.md` for the verbatim mandates.

## Requirements

- Go 1.25.0+
- Single runtime dependency: `github.com/google/uuid` (request-ID generation)
- Test dependency: `github.com/stretchr/testify`

## Documentation

- [Architecture](docs/ARCHITECTURE.md) — Design patterns, package relationships
- [Test Coverage Ledger (round-251)](docs/test-coverage.md) — symbol -> test source mapping
- [Host Power Management](docs/HOST_POWER_MANAGEMENT.md) — CONST-033 enforcement

## License

See LICENSE file for details.
