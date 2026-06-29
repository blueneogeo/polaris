package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port           string
	UpstreamURL    string
	UpstreamAPIKey string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:           envOrDefault("POLARIS_PORT", "8777"),
		UpstreamURL:    envOrDefault("POLARIS_UPSTREAM_URL", ""),
		UpstreamAPIKey: envOrDefault("POLARIS_UPSTREAM_API_KEY", ""),
	}

	var missing []string
	if cfg.UpstreamURL == "" {
		missing = append(missing, "POLARIS_UPSTREAM_URL")
	}
	if cfg.UpstreamAPIKey == "" {
		missing = append(missing, "POLARIS_UPSTREAM_API_KEY")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
