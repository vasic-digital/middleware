// Round-251 challenge runner for Middleware.
//
// Builds the bilingual fixture set from challenges/fixtures/{en,sr-Latn}.yaml,
// composes the real Middleware chain (requestid -> logging -> recovery ->
// cors) around a real terminal handler, stands up a real httptest.Server
// (real net.Listener on 127.0.0.1, real net/http transport), and drives
// every fixture request end-to-end through the live socket. The runner
// asserts on user-visible response signals: status code, response body
// byte-equality, CORS headers on preflight, X-Request-ID propagation
// (CONST-046 / Article XI §11.9 alignment for non-ASCII paths + bodies).
//
// Anti-bluff invariants enforced by this runner (Article XI §11.9 / CONST-035):
//
//   - No metadata-only / grep-only PASS. Every PASS line is preceded by the
//     actual locale, the actual HTTP method+path, the actual decoded status,
//     and the actual byte counts.
//   - Real net.Listener + real net/http client — no in-memory shortcut. The
//     middleware code paths (header set, panic recover, request-ID inject,
//     CORS preflight short-circuit) all execute exactly as they would
//     against a production listener.
//   - Failure to round-trip non-ASCII bytes in path or body, missing CORS
//     header on preflight, missing X-Request-ID, unrecovered panic, or
//     wrong status code is a hard FAIL — exit non-zero.
//   - No mocks injected into the middleware. The runner uses the public
//     New() factories of every Middleware package exactly as a downstream
//     consumer would.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"digital.vasic.middleware/pkg/chain"
	"digital.vasic.middleware/pkg/cors"
	"digital.vasic.middleware/pkg/logging"
	"digital.vasic.middleware/pkg/recovery"
	"digital.vasic.middleware/pkg/requestid"
)

// fixture is the minimal YAML subset we hand-parse to avoid pulling
// gopkg.in/yaml.v3 into the runner's dependency surface. Schema:
//
//	locale: <string>
//	description: <string>
//	requests:
//	  - method: <string>
//	    path: <string>
//	    origin: <string>
//	    body: |
//	      <line>
//	    expect_status: <int>
//	    expect_body: <string>
//	    expect_body_contains: <string>
//	    expect_cors_origin: <string>
//	    expect_request_id_set: <bool>
type fixture struct {
	Locale      string
	Description string
	Requests    []fixtureRequest
}

type fixtureRequest struct {
	Method              string
	Path                string
	Origin              string
	Body                string
	ExpectStatus        int
	ExpectBody          string
	ExpectBodyContains  string
	ExpectCORSOrigin    string
	ExpectRequestIDSet  bool
}

