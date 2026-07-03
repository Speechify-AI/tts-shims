// Package config loads runtime configuration from the environment, shared by
// every provider binary in the monorepo.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config is the fully-resolved runtime configuration for a shim binary.
type Config struct {
	// Addr is the TCP address the HTTP server listens on (e.g. ":8080").
	Addr string

	// UpstreamBaseURL is the Speechify API base, without a trailing slash.
	UpstreamBaseURL string

	// APIKey is the Speechify key injected upstream. Empty enables pass-through
	// mode for providers that support forwarding a caller credential.
	APIKey string

	// SpeechifyVersion, when set, pins the upstream API contract via the
	// Speechify-Version header.
	SpeechifyVersion string

	// DefaultModel is the Speechify model used when a request does not select one.
	DefaultModel string

	// RequestTimeout bounds a single upstream synthesis call end to end.
	RequestTimeout time.Duration

	// ReadHeaderTimeout guards against slow-loris style header stalls.
	ReadHeaderTimeout time.Duration

	// ShutdownTimeout bounds graceful shutdown draining.
	ShutdownTimeout time.Duration
}

// Load reads configuration from the environment, applying defaults. It errors
// only for values that are present but malformed.
func Load() (Config, error) {
	c := Config{
		Addr:              getenv("SHIM_ADDR", ":8080"),
		UpstreamBaseURL:   strings.TrimRight(getenv("SPEECHIFY_BASE_URL", "https://api.speechify.ai"), "/"),
		APIKey:            os.Getenv("SPEECHIFY_API_KEY"),
		SpeechifyVersion:  os.Getenv("SPEECHIFY_VERSION"),
		DefaultModel:      getenv("SHIM_DEFAULT_MODEL", "simba-english"),
		RequestTimeout:    30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		ShutdownTimeout:   15 * time.Second,
	}

	if v := os.Getenv("SHIM_REQUEST_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("SHIM_REQUEST_TIMEOUT: %w", err)
		}
		c.RequestTimeout = d
	}
	if v := os.Getenv("SHIM_SHUTDOWN_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("SHIM_SHUTDOWN_TIMEOUT: %w", err)
		}
		c.ShutdownTimeout = d
	}
	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
