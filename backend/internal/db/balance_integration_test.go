//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
	"github.com/srjn45/pocket-money/backend/testutil"
)

// TestGetBalanceForGroup_DirectionBased verifies that balance is computed as
// Σ approved credits − Σ approved debits using the direction column, with no
// dependency on the chores JOIN. The critical regression guard: settlement entries
// have NULL chore_id — the old INNER JOIN chores query silently dropped them and
// miscomputed the balance; the new direction-based query must count them.
//
// Also asserts the ::bigint casts (WP-1.1 §2) let pgx scan aggregates into int64.
func TestGetBalanceForGroup_DirectionBased(t *testing.T) {
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
	choreRepo := db.NewChoreRepo(pool)
	ledgerRepo := db.NewLedgerRepo(pool)

	head, err := userRepo.Create(ctx, "head@example.com", "hash", "Head", nil, nil)
	require.NoError(t, err)
	member, err := userRepo.Create(ctx, "member@example.com", "hash", "Member", nil, nil)
	require.NoError(t, err)

	group, err := groupRepo.Create(ctx, "Test Family", head.ID, models.CurrencyINR)
	require.NoError(t, err)
	_, err = groupRepo.AddMember(ctx, group.ID, member.ID, models.RoleMember)
	require.NoError(t, err)

	// Regular chore for credit entries
	regularChore, err := choreRepo.CreateWithSystem(ctx, group.ID, "Dishes", nil, 1250, false)
	require.NoError(t, err)
	choreID := regularChore.ID

	now := time.Now()

	// Approved chore credits: 1250 + 750 = 2000 minor units
	_, err = ledgerRepo.Create(ctx, group.ID, member.ID, &choreID, head.ID, 1250,
		models.EntryTypeChore, models.DirectionCredit, models.StatusApproved, nil, &head.ID, &now)
	require.NoError(t, err)
	_, err = ledgerRepo.Create(ctx, group.ID, member.ID, &choreID, head.ID, 750,
		models.EntryTypeChore, models.DirectionCredit, models.StatusApproved, nil, &head.ID, &now)
	require.NoError(t, err)

	// Approved settlement debit with NULL chore_id: 575 minor units.
	// This is the regression guard: the old chore-JOIN query would silently exclude
	// this row because chore_id IS NULL; the new direction-based query must count it.
	_, err = ledgerRepo.Create(ctx, group.ID, member.ID, nil, head.ID, 575,
		models.EntryTypeSettlement, models.DirectionDebit, models.StatusApproved, nil, &head.ID, &now)
	require.NoError(t, err)

	// Pending and rejected entries must NOT count toward the balance.
	_, err = ledgerRepo.Create(ctx, group.ID, member.ID, &choreID, head.ID, 9999,
		models.EntryTypeChore, models.DirectionCredit, models.StatusPendingApproval, nil, nil, nil)
	require.NoError(t, err)
	_, err = ledgerRepo.Create(ctx, group.ID, member.ID, &choreID, head.ID, 8888,
		models.EntryTypeChore, models.DirectionCredit, models.StatusRejected, nil, nil, nil)
	require.NoError(t, err)

	balances, err := ledgerRepo.GetBalanceForGroup(ctx, group.ID)
	require.NoError(t, err, "GetBalanceForGroup must not fail (numeric->int64 scan requires ::bigint casts)")

	var memberBalance *models.Balance
	for _, b := range balances {
		if b.UserID == member.ID {
			memberBalance = b
			break
		}
	}
	require.NotNil(t, memberBalance, "member should appear in balances")

	// credits(2000) - debits(575) = 1425, integer-exact in minor units.
	assert.Equal(t, int64(1425), memberBalance.Balance)
}

// TestGetBalanceForGroup_AdjustmentBothDirections verifies that adjustment entries
// in both directions are counted correctly, and that negative balance is allowed.
func TestGetBalanceForGroup_AdjustmentBothDirections(t *testing.T) {
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

	head, err := userRepo.Create(ctx, "head2@example.com", "hash", "Head", nil, nil)
	require.NoError(t, err)
	member, err := userRepo.Create(ctx, "member2@example.com", "hash", "Member", nil, nil)
	require.NoError(t, err)

	group, err := groupRepo.Create(ctx, "Family2", head.ID, models.CurrencyINR)
	require.NoError(t, err)
	_, err = groupRepo.AddMember(ctx, group.ID, member.ID, models.RoleMember)
	require.NoError(t, err)

	now := time.Now()

	// Credit adjustment: +500
	_, err = ledgerRepo.Create(ctx, group.ID, member.ID, nil, head.ID, 500,
		models.EntryTypeAdjustment, models.DirectionCredit, models.StatusApproved, nil, &head.ID, &now)
	require.NoError(t, err)
	// Debit adjustment: -800 (resulting balance is negative)
	_, err = ledgerRepo.Create(ctx, group.ID, member.ID, nil, head.ID, 800,
		models.EntryTypeAdjustment, models.DirectionDebit, models.StatusApproved, nil, &head.ID, &now)
	require.NoError(t, err)

	balances, err := ledgerRepo.GetBalanceForGroup(ctx, group.ID)
	require.NoError(t, err)

	var memberBalance *models.Balance
	for _, b := range balances {
		if b.UserID == member.ID {
			memberBalance = b
			break
		}
	}
	require.NotNil(t, memberBalance)
	// credits(500) - debits(800) = -300
	assert.Equal(t, int64(-300), memberBalance.Balance)
}
