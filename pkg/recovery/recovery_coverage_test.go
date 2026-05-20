package recovery

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew_NilOutput covers the cfg.Output == nil branch in New(), where
// a Config is provided but Output is nil. It should default to os.Stderr.
func TestNew_NilOutput(t *testing.T) {
	cfg := &Config{
		Output:              nil, // triggers the nil Output branch
		PrintStack:          false,
		ResponseBody:        []byte("error\n"),
		ResponseContentType: "text/plain; charset=utf-8",
	}
	middleware := New(cfg)
	require.NotNil(t, middleware)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("nil output test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "error")
}

// TestNew_NilResponseBody covers the cfg.ResponseBody == nil branch in New(),
// where a Config is provided but ResponseBody is nil. Per CONST-046 the
// body is then rendered through the configured Translator; with the
// default NoopTranslator the verbatim message ID is returned so
// absence-of-bundle is loudly visible.
func TestNew_NilResponseBody(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Output:              &buf,
		PrintStack:          false,
		ResponseBody:        nil, // triggers the nil ResponseBody branch
		ResponseContentType: "text/plain; charset=utf-8",
	}
	middleware := New(cfg)
	require.NotNil(t, middleware)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("nil body test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "middleware_recovery_internal_server_error\n", rec.Body.String())
}

// TestNew_EmptyResponseContentType covers the cfg.ResponseContentType == ""
// branch in New(), where a Config is provided but ResponseContentType is
// empty. It should default to "text/plain; charset=utf-8".
func TestNew_EmptyResponseContentType(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Output:              &buf,
		PrintStack:          false,
		ResponseBody:        []byte("fail\n"),
		ResponseContentType: "", // triggers the empty ContentType branch
	}
	middleware := New(cfg)
	require.NotNil(t, middleware)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("empty content type test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
}

// TestNew_AllNilDefaults covers the case where all optional Config fields are
// at their zero values, triggering all default branches simultaneously.
func TestNew_AllNilDefaults(t *testing.T) {
	cfg := &Config{
		Output:              nil,
		PrintStack:          false,
		ResponseBody:        nil,
		ResponseContentType: "",
	}
	middleware := New(cfg)
	require.NotNil(t, middleware)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("all defaults test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	// Per CONST-046: nil ResponseBody + default NoopTranslator yields the
	// verbatim message ID, not a hardcoded English literal.
	assert.Equal(t, "middleware_recovery_internal_server_error\n", rec.Body.String())
}

// TestNew_PanicWithError covers panic with an error value (not a string or int).
func TestNew_PanicWithError(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Output:              &buf,
		PrintStack:          false,
		ResponseBody:        []byte("error\n"),
		ResponseContentType: "text/plain; charset=utf-8",
	}
	middleware := New(cfg)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrBodyNotAllowed)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, buf.String(), "[RECOVERY]")
}

// TestNew_PrintStackTrue_WithNilOutput covers PrintStack=true together
// with nil Output (both default branches active at once, plus stack printing).
func TestNew_PrintStackTrue_WithNilOutput(t *testing.T) {
	cfg := &Config{
		Output:              nil,
		PrintStack:          true,
		ResponseBody:        []byte("error\n"),
		ResponseContentType: "text/plain; charset=utf-8",
	}
	middleware := New(cfg)
	require.NotNil(t, middleware)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("stack with nil output")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// Should not panic; recovery should handle it gracefully.
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestNew_CustomAllFields covers a fully customized Config with all fields
// explicitly set to non-default values.
func TestNew_CustomAllFields(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Output:              &buf,
		PrintStack:          true,
		ResponseBody:        []byte(`{"error":"panic"}`),
		ResponseContentType: "application/json; charset=utf-8",
	}
	middleware := New(cfg)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("custom fields test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/custom", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, `{"error":"panic"}`, rec.Body.String())
	assert.Contains(t, buf.String(), "custom fields test")
	assert.Contains(t, buf.String(), "goroutine") // stack trace present
}
