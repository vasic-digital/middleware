package recovery

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.NotNil(t, cfg.Output)
	assert.True(t, cfg.PrintStack)
	// Per CONST-046 the default body is rendered at request time from the
	// Translator, so ResponseBody is intentionally nil and a Translator
	// is wired instead.
	assert.Nil(t, cfg.ResponseBody)
	assert.NotNil(t, cfg.Translator)
	assert.Equal(t, "text/plain; charset=utf-8", cfg.ResponseContentType)
}

func TestNew_RecoversPanic(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Output:              &buf,
		PrintStack:          false,
		ResponseBody:        []byte("Internal Server Error\n"),
		ResponseContentType: "text/plain; charset=utf-8",
	}
	middleware := New(cfg)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	// Should not panic.
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "Internal Server Error")
	assert.Contains(t, buf.String(), "[RECOVERY]")
	assert.Contains(t, buf.String(), "something went wrong")
}

func TestNew_RecoversPanicWithStackTrace(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Output:              &buf,
		PrintStack:          true,
		ResponseBody:        []byte("error\n"),
		ResponseContentType: "text/plain; charset=utf-8",
	}
	middleware := New(cfg)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("stack trace test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	logOutput := buf.String()
	assert.Contains(t, logOutput, "stack trace test")
	assert.Contains(t, logOutput, "goroutine")
}

func TestNew_NoPanicPassesThrough(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Output:              &buf,
		PrintStack:          true,
		ResponseBody:        []byte("error\n"),
		ResponseContentType: "text/plain; charset=utf-8",
	}
	middleware := New(cfg)

	called := false
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("all good"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "all good", rec.Body.String())
	assert.Empty(t, buf.String())
}

func TestNew_CustomResponseBody(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Output:              &buf,
		PrintStack:          false,
		ResponseBody:        []byte(`{"error":"internal_error"}`),
		ResponseContentType: "application/json",
	}
	middleware := New(cfg)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("json error test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/fail", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, `{"error":"internal_error"}`, rec.Body.String())
}

func TestNew_RecoversPanicWithNonStringValue(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Output:              &buf,
		PrintStack:          false,
		ResponseBody:        []byte("error\n"),
		ResponseContentType: "text/plain; charset=utf-8",
	}
	middleware := New(cfg)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(42)
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic-int", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, buf.String(), "42")
}

// recordingTranslator captures the message ID it was asked to render so
// the i18n wiring can be asserted on a user-visible surface (the HTTP
// 500 body), not on a call-count. It returns a recognisable localised
// stand-in to prove the body really flows through the Translator.
type recordingTranslator struct {
	lastID string
	render string
}

func (rt *recordingTranslator) T(_ context.Context, msgID string, _ map[string]any) string {
	rt.lastID = msgID
	if rt.render != "" {
		return rt.render
	}
	return msgID
}

func (rt *recordingTranslator) TPlural(_ context.Context, msgID string, _ int, _ map[string]any) string {
	rt.lastID = msgID
	return msgID
}

// TestNew_PanicBodyFromTranslator proves the panic 500 body is rendered
// through the configured i18n.Translator (CONST-046) — a consuming
// project wiring a localised Translator gets a localised body in the
// HTTP response a real end user receives.
func TestNew_PanicBodyFromTranslator(t *testing.T) {
	var buf bytes.Buffer
	tr := &recordingTranslator{render: "Interná chyba servera"}
	cfg := &Config{
		Output:     &buf,
		PrintStack: false,
		Translator: tr,
		// ResponseBody intentionally nil — body comes from Translator.
	}
	middleware := New(cfg)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("translated body test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	// Primary assertion: user-visible HTTP body carries the localised text.
	assert.Equal(t, "Interná chyba servera\n", rec.Body.String())
	// The Translator was asked for the correct, namespaced message ID.
	assert.Equal(t, "middleware_recovery_internal_server_error", tr.lastID)
}

// TestNew_PanicBodyNoopTranslatorVerbatimID is the paired-mutation guard:
// with the default NoopTranslator the body is the verbatim message ID
// (loud absence-of-bundle evidence per CONST-035), NOT a hardcoded
// English literal. If a regression reintroduces a hardcoded
// "Internal Server Error" literal in recovery.go, this test fails
// because the body would no longer equal the message ID.
func TestNew_PanicBodyNoopTranslatorVerbatimID(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Output:     &buf,
		PrintStack: false,
		// No Translator, no ResponseBody — New() must default to Noop.
	}
	middleware := New(cfg)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("noop body test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "middleware_recovery_internal_server_error\n", rec.Body.String())
}

// TestNew_ExplicitResponseBodyWinsOverTranslator proves the documented
// precedence: an explicit Config.ResponseBody is returned verbatim even
// when a Translator is wired (backward compatibility for consumers that
// already supply a fixed body).
func TestNew_ExplicitResponseBodyWinsOverTranslator(t *testing.T) {
	var buf bytes.Buffer
	tr := &recordingTranslator{render: "should-not-appear"}
	cfg := &Config{
		Output:       &buf,
		PrintStack:   false,
		ResponseBody: []byte(`{"error":"boom"}`),
		Translator:   tr,
	}
	middleware := New(cfg)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("precedence test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, `{"error":"boom"}`, rec.Body.String())
	assert.Empty(t, tr.lastID, "Translator must not be consulted when ResponseBody is set")
}
