package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHTTPTraceMiddlewareCapturesRequestAndResponse(t *testing.T) {
	writer := &memoryHTTPTraceWriter{}
	clock := newSequenceClock(
		time.Unix(100, 0).UTC(),
		time.Unix(102, 0).UTC(),
		time.Unix(103, 0).UTC(),
	)
	m := NewHTTPTraceMiddleware(writer, WithHTTPTraceClock(clock.Now))
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, "{\"echo\":%q}", string(data))
	}))
	request := httptest.NewRequest(http.MethodPost, "http://localhost:8080/v1/run?beta=true", strings.NewReader(`{"prompt":"hi"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-API-Key", "sk-1234567890ABCDE")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	evt := writer.last()
	if evt == nil {
		t.Fatal("expected trace event")
	}
	if evt.Request.Method != http.MethodPost {
		t.Fatalf("method mismatch: %s", evt.Request.Method)
	}
	if evt.Request.URL != "http://localhost:8080/v1/run?beta=true" {
		t.Fatalf("url mismatch: %s", evt.Request.URL)
	}
	if evt.Request.Headers["x-api-key"] == "sk-1234567890ABCDE" {
		t.Fatalf("x-api-key not masked: %v", evt.Request.Headers["x-api-key"])
	}
	body, ok := evt.Request.Body.(map[string]any)
	if !ok || body["prompt"] != "hi" {
		t.Fatalf("unexpected request body: %#v", evt.Request.Body)
	}
	if evt.Response.Status != http.StatusOK {
		t.Fatalf("status mismatch: %d", evt.Response.Status)
	}
	if evt.Response.Headers["content-type"] != "application/json" {
		t.Fatalf("missing response header: %#v", evt.Response.Headers)
	}
	if evt.Response.BodyRaw != `{"echo":"{\"prompt\":\"hi\"}"}` {
		t.Fatalf("unexpected response body: %s", evt.Response.BodyRaw)
	}
	if got := evt.Request.Timestamp; got != 100 {
		t.Fatalf("request timestamp mismatch: %v", got)
	}
	if got := evt.Response.Timestamp; got != 102 {
		t.Fatalf("response timestamp mismatch: %v", got)
	}
	wantedLoggedAt := time.Unix(103, 0).UTC().Format(time.RFC3339Nano)
	if evt.LoggedAt != wantedLoggedAt {
		t.Fatalf("logged_at mismatch: %s", evt.LoggedAt)
	}
	masked := evt.Request.Headers["x-api-key"]
	if !strings.HasPrefix(masked, "sk-12") || !strings.HasSuffix(masked, "ABCDE") {
		t.Fatalf("unexpected mask: %s", masked)
	}
}

func TestHTTPTraceMiddlewareCapturesStreamingResponse(t *testing.T) {
	writer := &memoryHTTPTraceWriter{}
	clock := newSequenceClock(
		time.Unix(200, 0).UTC(),
		time.Unix(201, 0).UTC(),
		time.Unix(202, 0).UTC(),
	)
	m := NewHTTPTraceMiddleware(writer, WithHTTPTraceClock(clock.Now))
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("response writer missing http.Flusher")
		}
		chunks := []string{"data: one\n\n", "data: two\n\n"}
		for _, chunk := range chunks {
			if _, err := w.Write([]byte(chunk)); err != nil {
				t.Fatalf("write chunk: %v", err)
			}
			flusher.Flush()
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "http://localhost/stream", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	evt := writer.last()
	if evt == nil {
		t.Fatal("expected trace event")
	}
	if evt.Response.BodyRaw != "data: one\n\ndata: two\n\n" {
		t.Fatalf("unexpected streaming body: %q", evt.Response.BodyRaw)
	}
}

func TestHTTPTraceMiddlewareTruncatesLargeBodies(t *testing.T) {
	writer := &memoryHTTPTraceWriter{}
	clock := newSequenceClock(
		time.Unix(300, 0).UTC(),
		time.Unix(301, 0).UTC(),
		time.Unix(302, 0).UTC(),
	)
	m := NewHTTPTraceMiddleware(writer,
		WithHTTPTraceClock(clock.Now),
		WithHTTPTraceMaxBodyBytes(5),
	)
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if _, err := w.Write([]byte("abcdefghij")); err != nil {
			t.Fatalf("write response body: %v", err)
		}
	}))
	req := httptest.NewRequest(http.MethodPost, "http://localhost/large", strings.NewReader("0123456789"))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	evt := writer.last()
	if evt == nil {
		t.Fatal("expected trace event")
	}
	if evt.Request.Body != "01234...(truncated)" {
		t.Fatalf("unexpected truncated request: %v", evt.Request.Body)
	}
	if evt.Response.BodyRaw != "abcde...(truncated)" {
		t.Fatalf("unexpected truncated response: %v", evt.Response.BodyRaw)
	}
}

type memoryHTTPTraceWriter struct {
	mu     sync.Mutex
	events []*HTTPTraceEvent
}

func (w *memoryHTTPTraceWriter) WriteHTTPTrace(event *HTTPTraceEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	dup := *event
	dup.Request.Headers = copyMap(event.Request.Headers)
	dup.Response.Headers = copyMap(event.Response.Headers)
	w.events = append(w.events, &dup)
	return nil
}

func (w *memoryHTTPTraceWriter) last() *HTTPTraceEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.events) == 0 {
		return nil
	}
	return w.events[len(w.events)-1]
}

func copyMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dup := make(map[string]string, len(src))
	for k, v := range src {
		dup[k] = v
	}
	return dup
}

type sequenceClock struct {
	mu       sync.Mutex
	times    []time.Time
	fallback time.Time
}

func newSequenceClock(times ...time.Time) *sequenceClock {
	return &sequenceClock{times: append([]time.Time(nil), times...), fallback: time.Unix(0, 0).UTC()}
}

func (c *sequenceClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.times) == 0 {
		return c.fallback
	}
	t := c.times[0]
	c.times = c.times[1:]
	return t
}
