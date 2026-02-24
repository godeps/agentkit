package middleware

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultHTTPTraceDir       = ".claude-trace"
	defaultHTTPTraceBodyLimit = 1 << 20 // 1 MiB safeguard per payload
)

// HTTPTraceEvent captures a single HTTP exchange in JSONL-friendly form.
type HTTPTraceEvent struct {
	Request  HTTPTraceRequest  `json:"request"`
	Response HTTPTraceResponse `json:"response"`
	LoggedAt string            `json:"logged_at"`
}

type HTTPTraceRequest struct {
	Timestamp float64           `json:"timestamp"`
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      any               `json:"body,omitempty"`
}

type HTTPTraceResponse struct {
	Timestamp float64           `json:"timestamp"`
	Status    int               `json:"status_code"`
	Headers   map[string]string `json:"headers,omitempty"`
	BodyRaw   string            `json:"body_raw,omitempty"`
}

// HTTPTraceWriter persists events. Implementations must be concurrency safe.
type HTTPTraceWriter interface {
	WriteHTTPTrace(event *HTTPTraceEvent) error
}

// FileHTTPTraceWriter appends JSONL events to a single file.
type FileHTTPTraceWriter struct {
	mu   sync.Mutex
	file *os.File
	path string
}

// NewFileHTTPTraceWriter creates/opens a JSONL log file under dir following
// Claude Code's log naming convention.
func NewFileHTTPTraceWriter(dir string) (*FileHTTPTraceWriter, error) {
	d := strings.TrimSpace(dir)
	if d == "" {
		d = defaultHTTPTraceDir
	}
	if err := os.MkdirAll(d, 0o755); err != nil {
		return nil, fmt.Errorf("http trace: mkdir %s: %w", d, err)
	}
	name := fmt.Sprintf("log-%s.jsonl", time.Now().UTC().Format("2006-01-02-15-04-05"))
	path := filepath.Join(d, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("http trace: open %s: %w", path, err)
	}
	return &FileHTTPTraceWriter{file: f, path: path}, nil
}

// Path returns the backing file path for observability/testing.
func (w *FileHTTPTraceWriter) Path() string {
	if w == nil {
		return ""
	}
	return w.path
}

// Close releases the underlying file handle.
func (w *FileHTTPTraceWriter) Close() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

// WriteHTTPTrace writes the event as a single JSON line.
func (w *FileHTTPTraceWriter) WriteHTTPTrace(event *HTTPTraceEvent) error {
	if w == nil {
		return errors.New("http trace: writer is nil")
	}
	if event == nil {
		return errors.New("http trace: event is nil")
	}
	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("http trace: marshal: %w", err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return errors.New("http trace: file closed")
	}
	if _, err := w.file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("http trace: write: %w", err)
	}
	return nil
}

// HTTPTraceMiddleware captures inbound/outbound payloads at the HTTP layer.
type HTTPTraceMiddleware struct {
	writer       HTTPTraceWriter
	maxBodyBytes int64
	clock        func() time.Time
}

// HTTPTraceOption configures the middleware.
type HTTPTraceOption func(*HTTPTraceMiddleware)

// NewHTTPTraceMiddleware wires the middleware with the provided writer.
func NewHTTPTraceMiddleware(writer HTTPTraceWriter, opts ...HTTPTraceOption) *HTTPTraceMiddleware {
	m := &HTTPTraceMiddleware{
		writer:       writer,
		maxBodyBytes: defaultHTTPTraceBodyLimit,
		clock:        time.Now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	return m
}

// WithHTTPTraceMaxBodyBytes overrides the capture limit.
// limit == 0 disables capture, limit <0 captures the full payload.
func WithHTTPTraceMaxBodyBytes(limit int64) HTTPTraceOption {
	return func(m *HTTPTraceMiddleware) {
		m.maxBodyBytes = limit
	}
}

// WithHTTPTraceClock injects a deterministic clock (useful for tests).
func WithHTTPTraceClock(clock func() time.Time) HTTPTraceOption {
	return func(m *HTTPTraceMiddleware) {
		if clock != nil {
			m.clock = clock
		}
	}
}

// Wrap returns an http.Handler that records a trace per request.
func (m *HTTPTraceMiddleware) Wrap(next http.Handler) http.Handler {
	if next == nil {
		next = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m == nil || m.writer == nil {
			next.ServeHTTP(w, r)
			return
		}
		reqTime := m.clock()
		requestHeaders := cloneTraceHeaders(r.Header)
		bodyBytes, bodyTruncated, err := readAndReplaceBody(r, m.maxBodyBytes)
		if err != nil {
			log.Printf("http trace: read request body: %v", err)
		}
		req := HTTPTraceRequest{
			Timestamp: unixFloat(reqTime),
			Method:    r.Method,
			URL:       buildFullURL(r),
			Headers:   requestHeaders,
			Body:      decodeRequestBody(bodyBytes, bodyTruncated),
		}
		recorder := newResponseRecorder(w, m.maxBodyBytes)
		var panicVal any
		defer func() {
			resp := HTTPTraceResponse{
				Timestamp: unixFloat(m.clock()),
				Status:    recorder.statusCode(),
				Headers:   cloneTraceHeaders(recorder.Header()),
				BodyRaw:   formatRawBody(recorder.bodyBytes(), recorder.bodyTruncated()),
			}
			evt := HTTPTraceEvent{
				Request:  req,
				Response: resp,
				LoggedAt: m.clock().UTC().Format(time.RFC3339Nano),
			}
			if err := m.writer.WriteHTTPTrace(&evt); err != nil {
				log.Printf("http trace: write event: %v", err)
			}
			if panicVal != nil {
				panic(panicVal)
			}
		}()
		defer func() {
			if rec := recover(); rec != nil {
				panicVal = rec
			}
		}()
		next.ServeHTTP(recorder, r)
	})
}

