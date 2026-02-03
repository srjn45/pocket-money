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
}

// Load loads configuration from environment variables
// Returns an error if required variables are missing
func Load() (*Config, error) {
	cfg := &Config{
		Port:        getEnvOrDefault("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		CORSOrigins: getEnvOrDefault("CORS_ORIGINS", "*"),
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