func main() {
	dir := flag.String(
		"fixtures",
		"",
		"directory holding *.yaml fixture files",
	)
	flag.Parse()

	if *dir == "" {
		fail("missing -fixtures <dir>")
	}

	entries, err := os.ReadDir(*dir)
	if err != nil {
		fail("cannot read fixtures dir %q: %v", *dir, err)
	}

	var fixtures []fixture
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(*dir, e.Name())
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			fail("cannot read fixture %q: %v", path, rerr)
		}
		fix, perr := parseFixture(string(raw))
		if perr != nil {
			fail("cannot parse fixture %q: %v", path, perr)
		}
		fixtures = append(fixtures, fix)
	}
	if len(fixtures) == 0 {
		fail("no fixtures found under %q", *dir)
	}

	// Compose the real middleware chain around a terminal handler that
	// echoes the request body for /api/echo|/api/odjek and panics on
	// /api/panic|/api/panika so the recovery middleware has something to
	// catch. Every fixture route is dispatched by path-suffix here.
	terminal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Surface request-id through a custom header so the runner can
		// verify requestid.FromRequest actually populated the context.
		if id := requestid.FromRequest(r); id != "" {
			w.Header().Set("X-Trace-ID", id)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/panic") ||
			strings.HasSuffix(r.URL.Path, "/panika"):
			panic("round-251 forced panic — exercises recovery middleware")
		case strings.HasSuffix(r.URL.Path, "/echo") ||
			strings.HasSuffix(r.URL.Path, "/odjek"):
			body, _ := io.ReadAll(r.Body)
			_ = r.Body.Close()
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		case strings.HasSuffix(r.URL.Path, "/hello"):
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "Hello, world!")
		case strings.HasSuffix(r.URL.Path, "/pozdrav"):
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "Zdravo, svete!")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	// Silence logging output during the run — we still exercise its code
	// path (the formatting, the status-recorder write-after-write), but
	// we discard the noise so the runner output stays readable.
	logCfg := &logging.Config{Output: io.Discard}

	composed := chain.Chain(
		requestid.New(),
		logging.New(logCfg),
		recovery.New(&recovery.Config{
			Output:              io.Discard,
			PrintStack:          false,
			ResponseBody:        []byte("Internal Server Error\n"),
			ResponseContentType: "text/plain; charset=utf-8",
		}),
		cors.New(cors.DefaultConfig()),
	)(terminal)

	srv := httptest.NewServer(composed)
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	pass := 0
	failures := 0

	for _, fx := range fixtures {
		for _, req := range fx.Requests {
			res, body, rerr := exercise(client, srv.URL, req)
			if rerr != nil {
				fmt.Printf(
					"FAIL [%s] %s %s: transport error: %v\n",
					fx.Locale, req.Method, req.Path, rerr,
				)
				failures++
				continue
			}

			if res.StatusCode != req.ExpectStatus {
				fmt.Printf(
					"FAIL [%s] %s %s: status want=%d got=%d body=%q\n",
					fx.Locale, req.Method, req.Path,
					req.ExpectStatus, res.StatusCode, truncate(body, 64),
				)
				failures++
				continue
			}

			// Strict body-equality when expect_body is set.
			if req.ExpectBody != "" {
				if string(body) != req.ExpectBody {
					fmt.Printf(
						"FAIL [%s] %s %s: body drift want=%q got=%q\n",
						fx.Locale, req.Method, req.Path,
						req.ExpectBody, truncate(body, 64),
					)
					failures++
					continue
				}
			}

			// Substring match when expect_body_contains is set. Proves
			// the UTF-8 echo round-tripped without corruption.
			if req.ExpectBodyContains != "" {
				if !bytes.Contains(body, []byte(req.ExpectBodyContains)) {
					fmt.Printf(
						"FAIL [%s] %s %s: body missing substring want=%q got=%q\n",
						fx.Locale, req.Method, req.Path,
						req.ExpectBodyContains, truncate(body, 96),
					)
					failures++
					continue
				}
			}

			// CORS preflight: every OPTIONS request must come back
			// with Access-Control-Allow-Origin set per the default
			// permissive config.
			if req.ExpectCORSOrigin != "" {
				got := res.Header.Get("Access-Control-Allow-Origin")
				if got != req.ExpectCORSOrigin {
					fmt.Printf(
						"FAIL [%s] %s %s: CORS origin want=%q got=%q\n",
						fx.Locale, req.Method, req.Path,
						req.ExpectCORSOrigin, got,
					)
					failures++
					continue
				}
			}

			// Request-ID propagation: requestid middleware MUST set
			// the X-Request-ID response header AND the terminal
			// handler echoed it back via X-Trace-ID (for non-panic
			// routes — recovery short-circuits before the terminal).
			if req.ExpectRequestIDSet {
				got := res.Header.Get(requestid.HeaderKey)
				if got == "" {
					fmt.Printf(
						"FAIL [%s] %s %s: missing X-Request-ID header\n",
						fx.Locale, req.Method, req.Path,
					)
					failures++
					continue
				}
				// For non-panic, non-OPTIONS routes the terminal
				// handler also set X-Trace-ID — cross-check the IDs
				// matched (proves FromRequest actually pulled the
				// value out of the context).
				if res.StatusCode == http.StatusOK {
					trace := res.Header.Get("X-Trace-ID")
					if trace == "" {
						fmt.Printf(
							"FAIL [%s] %s %s: missing X-Trace-ID — FromRequest returned empty\n",
							fx.Locale, req.Method, req.Path,
						)
						failures++
						continue
					}
					if trace != got {
						fmt.Printf(
							"FAIL [%s] %s %s: trace/request-id drift trace=%q req-id=%q\n",
							fx.Locale, req.Method, req.Path, trace, got,
						)
						failures++
						continue
					}
				}
			}

			fmt.Printf(
				"PASS [%s] %s %s status=%d resp_bytes=%d req-id=%q\n",
				fx.Locale, req.Method, req.Path,
				res.StatusCode, len(body),
				res.Header.Get(requestid.HeaderKey),
			)
			pass++
		}
	}

	// Section B: exercise the existing X-Request-ID header path —
	// requestid middleware MUST reuse the inbound value, not regenerate.
	{
		const provided = "round-251-fixed-trace-id-bilingual"
		req, _ := http.NewRequestWithContext(
			context.Background(), http.MethodGet, srv.URL+"/api/hello", nil,
		)
		req.Header.Set(requestid.HeaderKey, provided)
		res, err := client.Do(req)
		if err != nil {
			fmt.Printf("FAIL [contract:requestid-reuse] transport: %v\n", err)
			failures++
		} else {
			body, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()
			got := res.Header.Get(requestid.HeaderKey)
			if got != provided {
				fmt.Printf(
					"FAIL [contract:requestid-reuse] want=%q got=%q body=%q\n",
					provided, got, truncate(body, 32),
				)
				failures++
			} else {
				fmt.Printf(
					"PASS [contract:requestid-reuse] inbound X-Request-ID preserved end-to-end\n",
				)
				pass++
			}
		}
	}

	// Section C: interface-contract verification — every package's
	// New() factory returns a non-nil middleware that wraps any
	// http.Handler without panic. Restating it here at runtime
	// guards against an accidental future regression where one
	// factory returns nil.
	contractChecks := []struct {
		name string
		mw   func(http.Handler) http.Handler
	}{
		{"requestid", requestid.New()},
		{"logging", logging.New(&logging.Config{Output: io.Discard})},
		{"recovery", recovery.New(&recovery.Config{Output: io.Discard})},
		{"cors", cors.New(cors.DefaultConfig())},
	}
	noop := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	for _, cc := range contractChecks {
		if cc.mw == nil {
			fmt.Printf("FAIL [contract:%s] New returned nil\n", cc.name)
			failures++
			continue
		}
		wrapped := cc.mw(noop)
		if wrapped == nil {
			fmt.Printf("FAIL [contract:%s] wrap returned nil\n", cc.name)
			failures++
			continue
		}
		// Exercise once against a real ResponseRecorder.
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/health", nil)
		wrapped.ServeHTTP(rec, r)
		if rec.Code != http.StatusOK {
			fmt.Printf(
				"FAIL [contract:%s] wrap broke pass-through status=%d\n",
				cc.name, rec.Code,
			)
			failures++
			continue
		}
		fmt.Printf("PASS [contract:%s] wrap + ServeHTTP pass-through ok\n", cc.name)
		pass++
	}

	fmt.Printf(
		"\nSummary: %d PASS, %d FAIL across %d locale(s) + interface contract\n",
		pass, failures, len(fixtures),
	)
	if failures > 0 {
		os.Exit(1)
	}
}