// Handler is an alias for Wrap to align with conventional middleware naming.
func (m *HTTPTraceMiddleware) Handler(next http.Handler) http.Handler {
	return m.Wrap(next)
}

func buildFullURL(r *http.Request) string {
	if r == nil {
		return ""
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = strings.ToLower(proto)
	}
	host := r.Host
	if host == "" {
		host = r.URL.Host
	}
	if host == "" {
		return r.URL.String()
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, r.URL.RequestURI())
}

func cloneTraceHeaders(h http.Header) map[string]string {
	if len(h) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) == 0 {
			continue
		}
		name := strings.ToLower(k)
		cloned[name] = maskSensitiveHeader(name, strings.Join(v, ","))
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}

func maskSensitiveHeader(name, value string) string {
	if value == "" {
		return ""
	}
	lower := strings.ToLower(name)
	if strings.Contains(lower, "api-key") || lower == "authorization" {
		return maskToken(value)
	}
	return value
}

func maskToken(v string) string {
	runes := []rune(v)
	if len(runes) <= 10 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:5]) + "..." + string(runes[len(runes)-5:])
}

func readAndReplaceBody(r *http.Request, limit int64) ([]byte, bool, error) {
	if r == nil || r.Body == nil || limit == 0 {
		return nil, false, nil
	}
	data, truncated, err := readBody(r.Body, limit)
	if err != nil {
		return nil, false, err
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return data, truncated, nil
}

func readBody(rc io.ReadCloser, limit int64) ([]byte, bool, error) {
	if rc == nil {
		return nil, false, nil
	}
	defer rc.Close()
	if limit < 0 {
		data, err := io.ReadAll(rc)
		return data, false, err
	}
	reader := io.LimitReader(rc, limit+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) <= limit {
		return data, false, nil
	}
	return data[:limit], true, nil
}

func decodeRequestBody(data []byte, truncated bool) any {
	trim := bytes.TrimSpace(data)
	if len(trim) == 0 {
		if truncated {
			return "(truncated)"
		}
		return nil
	}
	if !truncated {
		var out any
		if err := json.Unmarshal(trim, &out); err == nil {
			return out
		}
	}
	return formatRawBody(trim, truncated)
}

func formatRawBody(data []byte, truncated bool) string {
	if len(data) == 0 {
		if truncated {
			return "(truncated)"
		}
		return ""
	}
	if truncated {
		return string(data) + "...(truncated)"
	}
	return string(data)
}

func unixFloat(t time.Time) float64 {
	if t.IsZero() {
		return 0
	}
	return float64(t.UnixNano()) / 1e9
}

type responseRecorder struct {
	http.ResponseWriter
	status       int
	wroteHeader  bool
	bodyLimit    int64
	bodyBuf      bytes.Buffer
	wasTruncated bool
	hijacker     http.Hijacker
	flusher      http.Flusher
}

func newResponseRecorder(w http.ResponseWriter, limit int64) *responseRecorder {
	rr := &responseRecorder{ResponseWriter: w, status: http.StatusOK, bodyLimit: limit}
	if hj, ok := w.(http.Hijacker); ok {
		rr.hijacker = hj
	}
	if fl, ok := w.(http.Flusher); ok {
		rr.flusher = fl
	}
	return rr
}

func (rr *responseRecorder) Header() http.Header {
	if rr.ResponseWriter == nil {
		return http.Header{}
	}
	return rr.ResponseWriter.Header()
}

func (rr *responseRecorder) WriteHeader(status int) {
	if rr.wroteHeader {
		rr.ResponseWriter.WriteHeader(status)
		return
	}
	rr.status = status
	rr.wroteHeader = true
	rr.ResponseWriter.WriteHeader(status)
}

func (rr *responseRecorder) Write(p []byte) (int, error) {
	if !rr.wroteHeader {
		rr.WriteHeader(http.StatusOK)
	}
	rr.capture(p)
	return rr.ResponseWriter.Write(p)
}

func (rr *responseRecorder) capture(p []byte) {
	if rr.bodyLimit == 0 {
		return
	}
	if rr.bodyLimit > 0 {
		remaining := rr.bodyLimit - int64(rr.bodyBuf.Len())
		if remaining <= 0 {
			rr.wasTruncated = true
			return
		}
		if int64(len(p)) > remaining {
			rr.bodyBuf.Write(p[:remaining])
			rr.wasTruncated = true
			return
		}
	}
	rr.bodyBuf.Write(p)
}

func (rr *responseRecorder) Flush() {
	if rr.flusher != nil {
		rr.flusher.Flush()
	}
}

func (rr *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if rr.hijacker == nil {
		return nil, nil, fmt.Errorf("http trace: hijacking not supported")
	}
	return rr.hijacker.Hijack()
}

func (rr *responseRecorder) statusCode() int {
	if rr.status == 0 {
		return http.StatusOK
	}
	return rr.status
}

func (rr *responseRecorder) bodyBytes() []byte {
	if rr.bodyBuf.Len() == 0 {
		return nil
	}
	return append([]byte(nil), rr.bodyBuf.Bytes()...)
}

func (rr *responseRecorder) bodyTruncated() bool { return rr.wasTruncated }
