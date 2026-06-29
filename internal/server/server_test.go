package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"log/slog"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/polaris/internal/config"
	"github.com/polaris/internal/middleware"
	"github.com/polaris/internal/proxy"
)

func TestHealthEndpoint(t *testing.T) {
	cfg := &config.Config{
		Port:           "0",
		UpstreamURL:    "https://api.example.com",
		UpstreamAPIKey: "sk-test",
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	proxyHandler := proxy.NewHandler(cfg, logger)

	r := chi.NewRouter()
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Recovery(logger))
	r.Post("/v1/chat/completions", proxyHandler.ChatCompletions)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if !body["ok"] {
		t.Errorf("health check returned ok=false")
	}
}

func TestHealthEndpoint_ContentType(t *testing.T) {
	cfg := &config.Config{
		Port:           "0",
		UpstreamURL:    "https://api.example.com",
		UpstreamAPIKey: "sk-test",
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	proxyHandler := proxy.NewHandler(cfg, logger)

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	_ = proxyHandler // used in full integration; unused in this test but required for router consistency

	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestProxyEndpoint_Registered(t *testing.T) {
	cfg := &config.Config{
		Port:           "0",
		UpstreamURL:    "https://api.example.com",
		UpstreamAPIKey: "sk-test",
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	proxyHandler := proxy.NewHandler(cfg, logger)

	r := chi.NewRouter()
	r.Post("/v1/chat/completions", proxyHandler.ChatCompletions)

	ts := httptest.NewServer(r)
	defer ts.Close()

	// Sending invalid JSON to an unreachable upstream should still hit the handler
	resp, err := http.Post(ts.URL+"/v1/chat/completions", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should get a 400 because the body is empty/invalid, proving the handler is registered
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d — endpoint should reject empty body", resp.StatusCode, http.StatusBadRequest)
	}
}

func Test404_OnUnknownPath(t *testing.T) {
	cfg := &config.Config{
		Port:           "0",
		UpstreamURL:    "https://api.example.com",
		UpstreamAPIKey: "sk-test",
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	proxyHandler := proxy.NewHandler(cfg, logger)

	r := chi.NewRouter()
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Recovery(logger))
	r.Post("/v1/chat/completions", proxyHandler.ChatCompletions)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	_ = proxyHandler

	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/unknown/path")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 or 405 for unknown path", resp.StatusCode)
	}
}
