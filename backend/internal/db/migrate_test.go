//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/migrations"
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

	err = testutil.ResetTestDB(pool)
	require.NoError(t, err, "Failed to clean up test database")

	// Run migrations up
	err = db.RunMigrations(dbURL)
	require.NoError(t, err, "Failed to run migrations up")

	ctx := context.Background()

	// Verify tables that must exist after full migration up (settlements dropped by 012)
	tables := []string{
		"users",
		"groups",
		"group_members",
		"chores",
		"ledger_entries",
		"invite_tokens",
		"allowances",
		"loans",
		"notifications", // V3-2.1
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

	// settlements must NOT exist after 012 drops it
	var settlementsExists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'settlements'
		)
	`).Scan(&settlementsExists)
	require.NoError(t, err)
	assert.False(t, settlementsExists, "settlements table should not exist after migration 012")

	// Verify enum types exist (including v2 types and loan_status from 011)
	types := []string{"member_role", "ledger_status", "ledger_entry_type", "ledger_direction", "loan_status"}
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

	// V3-2.1: users gained shadow-user columns and password_hash became nullable.
	var pwNullable string
	err = pool.QueryRow(ctx, `
		SELECT is_nullable FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'password_hash'
	`).Scan(&pwNullable)
	require.NoError(t, err)
	assert.Equal(t, "YES", pwNullable, "users.password_hash should be nullable after 014")

	for _, col := range []string{"status", "claimed_at"} {
		var exists bool
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = 'users' AND column_name = $1
			)
		`, col).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "users.%s should exist after migration 014", col)
	}

	// Run migrations down
	err = db.RunMigrationsDown(dbURL)
	require.NoError(t, err, "Failed to run migrations down")

	// Verify tables no longer exist (down rolled everything back)
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
	err = testutil.ResetTestDB(pool)
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

// TestMigration015_RenamesHeadToAdmin drives golang-migrate to v14 (schema still
// has role 'head' and groups.head_user_id), inserts a raw 'head' membership, then
// applies 015 and asserts the row was rewritten to 'admin', the column renamed to
// admin_user_id, and the enum ends as exactly {admin, member}.
func TestMigration015_RenamesHeadToAdmin(t *testing.T) {
	dbURL := testutil.GetTestDatabaseURL()

	pool, err := testutil.NewTestPool()
	if err != nil {
		t.Skipf("Skipping test: could not connect to test database: %v", err)
	}
	defer pool.Close()

	require.NoError(t, testutil.ResetTestDB(pool))

	// Build the migrate instance from the embedded migrations FS (iofs), matching
	// the production source. No dependency on the on-disk source/file driver.
	src, err := iofs.New(migrations.FS, ".")
	require.NoError(t, err)
	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	require.NoError(t, err)
	defer m.Close()

	// Migrate to v14 — before the rename. role 'head' and groups.head_user_id valid.
	require.NoError(t, m.Migrate(14))

	ctx := context.Background()
	userID := uuid.New()
	groupID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, name) VALUES ($1, $2, $3, $4)`,
		userID, "mig015@example.com", "hash", "Mig")
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO groups (id, name, head_user_id, currency) VALUES ($1, $2, $3, $4)`,
		groupID, "MigGroup", userID, "INR")
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, 'head')`,
		groupID, userID)
	require.NoError(t, err)

	// Apply 015.
	require.NoError(t, m.Migrate(15))

	// The pre-existing row was rewritten from 'head' to 'admin' in place.
	var role string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT role::text FROM group_members WHERE group_id = $1 AND user_id = $2`,
		groupID, userID).Scan(&role))
	assert.Equal(t, "admin", role)

	// The owner column was renamed head_user_id → admin_user_id (value preserved).
	var adminUserID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT admin_user_id FROM groups WHERE id = $1`, groupID).Scan(&adminUserID))
	assert.Equal(t, userID, adminUserID)

	// The enum ends as exactly {admin, member} — no 'head' label survives.
	var labels []string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT array(SELECT unnest(enum_range(NULL::member_role))::text)`).Scan(&labels))
	assert.ElementsMatch(t, []string{"admin", "member"}, labels)
}
