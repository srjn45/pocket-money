//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
	"github.com/srjn45/pocket-money/backend/testutil"
)

// TestInsertAllowancePosting_UniqueConflict verifies that InsertAllowancePosting
// returns (inserted=true) on first call and (inserted=false) on duplicate, with
// exactly one row in the ledger — the ON CONFLICT DO NOTHING guarantee (§2.3, §3.4).
func TestInsertAllowancePosting_UniqueConflict(t *testing.T) {
	pool, err := testutil.NewTestPool()
	if err != nil {
		t.Skipf("Skipping test: could not connect to test database: %v", err)
	}
	defer pool.Close()

	require.NoError(t, testutil.ResetTestDB(pool))
	require.NoError(t, db.RunMigrations(testutil.GetTestDatabaseURL()))
	defer testutil.CleanupTestDB(pool)

	ctx := context.Background()

	userRepo := db.NewUserRepo(pool)
	groupRepo := db.NewGroupRepo(pool)
	ledgerRepo := db.NewLedgerRepo(pool)

	head, err := userRepo.Create(ctx, "head-posting@example.com", "hash", "Head", nil, nil)
	require.NoError(t, err)
	member, err := userRepo.Create(ctx, "member-posting@example.com", "hash", "Member", nil, nil)
	require.NoError(t, err)

	group, err := groupRepo.Create(ctx, "Posting Family", head.ID, models.CurrencyINR)
	require.NoError(t, err)
	_, err = groupRepo.AddMember(ctx, group.ID, head.ID, models.RoleHead)
	require.NoError(t, err)
	_, err = groupRepo.AddMember(ctx, group.ID, member.ID, models.RoleMember)
	require.NoError(t, err)

	period := "2026-05"

	// First insert → inserted=true
	inserted, err := ledgerRepo.InsertAllowancePosting(ctx, pool, group.ID, member.ID, 1000, period, head.ID)
	require.NoError(t, err)
	assert.True(t, inserted, "first insert must return inserted=true")

	// Second insert (same group/user/period) → inserted=false (ON CONFLICT DO NOTHING)
	inserted, err = ledgerRepo.InsertAllowancePosting(ctx, pool, group.ID, member.ID, 1000, period, head.ID)
	require.NoError(t, err)
	assert.False(t, inserted, "duplicate must return inserted=false, no error")

	// Exactly one row in the ledger for this (group, user, allowance, period).
	posted, err := ledgerRepo.PostedAllowancePeriods(ctx, group.ID)
	require.NoError(t, err)
	require.NotNil(t, posted[member.ID])
	assert.True(t, posted[member.ID][period])
	assert.Len(t, posted[member.ID], 1, "exactly one period posted")
}

// TestAllowanceLedgerFields verifies that machine-posted allowance entries have
// the correct field values per §3.6.
func TestAllowanceLedgerFields(t *testing.T) {
	pool, err := testutil.NewTestPool()
	if err != nil {
		t.Skipf("Skipping test: could not connect to test database: %v", err)
	}
	defer pool.Close()

	require.NoError(t, testutil.ResetTestDB(pool))
	require.NoError(t, db.RunMigrations(testutil.GetTestDatabaseURL()))
	defer testutil.CleanupTestDB(pool)

	ctx := context.Background()

	userRepo := db.NewUserRepo(pool)
	groupRepo := db.NewGroupRepo(pool)
	ledgerRepo := db.NewLedgerRepo(pool)

	head, err := userRepo.Create(ctx, "head-fields@example.com", "hash", "Head", nil, nil)
	require.NoError(t, err)
	member, err := userRepo.Create(ctx, "member-fields@example.com", "hash", "Member", nil, nil)
	require.NoError(t, err)

	group, err := groupRepo.Create(ctx, "Fields Family", head.ID, models.CurrencyINR)
	require.NoError(t, err)
	_, err = groupRepo.AddMember(ctx, group.ID, head.ID, models.RoleHead)
	require.NoError(t, err)
	_, err = groupRepo.AddMember(ctx, group.ID, member.ID, models.RoleMember)
	require.NoError(t, err)

	period := "2026-06"
	inserted, err := ledgerRepo.InsertAllowancePosting(ctx, pool, group.ID, member.ID, 750, period, head.ID)
	require.NoError(t, err)
	require.True(t, inserted)

	// Fetch the entry and verify machine-post fields.
	entries, err := ledgerRepo.ListForGroupWithUser(ctx, group.ID, nil, &member.ID, nil, &period)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	e := entries[0]
	assert.Equal(t, models.EntryTypeAllowance, e.EntryType)
	assert.Equal(t, models.DirectionCredit, e.Direction)
	assert.Equal(t, models.StatusApproved, e.Status)
	assert.Equal(t, int64(750), e.Amount)
	assert.Equal(t, head.ID, e.CreatedByUserID)
	assert.Nil(t, e.DecidedBy, "machine post: decided_by must be NULL")
	assert.Nil(t, e.DecidedAt, "machine post: decided_at must be NULL")
	assert.Nil(t, e.ChoreID, "machine post: chore_id must be NULL")
	assert.Nil(t, e.LoanID, "machine post: loan_id must be NULL")
	require.NotNil(t, e.Period)
	assert.Equal(t, period, *e.Period)
	_ = uuid.Nil // imported for UUID type checking
}
