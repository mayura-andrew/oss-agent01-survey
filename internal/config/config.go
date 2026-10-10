// Package config loads the survey service settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Defaults for optional settings.
const (
	DefaultPort       = "8080"
	DefaultTable      = "survey_responses"
	DefaultRateLimit  = 5
	DefaultRateWindow = 10 * time.Minute
	DefaultLogFormat  = "text"
)

// Config holds every setting the service needs.
type Config struct {
	// Port is the TCP port to listen on (PORT).
	Port string
	// SupabaseURL is the project URL without a trailing slash (SUPABASE_URL).
	SupabaseURL string
	// SupabaseKey is the secret or service_role key (SUPABASE_SECRET_KEY).
	SupabaseKey string
	// Table is the table responses are inserted into (SUPABASE_TABLE).
	Table string
	// AllowedOrigin enables CORS for one origin when the page is hosted
	// elsewhere (ALLOWED_ORIGIN). Empty disables CORS.
	AllowedOrigin string
	// TrustProxy reads the client IP from X-Forwarded-For (TRUST_PROXY=true).
	TrustProxy bool
	// RateLimit is the number of submissions allowed per IP per RateWindow (RATE_LIMIT).
	RateLimit int
	// RateWindow is the rate-limit window. It is not configurable.
	RateWindow time.Duration
	// LogFormat is "text" or "json" (LOG_FORMAT).
	LogFormat string
}

// Load reads the configuration through getenv (usually os.Getenv). It reports
// every problem at once, joined into a single error.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		Port:          envOr(getenv, "PORT", DefaultPort),
		SupabaseURL:   strings.TrimRight(getenv("SUPABASE_URL"), "/"),
		SupabaseKey:   getenv("SUPABASE_SECRET_KEY"),
		Table:         envOr(getenv, "SUPABASE_TABLE", DefaultTable),
		AllowedOrigin: strings.TrimRight(getenv("ALLOWED_ORIGIN"), "/"),
		TrustProxy:    getenv("TRUST_PROXY") == "true",
		RateLimit:     DefaultRateLimit,
		RateWindow:    DefaultRateWindow,
		LogFormat:     envOr(getenv, "LOG_FORMAT", DefaultLogFormat),
	}

	var errs []error
	if v := getenv("RATE_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			errs = append(errs, errors.New("RATE_LIMIT must be a positive number"))
		} else {
			c.RateLimit = n
		}
	}
	if c.SupabaseURL == "" || c.SupabaseKey == "" {
		errs = append(errs, errors.New("SUPABASE_URL and SUPABASE_SECRET_KEY must be set"))
	}
	if c.SupabaseURL != "" {
		if u, err := url.Parse(c.SupabaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			errs = append(errs, fmt.Errorf("SUPABASE_URL must be an absolute http(s) URL, got %q", c.SupabaseURL))
		}
	}
	if c.LogFormat != "text" && c.LogFormat != "json" {
		errs = append(errs, fmt.Errorf("LOG_FORMAT must be text or json, got %q", c.LogFormat))
	}
	return c, errors.Join(errs...)
}

func envOr(getenv func(string) string, key, def string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return def
}
