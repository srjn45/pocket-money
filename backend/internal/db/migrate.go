package db

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/srjn45/pocket-money/backend/migrations"
)

// newMigrate builds a migrate instance whose source is the embedded migrations
// FS (iofs), so the runtime binary is self-contained and needs no on-disk
// migrations/ directory. The postgres DB driver is still selected from the URL
// scheme via the blank import above.
func newMigrate(databaseURL string) (*migrate.Migrate, error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to build iofs source: %w", err)
	}
	return migrate.NewWithSourceInstance("iofs", src, databaseURL)
}

// RunMigrations runs all pending migrations
func RunMigrations(databaseURL string) error {
	m, err := newMigrate(databaseURL)
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
	m, err := newMigrate(databaseURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to roll back migrations: %w", err)
	}

	return nil
}
