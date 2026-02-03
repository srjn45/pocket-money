package testutil

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DefaultTestDBHost     = "localhost"
	DefaultTestDBPort     = "5433"
	DefaultTestDBUser     = "test"
	DefaultTestDBPassword = "test"
	DefaultTestDBName     = "pocket_money_test"
)

// GetTestDatabaseURL returns the database URL for testing
func GetTestDatabaseURL() string {
	host := getEnvOrDefault("TEST_DB_HOST", DefaultTestDBHost)
	port := getEnvOrDefault("TEST_DB_PORT", DefaultTestDBPort)
	user := getEnvOrDefault("TEST_DB_USER", DefaultTestDBUser)
	password := getEnvOrDefault("TEST_DB_PASSWORD", DefaultTestDBPassword)
	dbName := getEnvOrDefault("TEST_DB_NAME", DefaultTestDBName)

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, password, host, port, dbName)
}

// NewTestPool creates a new database connection pool for testing
func NewTestPool() (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbURL := GetTestDatabaseURL()
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// CleanupTestDB cleans up all data in the test database (truncates tables)
func CleanupTestDB(pool *pgxpool.Pool) error {
	ctx := context.Background()

	// Truncate all tables in reverse order of dependencies (preserves schema)
	tables := []string{
		"invite_tokens",
		"settlements",
		"ledger_entries",
		"chores",
		"group_members",
		"groups",
		"users",
	}

	for _, table := range tables {
		_, err := pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			// Table might not exist yet, that's OK
			continue
		}
	}

	return nil
}

// ResetTestDB drops all tables and types in the test database (full reset)
func ResetTestDB(pool *pgxpool.Pool) error {
	ctx := context.Background()

	// Drop all tables in reverse order of dependencies
	tables := []string{
		"schema_migrations",
		"invite_tokens",
		"settlements",
		"ledger_entries",
		"chores",
		"group_members",
		"groups",
		"users",
	}

	for _, table := range tables {
		_, err := pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table))
		if err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	// Drop custom types
	types := []string{"ledger_status", "member_role"}
	for _, t := range types {
		_, err := pool.Exec(ctx, fmt.Sprintf("DROP TYPE IF EXISTS %s CASCADE", t))
		if err != nil {
			return fmt.Errorf("failed to drop type %s: %w", t, err)
		}
	}

	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
