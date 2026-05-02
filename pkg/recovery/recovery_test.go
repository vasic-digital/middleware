package recovery

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.NotNil(t, cfg.Output)
	assert.True(t, cfg.PrintStack)
	assert.NotEmpty(t, cfg.ResponseBody)
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
