//go:build integration

package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/srjn45/pocket-money/backend/internal/auth"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/handlers"
	"github.com/srjn45/pocket-money/backend/internal/models"
	"github.com/srjn45/pocket-money/backend/internal/posting"
	"github.com/srjn45/pocket-money/backend/testutil"
)

// hygieneTestEnv holds wired-up deps for WP-4.7 integration tests.
type hygieneTestEnv struct {
	router        *gin.Engine
	pool          *pgxpool.Pool
	userRepo      *db.UserRepo
	groupRepo     *db.GroupRepo
	ledgerRepo    *db.LedgerRepo
	allowanceRepo *db.AllowanceRepo
	loanRepo      *db.LoanRepo
	choreRepo     *db.ChoreRepo
	postingSvc    *posting.Service
	cleanup       func()
}

func setupHygieneTestEnv(t *testing.T) *hygieneTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	pool, err := testutil.NewTestPool()
	if err != nil {
		t.Skipf("Skipping test: could not connect to test database: %v", err)
	}

	require.NoError(t, testutil.ResetTestDB(pool))
	require.NoError(t, db.RunMigrations(testutil.GetTestDatabaseURL()))

	userRepo := db.NewUserRepo(pool)
	groupRepo := db.NewGroupRepo(pool)
	choreRepo := db.NewChoreRepo(pool)
	ledgerRepo := db.NewLedgerRepo(pool)
	allowanceRepo := db.NewAllowanceRepo(pool)
	loanRepo := db.NewLoanRepo(pool)
	inviteRepo := db.NewInviteRepo(pool)

	postingSvc := posting.NewService(allowanceRepo, ledgerRepo, loanRepo, groupRepo, pool)

	authH := handlers.NewAuthHandler(userRepo, testJWTSecret)
	lh := handlers.NewLedgerHandler(ledgerRepo, groupRepo, nil, postingSvc)
	gh := handlers.NewGroupHandler(groupRepo, inviteRepo, choreRepo, ledgerRepo, loanRepo, allowanceRepo, postingSvc, pool, "")
	loansH := handlers.NewLoanHandler(loanRepo, ledgerRepo, groupRepo, pool)

	router := gin.New()
	authMw := auth.AuthMiddleware(testJWTSecret)
	router.Use(authMw)

	// auth routes (PUT /auth/password needs the middleware)
	router.PUT("/auth/password", authH.ChangePassword)
	router.POST("/auth/login", func(c *gin.Context) {
		// Bypass JWT middleware for login — use a naked handler route
		authH.Login(c)
	})

	// group/member routes
	router.DELETE("/groups/:id/members/:userId", gh.RemoveMember)
	router.GET("/groups/:id/balance", lh.GetBalance)
	router.POST("/groups/:id/loans", loansH.CreateLoan)
	router.POST("/loans/:id/approve", loansH.ApproveLoan)

	return &hygieneTestEnv{
		router:        router,
		pool:          pool,
		userRepo:      userRepo,
		groupRepo:     groupRepo,
		ledgerRepo:    ledgerRepo,
		allowanceRepo: allowanceRepo,
		loanRepo:      loanRepo,
		choreRepo:     choreRepo,
		postingSvc:    postingSvc,
		cleanup: func() {
			testutil.CleanupTestDB(pool)
			pool.Close()
		},
	}
}

// seedHygieneGroup creates head+member with a real bcrypt hash for the head (for password tests).
func (e *hygieneTestEnv) seedHygieneGroup(t *testing.T, suffix string, headPassword string) (head, member *models.User, group *models.Group) {
	t.Helper()
	ctx := t.Context()

	hash, err := bcrypt.GenerateFromPassword([]byte(headPassword), bcrypt.MinCost)
	require.NoError(t, err)

	head, err = e.userRepo.Create(ctx, fmt.Sprintf("head-%s@example.com", suffix), string(hash), "Head"+suffix, nil, nil)
	require.NoError(t, err)
	member, err = e.userRepo.Create(ctx, fmt.Sprintf("member-%s@example.com", suffix), "hash", "Member"+suffix, nil, nil)
	require.NoError(t, err)

	group, err = e.groupRepo.Create(ctx, "Family "+suffix, head.ID)
	require.NoError(t, err)
	_, err = e.groupRepo.AddMember(ctx, group.ID, head.ID, models.RoleHead)
	require.NoError(t, err)
	_, err = e.groupRepo.AddMember(ctx, group.ID, member.ID, models.RoleMember)
	require.NoError(t, err)
	return
}

