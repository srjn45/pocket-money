//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
	"github.com/srjn45/pocket-money/backend/testutil"
)

// TestGetBalanceForGroup_MinorUnits verifies the balance query returns integer
// minor units and that the ::bigint casts on the aggregated SUM(...) columns
// (WP-1.1) let pgx scan the result straight into int64. Without the cast,
// Postgres returns SUM(bigint) as numeric and the Scan into int64 fails at
// runtime — this is the single most likely break of the money-to-minor-units
// change (spec WP-1.1 §2/§8), so it is asserted here integer-exact.
func TestGetBalanceForGroup_MinorUnits(t *testing.T) {
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

	// Head + member users.
	head, err := userRepo.Create(ctx, "head@example.com", "hash", "Head", nil, nil)
	require.NoError(t, err)
	member, err := userRepo.Create(ctx, "member@example.com", "hash", "Member", nil, nil)
	require.NoError(t, err)

	group, err := groupRepo.Create(ctx, "Test Family", head.ID)
	require.NoError(t, err)
	_, err = groupRepo.AddMember(ctx, group.ID, member.ID, models.RoleMember)
	require.NoError(t, err)

	// Regular chore (earnings) and a system chore (settlements). Amounts are
	// minor units; the system chore itself carries 0, settlement entries carry
	// the custom amount.
	regularChore, err := choreRepo.CreateWithSystem(ctx, group.ID, "Dishes", nil, 1250, false)
	require.NoError(t, err)
	systemChore, err := choreRepo.CreateWithSystem(ctx, group.ID, "Settlement", nil, 0, true)
	require.NoError(t, err)

	// Approved earnings: 1250 + 750 = 2000 minor units.
	_, err = ledgerRepo.Create(ctx, group.ID, member.ID, regularChore.ID, head.ID, 1250, models.StatusApproved, &head.ID)
	require.NoError(t, err)
	_, err = ledgerRepo.Create(ctx, group.ID, member.ID, regularChore.ID, head.ID, 750, models.StatusApproved, &head.ID)
	require.NoError(t, err)

	// Approved settlement: 575 minor units.
	_, err = ledgerRepo.Create(ctx, group.ID, member.ID, systemChore.ID, head.ID, 575, models.StatusApproved, &head.ID)
	require.NoError(t, err)

	// Pending and rejected entries must NOT count toward the balance.
	_, err = ledgerRepo.Create(ctx, group.ID, member.ID, regularChore.ID, head.ID, 9999, models.StatusPendingApproval, nil)
	require.NoError(t, err)
	_, err = ledgerRepo.Create(ctx, group.ID, member.ID, regularChore.ID, head.ID, 8888, models.StatusRejected, nil)
	require.NoError(t, err)

	balances, err := ledgerRepo.GetBalanceForGroup(ctx, group.ID)
	require.NoError(t, err, "GetBalanceForGroup must not fail (numeric->int64 scan requires the ::bigint cast)")

	var memberBalance *models.Balance
	for _, b := range balances {
		if b.UserID == member.ID {
			memberBalance = b
			break
		}
	}
	require.NotNil(t, memberBalance, "member should appear in balances")

	// earned(2000) - settled(575) = 1425, integer-exact in minor units.
	assert.Equal(t, int64(1425), memberBalance.Balance)
}
