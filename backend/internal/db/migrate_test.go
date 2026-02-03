//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/testutil"
)

func TestMigrations_UpAndDown(t *testing.T) {
	dbURL := testutil.GetTestDatabaseURL()

	// First, clean up any existing tables
	pool, err := testutil.NewTestPool()
	if err != nil {
		t.Skipf("Skipping test: could not connect to test database: %v", err)
	}
	defer pool.Close()

	err = testutil.CleanupTestDB(pool)
	require.NoError(t, err, "Failed to clean up test database")

	// Run migrations up
	err = db.RunMigrations(dbURL)
	require.NoError(t, err, "Failed to run migrations up")

	// Verify tables exist
	ctx := context.Background()

	tables := []string{
		"users",
		"groups",
		"group_members",
		"chores",
		"ledger_entries",
		"settlements",
		"invite_tokens",
	}

	for _, table := range tables {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = $1
			)
		`, table).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "Table %s should exist after migration up", table)
	}

	// Verify enum types exist
	types := []string{"member_role", "ledger_status"}
	for _, typeName := range types {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_type WHERE typname = $1
			)
		`, typeName).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "Type %s should exist after migration up", typeName)
	}

	// Run migrations down
	err = db.RunMigrationsDown(dbURL)
	require.NoError(t, err, "Failed to run migrations down")

	// Verify tables no longer exist
	for _, table := range tables {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = $1
			)
		`, table).Scan(&exists)
		require.NoError(t, err)
		assert.False(t, exists, "Table %s should not exist after migration down", table)
	}
}

func TestMigrations_Idempotent(t *testing.T) {
	dbURL := testutil.GetTestDatabaseURL()

	pool, err := testutil.NewTestPool()
	if err != nil {
		t.Skipf("Skipping test: could not connect to test database: %v", err)
	}
	defer pool.Close()

	// Clean up first
	err = testutil.CleanupTestDB(pool)
	require.NoError(t, err)

	// Run migrations twice - should not error
	err = db.RunMigrations(dbURL)
	require.NoError(t, err)

	err = db.RunMigrations(dbURL)
	require.NoError(t, err, "Running migrations twice should not error")

	// Clean up
	err = db.RunMigrationsDown(dbURL)
	require.NoError(t, err)
}
