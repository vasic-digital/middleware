# Middleware Test Coverage Ledger (round-251)

Round-251 deep-doc enrichment under CONST-035 / Article XI §11.9 / CONST-050(B).

This document is the authoritative mapping of every exported symbol in
`pkg/{chain,cors,logging,recovery,requestid}` to the test sources that exercise
it. Drift between this file and `go test -cover` output is a CONST-035 bluff at
the documentation-truth layer — fix the document OR add the missing test, never
silently leave the gap.

## Verbatim 2026-05-19 operator mandate (CONST-049 §11.4.17)

> "all existing tests and Challenges do work in anti-bluff manner - they MUST confirm that all tested codebase really works as expected! We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completition and full usability by end users of the product!"

## Test-type matrix (CONST-050(B))

| Test type | Location | Status |
|-----------|----------|--------|
| Unit | `pkg/*/`*_test.go` | PRESENT — every package |
| Coverage-driven unit | `pkg/{logging,recovery,validation,ratelimit,brotli,cache}/*_coverage_test.go` | PRESENT |
| Edge-case unit | `pkg/{recovery,validation}/*_edge_test.go` | PRESENT |
| i18n unit | `pkg/{auth,ratelimit,validation}/*_i18n_test.go` | PRESENT (round-124) |
| Bilingual Challenge | `challenges/middleware_describe_challenge.sh` | PRESENT (round-251) |
| Bilingual fixtures | `challenges/fixtures/{en,sr-Latn}.yaml` | PRESENT (round-251) |
| Real-net runner | `challenges/runner/main.go` | PRESENT (round-251) |
| Operational Challenges | `challenges/scripts/{chaos,ddos,scaling,stress,ui,ux,no_suspend,host_no_auto_suspend}_*.sh` | PRESENT |

## `pkg/chain`

| Symbol | Kind | Test source(s) |
|--------|------|----------------|
| `Chain` | func | `pkg/chain/chain_test.go` (TestChain_EmptyMiddleware, TestChain_SingleMiddleware, TestChain_ExecutionOrder, TestChain_MiddlewareCanShortCircuit, TestChain_PreservesResponseBody) |
| Execution-order invariant | runtime invariant | TestChain_ExecutionOrder + round-251 runner Section A (request-id wraps outermost; CORS wraps innermost) |
| Short-circuit invariant | runtime invariant | TestChain_MiddlewareCanShortCircuit + round-251 runner OPTIONS preflight path |
| Empty-chain identity | runtime invariant | TestChain_EmptyMiddleware |
| Response-body preservation | runtime invariant | TestChain_PreservesResponseBody + round-251 runner POST /api/echo byte-equality |

## `pkg/cors`

| Symbol | Kind | Test source(s) |
|--------|------|----------------|
| `Config` | struct | `pkg/cors/cors_test.go` (TestDefaultConfig, TestNew_*) |
| `DefaultConfig` | constructor | TestDefaultConfig, TestNew_NilConfigUsesDefaults + round-251 runner cors.DefaultConfig() composition |
| `New` | constructor | TestNew_NilConfigUsesDefaults, TestNew_WildcardOrigin, TestNew_SpecificOriginAllowed, TestNew_SpecificOriginDenied, TestNew_PreflightReturns204, TestNew_AllowCredentials, TestNew_ExposeHeaders, TestNew_MaxAgeZeroOmitsHeader, TestNew_NoOriginHeader, TestNew_PassesThroughToHandler |
| Preflight short-circuit | runtime invariant | TestNew_PreflightReturns204 + round-251 runner OPTIONS /api/hello + /api/pozdrav (both locales) |
| Wildcard-origin echo | runtime invariant | TestNew_WildcardOrigin + round-251 runner CORS-origin assertion on OPTIONS |
| Specific-origin allow + Vary | runtime invariant | TestNew_SpecificOriginAllowed |
| Specific-origin deny | runtime invariant | TestNew_SpecificOriginDenied |
| Credentials header | runtime invariant | TestNew_AllowCredentials |
| Expose-headers serialization | runtime invariant | TestNew_ExposeHeaders |
| Max-age omission on 0 | runtime invariant | TestNew_MaxAgeZeroOmitsHeader |

## `pkg/logging`

| Symbol | Kind | Test source(s) |
|--------|------|----------------|
| `Config` | struct | `pkg/logging/logging_test.go` (TestDefaultConfig, TestNew_*) |
| `DefaultConfig` | constructor | TestDefaultConfig |
| `New` | constructor | TestNew_LogsRequestDetails, TestNew_LogsNonOKStatus, TestNew_LogsDefaultStatusWhenWriteOnly, TestNew_NilOutputUsesDefault, TestNew_SkipPaths, TestNew_EmptySkipPaths, TestNew_MultipleSkipPaths, TestNew_PassesThroughToHandler |
| `Write` (statusRecorder) | method | TestStatusRecorder_WriteBeforeWriteHeader, TestStatusRecorder_WriteAfterWriteHeader |
| `WriteHeader` (statusRecorder) | method | TestStatusRecorder_WriteHeaderIdempotent + round-251 runner indirect (every POST/GET exercises the recorder) |
| Skip-paths matching | runtime invariant | TestNew_SkipPaths, TestNew_MultipleSkipPaths, TestNew_EmptySkipPaths |
| Nil-output fallback | runtime invariant | TestNew_NilOutputUsesDefault |
| Coverage edge cases | misc | `pkg/logging/logging_coverage_test.go` (status-recorder branch coverage) |

## `pkg/recovery`

| Symbol | Kind | Test source(s) |
|--------|------|----------------|
| `Config` | struct | `pkg/recovery/recovery_test.go` (TestDefaultConfig, TestNew_*) |
| `DefaultConfig` | constructor | TestDefaultConfig, TestNew_AllNilDefaults |
| `New` | constructor | TestNew_RecoversPanic, TestNew_RecoversPanicWithStackTrace, TestNew_RecoversPanicWithNonStringValue, TestNew_PanicWithError, TestNew_NoPanicPassesThrough, TestNew_CustomResponseBody, TestNew_NilOutput, TestNew_NilResponseBody, TestNew_EmptyResponseContentType, TestNew_PrintStackTrue_WithNilOutput, TestNew_CustomAllFields |
| Panic-recovery happy path | runtime invariant | TestNew_RecoversPanic + round-251 runner GET /api/panic + /api/panika (both locales) |
| Panic with error value | runtime invariant | TestNew_PanicWithError, TestRecovery_PanicWithError |
| Panic with non-string value | runtime invariant | TestNew_RecoversPanicWithNonStringValue, TestRecovery_PanicWithInt, TestRecovery_PanicWithString, TestRecovery_PanicWithNil |
| Stack-trace control | runtime invariant | TestNew_RecoversPanicWithStackTrace, TestRecovery_PrintStackFalse, TestRecovery_PrintStackTrue |
| Custom response body | runtime invariant | TestNew_CustomResponseBody, TestRecovery_CustomResponseBody |
| No-panic pass-through | runtime invariant | TestNew_NoPanicPassesThrough, TestRecovery_NoPanic |
| Composition: recovery+cors+requestid | integration | TestChain_RecoveryWithCORSAndRequestID + round-251 runner full chain |
| Edge-case unit | misc | `pkg/recovery/recovery_edge_test.go` |
| Coverage-driven unit | misc | `pkg/recovery/recovery_coverage_test.go` |

## `pkg/requestid`

| Symbol | Kind | Test source(s) |
|--------|------|----------------|
| `HeaderKey` | const | exercised by every test; also asserted as the header literal by round-251 runner |
| `New` | constructor | TestNew_GeneratesUUID, TestNew_ReusesExistingHeader, TestNew_UniquePerRequest, TestNew_PassesThroughToHandler + round-251 runner Section B (inbound X-Request-ID preserved end-to-end) |
| `FromContext` | func | TestFromContext_EmptyWhenNotSet, TestFromContext_ReturnsStoredID, TestRequestID_FromContext_Empty |
| `FromRequest` | func | TestFromRequest_ReturnsID + round-251 runner X-Trace-ID echo assertion |
| UUID generation on missing header | runtime invariant | TestNew_GeneratesUUID, TestRequestID_GeneratesNewID + round-251 runner GET /api/hello |
| Existing-header reuse | runtime invariant | TestNew_ReusesExistingHeader, TestRequestID_ExistingHeader + round-251 runner Section B |
| Uniqueness per request | runtime invariant | TestNew_UniquePerRequest, TestRequestID_UniquePerRequest |
| Extremely-long existing-ID tolerance | runtime invariant | TestRequestID_ExtremelyLongExistingID |
| Pass-through to handler | runtime invariant | TestNew_PassesThroughToHandler |

## Additional packages (out of round-251 scope but tested)

| Package | Test source(s) | Coverage notes |
|---------|----------------|----------------|
| `pkg/altsvc` | `pkg/altsvc/altsvc_test.go` | Alt-Svc header injection middleware |
| `pkg/auth` | `pkg/auth/auth_test.go` + `auth_i18n_test.go` | Auth middleware with i18n error messages |
| `pkg/brotli` | `pkg/brotli/brotli_test.go` + `brotli_coverage_test.go` | Brotli response compression |
| `pkg/cache` | `pkg/cache/cache_test.go` + `cache_coverage_test.go` | Cache-Control header middleware |
| `pkg/gin` | `pkg/gin/gin_test.go` | Gin-framework bridge adapters |
| `pkg/i18n` | `pkg/i18n/translator_test.go` | i18n translator (CONST-046) |
| `pkg/ratelimit` | `pkg/ratelimit/ratelimit_test.go` + `ratelimit_coverage_test.go` + `ratelimit_i18n_test.go` | Rate-limit middleware with i18n |
| `pkg/validation` | `pkg/validation/validation_test.go` + `validation_edge_test.go` + `validation_coverage_test.go` + `validation_i18n_test.go` | Request-validation middleware |

These adapters are covered by their own per-package unit + coverage suites and
are exercised indirectly when consumed alongside the core chain — `pkg/{chain,
cors,logging,recovery,requestid}` is the round-251 exerciser because it
requires no external framework while still proving the standard-library
`net/http` middleware contract end-to-end with real socket transport.

## Edge cases covered (round-251)

- Empty request body — `challenges/fixtures/en.yaml` (GET /api/hello)
- UTF-8 body with diacritics — `challenges/fixtures/sr-Latn.yaml` (POST /api/odjek with `čšđžć`)
- Non-ASCII URL path — `challenges/fixtures/sr-Latn.yaml` (GET /api/pozdrav, GET /api/panika)
- Preflight OPTIONS — both locales, wildcard origin echo asserted
- Forced panic recovery — both locales, status 500 + recovery body asserted
- X-Request-ID generation when header absent — every fixture request
- X-Request-ID reuse when header present — round-251 runner Section B
- FromRequest context extraction — terminal handler echoes via X-Trace-ID
- Request-ID / Trace-ID equality — runner asserts request-id == trace-id on 200s

## Paired-mutation Challenge

`challenges/middleware_describe_challenge.sh` accepts `--anti-bluff-mutate` to plant a
deliberate ledger-vs-source mismatch (renames one tracked symbol in the ledger)
and asserts the gate FAILS with exit 99. Without the flag the gate runs normal
validation and MUST exit 0. Composition: CONST-035 (anti-bluff) × CONST-050(B)
(paired mutation) × CONST-047 (cascade).

## Anti-bluff acceptance criteria

1. `go test -count=1 -race ./...` exits 0 — all packages PASS (verified round-251).
2. `bash challenges/middleware_describe_challenge.sh` exits 0 (gate PASS on clean tree).
3. `bash challenges/middleware_describe_challenge.sh --anti-bluff-mutate` exits 99 (gate correctly fails on planted mutation).
4. Every symbol in this ledger appears in the listed test source verbatim — no metadata-only / configuration-only ledger entries.
5. Runner output preserves UTF-8 bodies byte-for-byte from `challenges/fixtures/sr-Latn.yaml` through the real httptest socket back to the captured response body.
