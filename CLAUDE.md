# CLAUDE.md

## INHERITED FROM constitution/CLAUDE.md

All rules in `constitution/CLAUDE.md` (and the `constitution/Constitution.md` it references) apply unconditionally. This file's rules below extend them — they MUST NOT weaken any inherited rule. See parent root `CLAUDE.md` §6.AD for the Lava-specific incorporation context (29th §6.L cycle, 2026-05-14) and §6.AD-debt for the implementation-gap inventory. Use `constitution/find_constitution.sh` from the parent project root to resolve the absolute path of the submodule from any nested location.

## INHERITED FROM the Helix Constitution

This module is governed by the Helix Constitution. All rules in the
constitution's `CLAUDE.md` and the `Constitution.md` it references apply
unconditionally. Locate the constitution from any nested depth via its
`find_constitution.sh` helper — do NOT hardcode a path (this module stays
fully decoupled and project-agnostic per §11.4.28).

Canonical reference: https://github.com/HelixDevelopment/HelixConstitution

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`digital.vasic.middleware` is a standalone Go module providing reusable HTTP middleware for `net/http`. It offers CORS handling, request logging, panic recovery, request ID generation, and middleware chaining -- all built on the Go standard library with zero framework dependencies.

## Commands

```bash
# Build all packages
go build ./...

# Run all tests
go test ./... -count=1

# Run tests with verbose output
go test -v ./... -count=1

# Run tests for a specific package
go test -v ./pkg/cors/ -count=1
go test -v ./pkg/logging/ -count=1
go test -v ./pkg/recovery/ -count=1
go test -v ./pkg/requestid/ -count=1
go test -v ./pkg/chain/ -count=1

# Run a single test
go test -v -run TestNew_RecoversPanic ./pkg/recovery/

# Test coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Tidy dependencies
go mod tidy
```

## Architecture

All middleware follows the standard `func(http.Handler) http.Handler` signature, making them composable with any `net/http`-compatible router or framework.

| Package | Purpose |
|---|---|
| `pkg/cors` | Configurable Cross-Origin Resource Sharing headers and preflight handling |
| `pkg/logging` | Request logging with method, path, status code, and duration |
| `pkg/recovery` | Panic recovery that catches panics and returns HTTP 500 |
| `pkg/requestid` | Request ID propagation via X-Request-ID header or UUID generation |
| `pkg/chain` | Middleware chaining utility to compose multiple middleware functions |

### Middleware Signature

Every middleware in this module uses the standard pattern:

```go
func New(cfg *Config) func(http.Handler) http.Handler
```

This allows composition via the `chain` package:

```go
combined := chain.Chain(
    requestid.New(),
    logging.New(nil),
    recovery.New(nil),
    cors.New(cors.DefaultConfig()),
)
handler := combined(myAppHandler)
```

## Constraints

- **Standard library only**: All middleware uses `net/http`. No framework dependencies (no Gin, no Echo, no Chi).
- **Single external runtime dependency**: `github.com/google/uuid` for UUID generation in the requestid package.
- **Test dependency**: `github.com/stretchr/testify` for assertions in tests only.
- **Go 1.24.0+** required.

## Conventions

- Each package exports a `Config` struct with a `DefaultConfig()` constructor and a `New(cfg)` middleware factory.
- Nil config arguments fall back to default configuration.
- Tests are colocated with source in `*_test.go` files.
- Table-driven tests where applicable.
- Context values use unexported key types to prevent collisions.
