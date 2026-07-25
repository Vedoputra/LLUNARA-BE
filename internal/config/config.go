// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds all runtime configuration for the API.
type Config struct {
	Port              string
	Env               string
	DatabaseURL       string
	SupabaseURL       string
	SupabaseJWKSURL   string
	SupabaseSecretKey string
	AllowedOrigins    []string
}

// Load reads configuration from environment variables. It returns an error
// if any essential variable is missing — callers must treat this as fatal.
func Load() (*Config, error) {
	cfg := &Config{
		Port:              getEnvDefault("PORT", "8080"),
		Env:               getEnvDefault("ENV", "development"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		SupabaseURL:       os.Getenv("SUPABASE_URL"),
		SupabaseJWKSURL:   os.Getenv("SUPABASE_JWKS_URL"),
		SupabaseSecretKey: os.Getenv("SUPABASE_SECRET_KEY"),
		AllowedOrigins:    splitAndTrim(os.Getenv("ALLOWED_ORIGINS")),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.SupabaseURL == "" {
		missing = append(missing, "SUPABASE_URL")
	}
	if cfg.SupabaseJWKSURL == "" {
		missing = append(missing, "SUPABASE_JWKS_URL")
	}
	if cfg.SupabaseSecretKey == "" {
		missing = append(missing, "SUPABASE_SECRET_KEY")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("config: missing required environment variable(s): %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitAndTrim(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