// ─── Change Password Tests ────────────────────────────────────────────────────

// TestChangePassword_WrongCurrentPassword asserts 403 (NOT 401) for wrong current password.
func TestChangePassword_WrongCurrentPassword(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	head, _, _ := env.seedHygieneGroup(t, "pw1", "correctpass")

	w := doRequest(env.router, http.MethodPut, "/auth/password", map[string]interface{}{
		"current_password": "wrongpass",
		"new_password":     "newpass123",
	}, bearerToken(t, head.ID))

	// Must be 403, NOT 401 — a 401 would trip the FE logout interceptor.
	assert.Equal(t, http.StatusForbidden, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["error"])

	// Stored hash is unchanged — old password still works for auth.
	updated, err := env.userRepo.GetByID(t.Context(), head.ID)
	require.NoError(t, err)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("correctpass")))
}

// TestChangePassword_Success asserts 204, then new password works and old does not.
func TestChangePassword_Success(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	head, _, _ := env.seedHygieneGroup(t, "pw2", "oldpassword")

	w := doRequest(env.router, http.MethodPut, "/auth/password", map[string]interface{}{
		"current_password": "oldpassword",
		"new_password":     "newpassword",
	}, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusNoContent, w.Code)

	// New password verifiable in DB.
	updated, err := env.userRepo.GetByID(t.Context(), head.ID)
	require.NoError(t, err)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("newpassword")))
	assert.Error(t, bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("oldpassword")))
}

// TestChangePassword_TooShort asserts 400 when new_password < 6 chars.
func TestChangePassword_TooShort(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	head, _, _ := env.seedHygieneGroup(t, "pw3", "correctpass")

	w := doRequest(env.router, http.MethodPut, "/auth/password", map[string]interface{}{
		"current_password": "correctpass",
		"new_password":     "abc",
	}, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Hash must be unchanged.
	updated, err := env.userRepo.GetByID(t.Context(), head.ID)
	require.NoError(t, err)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("correctpass")))
}

