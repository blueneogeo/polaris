package config

import (
	"os"
	"testing"
)

func TestLoad_Valid(t *testing.T) {
	t.Setenv("UPSTREAM_URL", "https://api.example.com")
	t.Setenv("UPSTREAM_API_KEY", "sk-test-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != "8777" {
		t.Errorf("Port = %q, want %q", cfg.Port, "8777")
	}
	if cfg.UpstreamURL != "https://api.example.com" {
		t.Errorf("UpstreamURL = %q, want %q", cfg.UpstreamURL, "https://api.example.com")
	}
	if cfg.UpstreamAPIKey != "sk-test-key" {
		t.Errorf("UpstreamAPIKey = %q, want %q", cfg.UpstreamAPIKey, "sk-test-key")
	}
}

func TestLoad_CustomPort(t *testing.T) {
	t.Setenv("UPSTREAM_URL", "https://api.example.com")
	t.Setenv("UPSTREAM_API_KEY", "sk-key")
	t.Setenv("PORT", "9999")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != "9999" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9999")
	}
}

func TestLoad_MissingUpstreamURL(t *testing.T) {
	t.Setenv("UPSTREAM_URL", "")
	t.Setenv("UPSTREAM_API_KEY", "sk-key")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing UPSTREAM_URL")
	}
}

func TestLoad_MissingUpstreamAPIKey(t *testing.T) {
	t.Setenv("UPSTREAM_URL", "https://api.example.com")
	t.Setenv("UPSTREAM_API_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing UPSTREAM_API_KEY")
	}
}

func TestLoad_MultipleMissing(t *testing.T) {
	t.Setenv("UPSTREAM_URL", "")
	t.Setenv("UPSTREAM_API_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required env vars")
	}
}

func TestEnvOrDefault_UsesFallback(t *testing.T) {
	_ = os.Unsetenv("NONEXISTENT_VAR")
	if v := envOrDefault("NONEXISTENT_VAR", "fallback"); v != "fallback" {
		t.Errorf("envOrDefault = %q, want %q", v, "fallback")
	}
}

func TestEnvOrDefault_UsesEnv(t *testing.T) {
	t.Setenv("TEST_VAR", "custom")
	if v := envOrDefault("TEST_VAR", "fallback"); v != "custom" {
		t.Errorf("envOrDefault = %q, want %q", v, "custom")
	}
}

func TestEnvOrDefault_EmptyEnv(t *testing.T) {
	_ = os.Unsetenv("TEST_EMPTY")
	if v := envOrDefault("TEST_EMPTY", "fallback"); v != "fallback" {
		t.Errorf("envOrDefault = %q, want %q", v, "fallback")
	}
}
