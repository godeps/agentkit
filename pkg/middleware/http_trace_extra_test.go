package middleware

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileHTTPTraceWriterLifecycle(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewFileHTTPTraceWriter(dir)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}
	if writer.Path() == "" {
		t.Fatalf("path not set")
	}
	if _, err := os.Stat(writer.Path()); err != nil {
		t.Fatalf("stat path: %v", err)
	}

	event := &HTTPTraceEvent{
		Request:  HTTPTraceRequest{Method: http.MethodGet, URL: "http://localhost"},
		Response: HTTPTraceResponse{Status: http.StatusAccepted},
		LoggedAt: "now",
	}
	if err := writer.WriteHTTPTrace(event); err != nil {
		t.Fatalf("write event: %v", err)
	}
	if err := writer.WriteHTTPTrace(nil); err == nil || !strings.Contains(err.Error(), "event is nil") {
		t.Fatalf("expected nil event error, got %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := writer.Close(); err != nil { // idempotent
		t.Fatalf("double close: %v", err)
	}
	if err := writer.WriteHTTPTrace(event); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("expected closed error, got %v", err)
	}

	var nilWriter *FileHTTPTraceWriter
	if got := nilWriter.Path(); got != "" {
		t.Fatalf("nil writer path should be empty, got %q", got)
	}
	if err := nilWriter.WriteHTTPTrace(event); err == nil || !strings.Contains(err.Error(), "writer is nil") {
		t.Fatalf("nil writer should error, got %v", err)
	}
}

func TestNewFileHTTPTraceWriterError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := NewFileHTTPTraceWriter(path); err == nil {
		t.Fatalf("expected mkdir error when path is a file")
	}
}

func TestFileHTTPTraceWriterMarshalError(t *testing.T) {
	writer, err := NewFileHTTPTraceWriter(t.TempDir())
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}
	defer writer.Close()

	event := &HTTPTraceEvent{
		Request:  HTTPTraceRequest{Body: make(chan int)},
		Response: HTTPTraceResponse{},
	}
	if err := writer.WriteHTTPTrace(event); err == nil {
		t.Fatalf("expected marshal error")
	}
}

func TestHTTPTraceHandlerFallsBackWithoutWriter(t *testing.T) {
	m := &HTTPTraceMiddleware{}
	called := false
	h := m.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/noop", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !called {
		t.Fatalf("next handler not invoked")
	}
}

func TestHTTPTraceMiddlewareNilWrap(t *testing.T) {
	var m *HTTPTraceMiddleware
	called := false
	handler := m.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/nil", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if !called {
		t.Fatalf("nil middleware should pass through")
	}
}

func TestHTTPTraceWrapDefaultsHandler(t *testing.T) {
	m := &HTTPTraceMiddleware{}
	handler := m.Wrap(nil)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/noop", nil))
}

func TestBuildFullURLVariants(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want string
	}{
		{"nil request", nil, ""},
		{"tls host", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "http://example.com/path?p=1", nil)
			r.TLS = &tls.ConnectionState{}
			return r
		}(), "https://example.com/path?p=1"},
		{"forwarded proto", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
			r.Header.Set("X-Forwarded-Proto", "HTTPS")
			return r
		}(), "https://example.com/path"},
		{"url host fallback", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "/onlypath", nil)
			r.Host = ""
			r.URL.Host = "api.test"
			r.URL.Scheme = "http"
			return r
		}(), "http://api.test/onlypath"},
		{"url only", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "/rel", nil)
			r.Host = ""
			r.URL.Host = ""
			return r
		}(), "/rel"},
	}
	for _, tt := range tests {
		if got := buildFullURL(tt.req); got != tt.want {
			t.Fatalf("%s: got %q want %q", tt.name, got, tt.want)
		}
	}
}

func TestCloneTraceHeadersAndMasking(t *testing.T) {
	if got := cloneTraceHeaders(http.Header{}); got != nil {
		t.Fatalf("empty header should return nil")
	}

	h := http.Header{}
	h["X-Empty"] = []string{}
	if got := cloneTraceHeaders(h); got != nil {
		t.Fatalf("empty values should be skipped")
	}

	h = http.Header{"Authorization": []string{"token-abcdefghijkl"}}
	h.Set("Custom", "ok")
	cloned := cloneTraceHeaders(h)
	if cloned["custom"] != "ok" {
		t.Fatalf("custom header missing: %#v", cloned)
	}
	if strings.Contains(cloned["authorization"], "token-") {
		t.Fatalf("authorization header not masked: %s", cloned["authorization"])
	}

	if got := maskToken("short"); got != "*****" {
		t.Fatalf("short token masking failed: %s", got)
	}
}

