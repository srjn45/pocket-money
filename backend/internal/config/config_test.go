package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_WithAllEnvVars(t *testing.T) {
	// Set all env vars
	os.Setenv("PORT", "9090")
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test")
	os.Setenv("JWT_SECRET", "supersecret")
	os.Setenv("CORS_ORIGINS", "http://localhost:3000")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("CORS_ORIGINS")
	}()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "postgres://test:test@localhost:5432/test", cfg.DatabaseURL)
	assert.Equal(t, "supersecret", cfg.JWTSecret)
	assert.Equal(t, "http://localhost:3000", cfg.CORSOrigins)
}

func TestLoad_DefaultPort(t *testing.T) {
	os.Unsetenv("PORT")
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test")
	os.Setenv("JWT_SECRET", "supersecret")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET")
	}()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "8080", cfg.Port)
}

func TestLoad_DefaultCORSOrigins(t *testing.T) {
	os.Unsetenv("CORS_ORIGINS")
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test")
	os.Setenv("JWT_SECRET", "supersecret")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET")
	}()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "*", cfg.CORSOrigins)
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	os.Setenv("JWT_SECRET", "supersecret")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()

	assert.Nil(t, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL is required")
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test")
	os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("DATABASE_URL")

	cfg, err := Load()

	assert.Nil(t, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET is required")
}

func TestMustLoad_Panics(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("JWT_SECRET")

	assert.Panics(t, func() {
		MustLoad()
	})
}

func TestMustLoad_Success(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test")
	os.Setenv("JWT_SECRET", "supersecret")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET")
	}()

	assert.NotPanics(t, func() {
		cfg := MustLoad()
		assert.NotNil(t, cfg)
	})
}
