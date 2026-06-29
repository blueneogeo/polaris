package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/polaris/internal/config"
)

// mockUpstream is a test server that simulates an OpenAI-compatible API.
type mockUpstream struct {
	server        *httptest.Server
	streamChunks  []string      // SSE chunks to return in stream mode
	nonStreamBody string        // body to return in non-stream mode
	errorStatus   int           // if > 0, return this error status
	slowWrite     time.Duration // simulate slow streaming for timeout/latency tests
}

func newMockUpstream(t *testing.T) *mockUpstream {
	t.Helper()
	m := &mockUpstream{
		streamChunks: []string{
			`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"}}]}`,
			`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":" world"}}]}`,
			`data: [DONE]`,
		},
		nonStreamBody: `{"id":"chatcmpl-1","object":"chat.completion","choices":[{"message":{"content":"Hello world"}}]}`,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.errorStatus > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(m.errorStatus)
			_, _ = w.Write([]byte(`{"error":{"message":"upstream error"}}`))
			return
		}

		body, _ := io.ReadAll(r.Body)
		r.Body.Close()

		var req chatRequest
		_ = json.Unmarshal(body, &req)

		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher := w.(http.Flusher)
			for _, chunk := range m.streamChunks {
				if m.slowWrite > 0 {
					time.Sleep(m.slowWrite)
				}
				_, _ = fmt.Fprintln(w, chunk)
				flusher.Flush()
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(m.nonStreamBody))
		}
	})

	m.server = httptest.NewServer(handler)
	return m
}

func (m *mockUpstream) URL() string {
	return m.server.URL
}

func (m *mockUpstream) Close() {
	m.server.Close()
}