func TestBodyHelpers(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/keep", strings.NewReader("keepme"))
	body, truncated, err := readAndReplaceBody(req, 0)
	if err != nil || truncated || body != nil {
		t.Fatalf("limit zero should skip read: body=%v truncated=%v err=%v", body, truncated, err)
	}
	data, err := io.ReadAll(req.Body)
	if err != nil || string(data) != "keepme" {
		t.Fatalf("body should remain readable, got %q err=%v", string(data), err)
	}

	full, trunc, err := readBody(io.NopCloser(strings.NewReader("abc")), -1)
	if err != nil || trunc || string(full) != "abc" {
		t.Fatalf("read all failed: %q trunc=%v err=%v", string(full), trunc, err)
	}
	partial, trunc, err := readBody(io.NopCloser(strings.NewReader("abcd")), 2)
	if err != nil || !trunc || string(partial) != "ab" {
		t.Fatalf("truncated read failed: %q trunc=%v err=%v", string(partial), trunc, err)
	}

	if got := decodeRequestBody(nil, true); got != "(truncated)" {
		t.Fatalf("decode truncated nil mismatch: %v", got)
	}
	if got := formatRawBody([]byte("abc"), true); got != "abc...(truncated)" {
		t.Fatalf("format truncated: %s", got)
	}
	if got := formatRawBody(nil, false); got != "" {
		t.Fatalf("empty format mismatch: %q", got)
	}
	if got := unixFloat(time.Time{}); got != 0 {
		t.Fatalf("zero time should return 0, got %v", got)
	}
	if got := unixFloat(time.Unix(5, 0)); got != 5 {
		t.Fatalf("unix float mismatch: %v", got)
	}
}

func TestResponseRecorderCaptureAndHijack(t *testing.T) {
	base := &stubResponseWriter{header: http.Header{}}
	rr := newResponseRecorder(base, 5)

	if _, err := rr.Write([]byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if rr.statusCode() != http.StatusOK {
		t.Fatalf("default status mismatch: %d", rr.statusCode())
	}
	if rr.bodyTruncated() {
		t.Fatalf("body should not be truncated")
	}
	if got := string(rr.bodyBytes()); got != "hello" {
		t.Fatalf("body capture mismatch: %q", got)
	}

	rr.WriteHeader(http.StatusAccepted) // second header call
	if base.status != http.StatusAccepted {
		t.Fatalf("forwarded header missing: %d", base.status)
	}

	if _, err := rr.Write([]byte("world")); err != nil {
		t.Fatalf("write second chunk: %v", err)
	}
	if !rr.bodyTruncated() {
		t.Fatalf("expected truncation after second write")
	}
	if got := string(rr.bodyBytes()); got != "hello" {
		t.Fatalf("truncated buffer mismatch: %q", got)
	}

	if _, _, err := rr.Hijack(); err == nil {
		t.Fatalf("expected hijack unsupported error")
	}
}

func TestResponseRecorderHijackAndFlush(t *testing.T) {
	base := &hijackableWriter{header: http.Header{}, flusherCalled: false}
	rr := newResponseRecorder(base, 0)

	rr.Flush()
	if !base.flusherCalled {
		t.Fatalf("flush not forwarded")
	}

	conn, rw, err := rr.Hijack()
	if err != nil {
		t.Fatalf("hijack: %v", err)
	}
	if rw == nil {
		t.Fatalf("readwriter nil")
	}
	if conn != nil {
		_ = conn.Close()
	}
}

func TestResponseRecorderDefaults(t *testing.T) {
	rr := newResponseRecorder(nil, 0)
	if rr == nil {
		t.Fatalf("recorder nil")
	}
	if rr.Header() == nil {
		t.Fatalf("default header should not be nil")
	}
	if rr.statusCode() != http.StatusOK {
		t.Fatalf("default status should be 200, got %d", rr.statusCode())
	}
	if rr.bodyBytes() != nil {
		t.Fatalf("empty body should return nil slice")
	}
}

func TestResponseRecorderStatusCodeZero(t *testing.T) {
	rr := &responseRecorder{}
	if rr.statusCode() != http.StatusOK {
		t.Fatalf("zero status should map to 200, got %d", rr.statusCode())
	}
}

type stubResponseWriter struct {
	header http.Header
	status int
	buf    bytes.Buffer
}

func (s *stubResponseWriter) Header() http.Header { return s.header }
func (s *stubResponseWriter) WriteHeader(status int) {
	s.status = status
}
func (s *stubResponseWriter) Write(p []byte) (int, error) {
	return s.buf.Write(p)
}

type hijackableWriter struct {
	header        http.Header
	status        int
	flusherCalled bool
}

func (h *hijackableWriter) Header() http.Header         { return h.header }
func (h *hijackableWriter) WriteHeader(status int)      { h.status = status }
func (h *hijackableWriter) Write(p []byte) (int, error) { return len(p), nil }
func (h *hijackableWriter) Flush()                      { h.flusherCalled = true }
func (h *hijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, bufio.NewReadWriter(bufio.NewReader(strings.NewReader("")), bufio.NewWriter(io.Discard)), nil
}