// TestChangePassword_Unauthenticated asserts 401 for missing token.
func TestChangePassword_Unauthenticated(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	w := doRequest(env.router, http.MethodPut, "/auth/password", map[string]interface{}{
		"current_password": "x",
		"new_password":     "newpass123",
	}, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ─── Remove Member / Leave Group Tests ────────────────────────────────────────

// countLedgerRows returns the total ledger entry count for a user in a group.
func (e *hygieneTestEnv) countLedgerRows(t *testing.T, groupID, userID uuid.UUID) int {
	t.Helper()
	var count int
	err := e.pool.QueryRow(t.Context(),
		`SELECT COUNT(*) FROM ledger_entries WHERE group_id = $1 AND user_id = $2`,
		groupID, userID).Scan(&count)
	require.NoError(t, err)
	return count
}

// countAllowanceRows returns the allowance row count for a user in a group.
func (e *hygieneTestEnv) countAllowanceRows(t *testing.T, groupID, userID uuid.UUID) int {
	t.Helper()
	var count int
	err := e.pool.QueryRow(t.Context(),
		`SELECT COUNT(*) FROM allowances WHERE group_id = $1 AND user_id = $2`,
		groupID, userID).Scan(&count)
	require.NoError(t, err)
	return count
}

// isMember checks whether a user is in group_members.
func (e *hygieneTestEnv) isMember(t *testing.T, groupID, userID uuid.UUID) bool {
	t.Helper()
	var count int
	err := e.pool.QueryRow(t.Context(),
		`SELECT COUNT(*) FROM group_members WHERE group_id = $1 AND user_id = $2`,
		groupID, userID).Scan(&count)
	require.NoError(t, err)
	return count > 0
}

// seedSettledMember adds an approved credit and matching debit to give a zero balance.
func (e *hygieneTestEnv) seedSettledMember(t *testing.T, groupID, memberID, actorID uuid.UUID) {
	t.Helper()
	ctx := t.Context()

	// chore_id is nullable (migration 009) and only chore entries carry one; these
	// synthetic balance-seed entries use non-chore types so no chores row is needed.
	_, err := e.pool.Exec(ctx, `
		INSERT INTO ledger_entries (id, group_id, user_id, amount, status, entry_type, direction, created_by_user_id)
		VALUES ($1, $2, $3, 50000, 'approved', 'adjustment', 'credit', $4)`,
		uuid.New(), groupID, memberID, actorID)
	require.NoError(t, err)

	_, err = e.pool.Exec(ctx, `
		INSERT INTO ledger_entries (id, group_id, user_id, amount, status, entry_type, direction, created_by_user_id)
		VALUES ($1, $2, $3, 50000, 'approved', 'settlement', 'debit', $4)`,
		uuid.New(), groupID, memberID, actorID)
	require.NoError(t, err)
}

// removePath returns the DELETE endpoint path for a member.
func removePath(groupID, userID uuid.UUID) string {
	return fmt.Sprintf("/groups/%s/members/%s", groupID, userID)
}

// TestRemoveMember_HeadRemovesSettledMember asserts 204 and verifies history is kept.
func TestRemoveMember_HeadRemovesSettledMember(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	head, _, group := env.seedHygieneGroup(t, "rm1", "pass")
	groupID := group.ID
	member, err := env.userRepo.Create(t.Context(), "target-rm1@example.com", "hash", "Target", nil, nil)
	require.NoError(t, err)
	_, err = env.groupRepo.AddMember(t.Context(), groupID, member.ID, models.RoleMember)
	require.NoError(t, err)

	// Set up a *future-effective* allowance and seed a settled balance. The allowance
	// row must exist so we can prove it is deleted on removal, but it must not be due:
	// PostDue posts one credit per month from max(effective_from, join-month) through
	// the current month (engine.go), and AddMember stamps joined_at = now(). A future
	// effective_from keeps the row present without posting a credit that would push the
	// balance non-zero (→ 409 instead of the expected 204).
	futureEff := time.Now().AddDate(0, 1, 0).Format("2006-01")
	_, err = env.allowanceRepo.SetAllowance(t.Context(), groupID, member.ID, 10000, futureEff, head.ID)
	require.NoError(t, err)
	env.seedSettledMember(t, groupID, member.ID, head.ID)

	ledgerBefore := env.countLedgerRows(t, groupID, member.ID)
	assert.Greater(t, ledgerBefore, 0)

	w := doRequest(env.router, http.MethodDelete, removePath(groupID, member.ID), nil, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Membership removed.
	assert.False(t, env.isMember(t, groupID, member.ID))
	// Ledger history kept.
	assert.Equal(t, ledgerBefore, env.countLedgerRows(t, groupID, member.ID))
	// Allowance config deleted.
	assert.Equal(t, 0, env.countAllowanceRows(t, groupID, member.ID))
}

// TestRemoveMember_MemberLeavesSelf asserts 204 when a member removes themselves.
func TestRemoveMember_MemberLeavesSelf(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	head, member, _ := env.seedHygieneGroup(t, "ls1", "pass")

	groups, err := env.groupRepo.ListForUserWithSummary(t.Context(), head.ID)
	require.NoError(t, err)
	groupID := groups[0].ID

	// Balance is zero (no entries).
	w := doRequest(env.router, http.MethodDelete, removePath(groupID, member.ID), nil, bearerToken(t, member.ID))
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.False(t, env.isMember(t, groupID, member.ID))
}

// TestRemoveMember_NonHeadRemovesOther asserts 403.
func TestRemoveMember_NonHeadRemovesOther(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	head, memberA, _ := env.seedHygieneGroup(t, "403a", "pass")
	memberB, err := env.userRepo.Create(t.Context(), "memberb-403a@example.com", "hash", "MemberB", nil, nil)
	require.NoError(t, err)

	groups, err := env.groupRepo.ListForUserWithSummary(t.Context(), head.ID)
	require.NoError(t, err)
	groupID := groups[0].ID

	_, err = env.groupRepo.AddMember(t.Context(), groupID, memberB.ID, models.RoleMember)
	require.NoError(t, err)

	// memberA tries to remove memberB — should be 403.
	w := doRequest(env.router, http.MethodDelete, removePath(groupID, memberB.ID), nil, bearerToken(t, memberA.ID))
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestRemoveMember_NonMemberCaller asserts 403.
func TestRemoveMember_NonMemberCaller(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	head, member, _ := env.seedHygieneGroup(t, "403b", "pass")
	outsider, err := env.userRepo.Create(t.Context(), "outsider-403b@example.com", "hash", "Outsider", nil, nil)
	require.NoError(t, err)

	groups, err := env.groupRepo.ListForUserWithSummary(t.Context(), head.ID)
	require.NoError(t, err)
	groupID := groups[0].ID

	w := doRequest(env.router, http.MethodDelete, removePath(groupID, member.ID), nil, bearerToken(t, outsider.ID))
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestRemoveMember_TargetNotMember asserts 404.
func TestRemoveMember_TargetNotMember(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	head, _, _ := env.seedHygieneGroup(t, "404a", "pass")

	groups, err := env.groupRepo.ListForUserWithSummary(t.Context(), head.ID)
	require.NoError(t, err)
	groupID := groups[0].ID

	ghost := uuid.New()
	w := doRequest(env.router, http.MethodDelete, removePath(groupID, ghost), nil, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestRemoveMember_HeadCannotLeave asserts 409 with the head-invariant message (D6).
func TestRemoveMember_HeadCannotLeave(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	head, _, _ := env.seedHygieneGroup(t, "hcl1", "pass")

	groups, err := env.groupRepo.ListForUserWithSummary(t.Context(), head.ID)
	require.NoError(t, err)
	groupID := groups[0].ID

	w := doRequest(env.router, http.MethodDelete, removePath(groupID, head.ID), nil, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["error"], "head")
	// Membership must still be intact.
	assert.True(t, env.isMember(t, groupID, head.ID))
}

// TestRemoveMember_NonZeroBalance asserts 409 when balance ≠ 0.
func TestRemoveMember_NonZeroBalance(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	head, member, _ := env.seedHygieneGroup(t, "bal1", "pass")

	groups, err := env.groupRepo.ListForUserWithSummary(t.Context(), head.ID)
	require.NoError(t, err)
	groupID := groups[0].ID

	// Add an approved credit with no offsetting debit → non-zero balance.
	_, err = env.pool.Exec(t.Context(), `
		INSERT INTO ledger_entries (id, group_id, user_id, amount, status, entry_type, direction, created_by_user_id)
		VALUES ($1, $2, $3, 50000, 'approved', 'adjustment', 'credit', $4)`,
		uuid.New(), groupID, member.ID, head.ID)
	require.NoError(t, err)

	w := doRequest(env.router, http.MethodDelete, removePath(groupID, member.ID), nil, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["error"], "balance")
	assert.True(t, env.isMember(t, groupID, member.ID))
}

// TestRemoveMember_ActiveLoan asserts 409 for a member with an active loan.
func TestRemoveMember_ActiveLoan(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	head, member, _ := env.seedHygieneGroup(t, "loan1", "pass")

	groups, err := env.groupRepo.ListForUserWithSummary(t.Context(), head.ID)
	require.NoError(t, err)
	groupID := groups[0].ID

	// Create an active loan for the member (balance = 0 from the loan rows — the
	// block comes from the loan check, not balance).
	now := time.Now()
	period := now.Format("2006-01")
	_, err = env.loanRepo.Create(t.Context(), groupID, member.ID,
		100000, 12, 8333, models.LoanStatusActive, &period, nil, &head.ID, &now)
	require.NoError(t, err)

	w := doRequest(env.router, http.MethodDelete, removePath(groupID, member.ID), nil, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["error"], "loan")
}

// TestRemoveMember_RequestedLoan asserts 409 for a member with a requested loan (D5).
func TestRemoveMember_RequestedLoan(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	head, member, _ := env.seedHygieneGroup(t, "loan2", "pass")

	groups, err := env.groupRepo.ListForUserWithSummary(t.Context(), head.ID)
	require.NoError(t, err)
	groupID := groups[0].ID

	// Create a requested loan.
	_, err = env.loanRepo.Create(t.Context(), groupID, member.ID,
		100000, 12, 8333, models.LoanStatusRequested, nil, nil, nil, nil)
	require.NoError(t, err)

	w := doRequest(env.router, http.MethodDelete, removePath(groupID, member.ID), nil, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["error"], "loan")
}

// TestRemoveMember_StaleUnpostedAllowance is the D3 correctness case (§6.14).
// Member is settled to ₹0; they have a due allowance that PostDue hasn't posted yet.
// Without PostDue the balance reads 0 → should let them out; WITH PostDue it's non-zero → 409.
func TestRemoveMember_StaleUnpostedAllowance(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	head, member, _ := env.seedHygieneGroup(t, "d3a", "pass")

	groups, err := env.groupRepo.ListForUserWithSummary(t.Context(), head.ID)
	require.NoError(t, err)
	groupID := groups[0].ID

	// Backdate joined_at so allowance backfill sees a prior month as due.
	twoMonthsAgo := time.Now().AddDate(0, -2, 0).Format("2006-01") + "-01T00:00:00Z"
	_, err = env.pool.Exec(t.Context(),
		`UPDATE group_members SET joined_at = $1::timestamptz WHERE group_id = $2 AND user_id = $3`,
		twoMonthsAgo, groupID, member.ID)
	require.NoError(t, err)

	// Set an allowance effective from two months ago — it has NOT been posted yet.
	twoMonthsPeriod := time.Now().AddDate(0, -2, 0).Format("2006-01")
	_, err = env.allowanceRepo.SetAllowance(t.Context(), groupID, member.ID, 50000, twoMonthsPeriod, head.ID)
	require.NoError(t, err)

	// Confirm: no ledger entries yet (PostDue hasn't run).
	preLedger := env.countLedgerRows(t, groupID, member.ID)
	assert.Equal(t, 0, preLedger)

	// DELETE should trigger PostDue internally → balance becomes non-zero → 409.
	w := doRequest(env.router, http.MethodDelete, removePath(groupID, member.ID), nil, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["error"], "balance")

	// PostDue ran: at least one allowance ledger row now exists.
	postLedger := env.countLedgerRows(t, groupID, member.ID)
	assert.Greater(t, postLedger, 0, "PostDue must have posted allowance entries before the balance check")
}

// TestRemoveMember_LeaveBlockedByBalance asserts member can't leave with non-zero balance.
func TestRemoveMember_LeaveBlockedByBalance(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	head, member, _ := env.seedHygieneGroup(t, "lvbal1", "pass")

	groups, err := env.groupRepo.ListForUserWithSummary(t.Context(), head.ID)
	require.NoError(t, err)
	groupID := groups[0].ID

	// Non-zero credit.
	_, err = env.pool.Exec(t.Context(), `
		INSERT INTO ledger_entries (id, group_id, user_id, amount, status, entry_type, direction, created_by_user_id)
		VALUES ($1, $2, $3, 30000, 'approved', 'adjustment', 'credit', $4)`,
		uuid.New(), groupID, member.ID, head.ID)
	require.NoError(t, err)

	w := doRequest(env.router, http.MethodDelete, removePath(groupID, member.ID), nil, bearerToken(t, member.ID))
	assert.Equal(t, http.StatusConflict, w.Code)
}

// TestRemoveMember_PendingAutoRejected asserts that a pending entry is auto-rejected on removal (D7).
func TestRemoveMember_PendingAutoRejected(t *testing.T) {
	env := setupHygieneTestEnv(t)
	defer env.cleanup()

	head, member, _ := env.seedHygieneGroup(t, "d7p1", "pass")

	groups, err := env.groupRepo.ListForUserWithSummary(t.Context(), head.ID)
	require.NoError(t, err)
	groupID := groups[0].ID

	// Insert a pending_approval entry (doesn't count toward balance).
	entryID := uuid.New()
	_, err = env.pool.Exec(t.Context(), `
		INSERT INTO ledger_entries (id, group_id, user_id, amount, status, entry_type, direction, created_by_user_id)
		VALUES ($1, $2, $3, 10000, 'pending_approval', 'adjustment', 'credit', $4)`,
		entryID, groupID, member.ID, member.ID)
	require.NoError(t, err)

	// Balance is 0 (pending doesn't count) → removal should succeed.
	w := doRequest(env.router, http.MethodDelete, removePath(groupID, member.ID), nil, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Membership gone.
	assert.False(t, env.isMember(t, groupID, member.ID))

	// Pending entry auto-rejected.
	var status string
	var decidedBy *uuid.UUID
	err = env.pool.QueryRow(t.Context(),
		`SELECT status, decided_by FROM ledger_entries WHERE id = $1`, entryID).Scan(&status, &decidedBy)
	require.NoError(t, err)
	assert.Equal(t, "rejected", status)
	assert.NotNil(t, decidedBy)
	assert.Equal(t, head.ID, *decidedBy)
}