// newTestProxy creates a test proxy server with the given upstream.
func newTestProxy(t *testing.T, upstreamURL string) *httptest.Server {
	t.Helper()
	cfg := &config.Config{
		Port:           "0",
		UpstreamURL:    upstreamURL,
		UpstreamAPIKey: "sk-test-key",
	}
	handler := NewHandler(cfg, slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	r := chi.NewRouter()
	r.Post("/v1/chat/completions", handler.ChatCompletions)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	return httptest.NewServer(r)
}

// ── Streaming tests ────────────────────────────────────────────────

func TestProxy_Streaming_PassesThroughSSEChunks(t *testing.T) {
	upstream := newMockUpstream(t)
	defer upstream.Close()

	proxy := newTestProxy(t, upstream.URL())
	defer proxy.Close()

	body := `{"model":"test-model","messages":[{"role":"user","content":"hi"}],"stream":true}`
	resp, err := http.Post(proxy.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	for _, chunk := range upstream.streamChunks {
		if !strings.Contains(string(responseBody), chunk) {
			t.Errorf("response body missing chunk: %q", chunk)
		}
	}
	if !strings.Contains(string(responseBody), "[DONE]") {
		t.Errorf("response body missing [DONE] marker")
	}
}

func TestProxy_Streaming_EmptyStream(t *testing.T) {
	upstream := newMockUpstream(t)
	upstream.streamChunks = []string{}
	defer upstream.Close()

	proxy := newTestProxy(t, upstream.URL())
	defer proxy.Close()

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}],"stream":true}`
	resp, err := http.Post(proxy.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// ── Non-streaming tests ────────────────────────────────────────────

func TestProxy_NonStreaming_PassesThroughResponse(t *testing.T) {
	upstream := newMockUpstream(t)
	defer upstream.Close()

	proxy := newTestProxy(t, upstream.URL())
	defer proxy.Close()

	body := `{"model":"test-model","messages":[{"role":"user","content":"hi"}],"stream":false}`
	resp, err := http.Post(proxy.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if string(responseBody) != upstream.nonStreamBody {
		t.Errorf("response body = %q, want %q", string(responseBody), upstream.nonStreamBody)
	}
}

func TestProxy_NonStreaming_StreamFalse(t *testing.T) {
	upstream := newMockUpstream(t)
	defer upstream.Close()

	proxy := newTestProxy(t, upstream.URL())
	defer proxy.Close()

	// stream: false explicitly
	body := `{"model":"test","messages":[{"role":"user","content":"hi"}],"stream":false}`
	resp, err := http.Post(proxy.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Error("non-streaming request should not return event-stream content type")
	}
}

// ── Error handling tests ───────────────────────────────────────────

func TestProxy_UpstreamError_400(t *testing.T) {
	upstream := newMockUpstream(t)
	upstream.errorStatus = http.StatusBadRequest
	defer upstream.Close()

	proxy := newTestProxy(t, upstream.URL())
	defer proxy.Close()

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}]}`
	resp, err := http.Post(proxy.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestProxy_UpstreamError_500(t *testing.T) {
	upstream := newMockUpstream(t)
	upstream.errorStatus = http.StatusInternalServerError
	defer upstream.Close()

	proxy := newTestProxy(t, upstream.URL())
	defer proxy.Close()

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}]}`
	resp, err := http.Post(proxy.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestProxy_InvalidJSON(t *testing.T) {
	upstream := newMockUpstream(t)
	defer upstream.Close()

	proxy := newTestProxy(t, upstream.URL())
	defer proxy.Close()

	body := `not json at all`
	resp, err := http.Post(proxy.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestProxy_EmptyBody(t *testing.T) {
	upstream := newMockUpstream(t)
	defer upstream.Close()

	proxy := newTestProxy(t, upstream.URL())
	defer proxy.Close()

	resp, err := http.Post(proxy.URL+"/v1/chat/completions", "application/json", strings.NewReader(""))
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestProxy_UpstreamUnreachable(t *testing.T) {
	cfg := &config.Config{
		Port:           "0",
		UpstreamURL:    "http://127.0.0.1:1", // invalid port, will fail to connect
		UpstreamAPIKey: "sk-test",
	}
	handler := NewHandler(cfg, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	r := chi.NewRouter()
	r.Post("/v1/chat/completions", handler.ChatCompletions)
	proxy := httptest.NewServer(r)
	defer proxy.Close()

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}]}`
	resp, err := http.Post(proxy.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	var apiErr struct {
		Status  int    `json:"status"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if apiErr.Code != "upstream_error" {
		t.Errorf("error code = %q, want %q", apiErr.Code, "upstream_error")
	}
}

// ── Header forwarding tests ────────────────────────────────────────

func TestProxy_ForwardsResponseHeaders(t *testing.T) {
	upstream := newMockUpstream(t)
	defer upstream.Close()

	proxy := newTestProxy(t, upstream.URL())
	defer proxy.Close()

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}],"stream":false}`
	resp, err := http.Post(proxy.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp.Body.Close()

	// Content-Type should be forwarded from upstream
	if ct := resp.Header.Get("Content-Type"); ct == "" {
		t.Error("Content-Type header not forwarded from upstream")
	}
}

// ── Large response test ────────────────────────────────────────────

func TestProxy_LargeNonStreamingResponse(t *testing.T) {
	upstream := newMockUpstream(t)
	// Create a large response body
	largeContent := strings.Repeat("hello world ", 10000)
	upstream.nonStreamBody = `{"id":"large","object":"chat.completion","choices":[{"message":{"content":"` + largeContent + `"}}]}`
	defer upstream.Close()

	proxy := newTestProxy(t, upstream.URL())
	defer proxy.Close()

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}],"stream":false}`
	resp, err := http.Post(proxy.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(responseBody), largeContent) {
		t.Error("large response body not fully proxied")
	}
}

// ── Authorization forwarding test ──────────────────────────────────

func TestProxy_ForwardsAuthorization(t *testing.T) {
	var capturedAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		Port:           "0",
		UpstreamURL:    upstream.URL,
		UpstreamAPIKey: "sk-my-secret-key",
	}
	handler := NewHandler(cfg, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	r := chi.NewRouter()
	r.Post("/v1/chat/completions", handler.ChatCompletions)
	proxy := httptest.NewServer(r)
	defer proxy.Close()

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}]}`
	resp, err := http.Post(proxy.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if capturedAuth != "Bearer sk-my-secret-key" {
		t.Errorf("Authorization = %q, want %q", capturedAuth, "Bearer sk-my-secret-key")
	}
}
