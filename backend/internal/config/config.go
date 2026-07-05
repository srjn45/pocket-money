package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the application
type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	CORSOrigins string
	// AppBaseURL is the web app origin (e.g. "http://192.168.1.5:8081").
	// When set, invite_url is built from this instead of c.Request.Host so the
	// link points at the Expo web server, not the API server.  Optional: if
	// empty, the request-host fallback is used (works for same-origin deploys).
	AppBaseURL string
}

// Load loads configuration from environment variables
// Returns an error if required variables are missing
func Load() (*Config, error) {
	cfg := &Config{
		Port:        getEnvOrDefault("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		CORSOrigins: getEnvOrDefault("CORS_ORIGINS", "*"),
		AppBaseURL:  os.Getenv("APP_BASE_URL"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// MustLoad loads configuration and panics if required variables are missing
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		panic(err)
	}
	return cfg
}

// validate checks that all required configuration is present
func (c *Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