func exercise(
	client *http.Client, base string, req fixtureRequest,
) (*http.Response, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}
	hr, err := http.NewRequestWithContext(ctx, req.Method, base+req.Path, bodyReader)
	if err != nil {
		return nil, nil, err
	}
	if req.Origin != "" {
		hr.Header.Set("Origin", req.Origin)
	}
	if req.Method == http.MethodOptions {
		hr.Header.Set("Access-Control-Request-Method", "GET")
	}
	res, err := client.Do(hr)
	if err != nil {
		return nil, nil, err
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, body, nil
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// parseFixture parses the minimal hand-rolled YAML subset used by the
// round-251 fixtures. Indentation is fixed at 2 spaces.
func parseFixture(src string) (fixture, error) {
	var fx fixture
	lines := strings.Split(src, "\n")

	type section int
	const (
		sectionNone section = iota
		sectionRequests
	)

	var cur section
	var curReq *fixtureRequest
	var inBlockScalar bool
	var blockBuf strings.Builder
	var blockIndent int
	var blockTarget *string

	flushBlock := func() {
		if inBlockScalar && blockTarget != nil {
			*blockTarget = blockBuf.String()
			blockBuf.Reset()
			inBlockScalar = false
			blockTarget = nil
		}
	}

	for _, raw := range lines {
		if inBlockScalar {
			if strings.TrimSpace(raw) == "" {
				blockBuf.WriteString("\n")
				continue
			}
			leading := len(raw) - len(strings.TrimLeft(raw, " "))
			if leading >= blockIndent {
				blockBuf.WriteString(raw[blockIndent:])
				blockBuf.WriteString("\n")
				continue
			}
			flushBlock()
			// fall through
		}

		trimmed := strings.TrimRight(raw, " \t")
		if t := strings.TrimSpace(trimmed); strings.HasPrefix(t, "#") || t == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "locale:") {
			fx.Locale = trimQuoted(strings.TrimPrefix(trimmed, "locale:"))
			cur = sectionNone
			continue
		}
		if strings.HasPrefix(trimmed, "description:") {
			fx.Description = trimQuoted(strings.TrimPrefix(trimmed, "description:"))
			cur = sectionNone
			continue
		}
		if trimmed == "requests:" {
			cur = sectionRequests
			curReq = nil
			continue
		}

		if cur == sectionRequests {
			if strings.HasPrefix(trimmed, "  - method:") {
				flushBlock()
				fx.Requests = append(fx.Requests, fixtureRequest{
					Method: trimQuoted(strings.TrimPrefix(trimmed, "  - method:")),
				})
				curReq = &fx.Requests[len(fx.Requests)-1]
				continue
			}
			if curReq == nil {
				continue
			}
			switch {
			case strings.HasPrefix(trimmed, "    path:"):
				curReq.Path = trimQuoted(strings.TrimPrefix(trimmed, "    path:"))
			case strings.HasPrefix(trimmed, "    origin:"):
				curReq.Origin = trimQuoted(strings.TrimPrefix(trimmed, "    origin:"))
			case strings.HasPrefix(trimmed, "    body:"):
				value := strings.TrimSpace(strings.TrimPrefix(trimmed, "    body:"))
				if value == "|" {
					inBlockScalar = true
					blockBuf.Reset()
					blockIndent = 6
					blockTarget = &curReq.Body
				} else {
					curReq.Body = trimQuoted(value)
				}
			case strings.HasPrefix(trimmed, "    expect_status:"):
				curReq.ExpectStatus = parseInt(
					strings.TrimSpace(strings.TrimPrefix(trimmed, "    expect_status:")),
				)
			case strings.HasPrefix(trimmed, "    expect_body:"):
				value := strings.TrimSpace(strings.TrimPrefix(trimmed, "    expect_body:"))
				curReq.ExpectBody = trimQuoted(value)
			case strings.HasPrefix(trimmed, "    expect_body_contains:"):
				curReq.ExpectBodyContains = trimQuoted(
					strings.TrimPrefix(trimmed, "    expect_body_contains:"),
				)
			case strings.HasPrefix(trimmed, "    expect_cors_origin:"):
				curReq.ExpectCORSOrigin = trimQuoted(
					strings.TrimPrefix(trimmed, "    expect_cors_origin:"),
				)
			case strings.HasPrefix(trimmed, "    expect_request_id_set:"):
				value := strings.TrimSpace(strings.TrimPrefix(trimmed, "    expect_request_id_set:"))
				curReq.ExpectRequestIDSet = (value == "true")
			}
		}
	}
	flushBlock()

	if fx.Locale == "" {
		return fx, fmt.Errorf("missing required key: locale")
	}
	if len(fx.Requests) == 0 {
		return fx, fmt.Errorf("fixture has zero requests")
	}
	return fx, nil
}

func trimQuoted(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "\n")
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		s = s[1 : len(s)-1]
	}
	return s
}

func parseInt(s string) int {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "runner-error: "+format+"\n", args...)
	os.Exit(2)
}
