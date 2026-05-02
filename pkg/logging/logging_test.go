package logging

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
	assert.NotNil(t, cfg.SkipPaths)
	assert.Empty(t, cfg.SkipPaths)
}

func TestNew_LogsRequestDetails(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Output:    &buf,
		SkipPaths: make(map[string]struct{}),
	}
	middleware := New(cfg)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "[HTTP]")
	assert.Contains(t, logOutput, "GET")
	assert.Contains(t, logOutput, "/api/test")
	assert.Contains(t, logOutput, "200")
}

func TestNew_LogsNonOKStatus(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Output:    &buf,
		SkipPaths: make(map[string]struct{}),
	}
	middleware := New(cfg)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodPost, "/missing", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "POST")
	assert.Contains(t, logOutput, "/missing")
	assert.Contains(t, logOutput, "404")
}

func TestNew_SkipPaths(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Output: &buf,
		SkipPaths: map[string]struct{}{
			"/health": {},
		},
	}
	middleware := New(cfg)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request to skipped path should not log.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Empty(t, buf.String())

	// Request to non-skipped path should log.
	req = httptest.NewRequest(http.MethodGet, "/api/data", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Contains(t, buf.String(), "/api/data")
}

func TestNew_LogsDefaultStatusWhenWriteOnly(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Output:    &buf,
		SkipPaths: make(map[string]struct{}),
	}
	middleware := New(cfg)

	// Handler that calls Write without explicitly calling WriteHeader.
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/implicit-ok", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "200")
	assert.Contains(t, logOutput, "/implicit-ok")
}

func TestNew_PassesThroughToHandler(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Output:    &buf,
		SkipPaths: make(map[string]struct{}),
	}
	middleware := New(cfg)

	called := false
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/create", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "created", rec.Body.String())
}
