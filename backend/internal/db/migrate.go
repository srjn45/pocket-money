package db

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations runs all pending migrations
func RunMigrations(databaseURL string) error {
	migrationsPath, err := getMigrationsPath()
	if err != nil {
		return fmt.Errorf("failed to get migrations path: %w", err)
	}

	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// RunMigrationsDown rolls back all migrations
func RunMigrationsDown(databaseURL string) error {
	migrationsPath, err := getMigrationsPath()
	if err != nil {
		return fmt.Errorf("failed to get migrations path: %w", err)
	}

	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to roll back migrations: %w", err)
	}

	return nil
}

// getMigrationsPath returns the absolute path to the migrations directory
func getMigrationsPath() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get current file path")
	}

	// Go from internal/db/migrate.go to migrations/
	dir := filepath.Dir(filename)
	migrationsPath := filepath.Join(dir, "..", "..", "migrations")

	return filepath.Abs(migrationsPath)
}
