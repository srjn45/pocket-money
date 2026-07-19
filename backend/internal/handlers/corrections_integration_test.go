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

	"github.com/srjn45/pocket-money/backend/internal/auth"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/handlers"
	"github.com/srjn45/pocket-money/backend/internal/models"
	"github.com/srjn45/pocket-money/backend/internal/posting"
	"github.com/srjn45/pocket-money/backend/testutil"
)

// correctionsTestEnv wires the ledger + group + chore handlers with every route the
// V3-3.2 corrections/flag tests exercise, and exposes the pool so tests can read
// the invisible entry_audit table and backdate manual entries into past months.
type correctionsTestEnv struct {
	router        *gin.Engine
	pool          *pgxpool.Pool
	userRepo      *db.UserRepo
	groupRepo     *db.GroupRepo
	choreRepo     *db.ChoreRepo
	ledgerRepo    *db.LedgerRepo
	allowanceRepo *db.AllowanceRepo
	cleanup       func()
}

func setupCorrectionsTestEnv(t *testing.T) *correctionsTestEnv {
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
	notificationRepo := db.NewNotificationRepo(pool)
	auditRepo := db.NewAuditRepo(pool)

	postingSvc := posting.NewService(allowanceRepo, ledgerRepo, loanRepo, groupRepo, pool)

	lh := handlers.NewLedgerHandler(ledgerRepo, groupRepo, choreRepo, postingSvc, pool, auditRepo, db.NewNotificationRepo(pool))
	gh := handlers.NewGroupHandler(groupRepo, inviteRepo, choreRepo, ledgerRepo, loanRepo, allowanceRepo, userRepo, notificationRepo, postingSvc, pool, "")
	ch := handlers.NewChoreHandler(choreRepo, groupRepo)

	router := gin.New()
	router.Use(auth.AuthMiddleware(testJWTSecret))
	router.POST("/groups/:id/ledger", lh.CreateLedger)
	router.POST("/ledger/:id/approve", lh.ApproveLedger)
	router.POST("/ledger/:id/reject", lh.RejectLedger)
	router.PUT("/ledger/:id", lh.EditLedger)
	router.DELETE("/ledger/:id", lh.DeleteLedger)
	router.GET("/groups/:id/statement", lh.GetStatement)
	router.GET("/groups/:id/balance", lh.GetBalance)
	router.GET("/groups/:id", gh.GetGroup)
	router.PATCH("/groups/:id", gh.UpdateGroup)
	router.PATCH("/chores/:id", ch.UpdateChore)

	return &correctionsTestEnv{
		router:        router,
		pool:          pool,
		userRepo:      userRepo,
		groupRepo:     groupRepo,
		choreRepo:     choreRepo,
		ledgerRepo:    ledgerRepo,
		allowanceRepo: allowanceRepo,
		cleanup: func() {
			testutil.CleanupTestDB(pool)
			pool.Close()
		},
	}
}

// seed creates admin + member users and a group (INR), adding both to
// group_members (admin explicitly, per project memory), plus a non-system chore.
func (e *correctionsTestEnv) seed(t *testing.T, suffix string) (admin, member *models.User, group *models.Group, chore *models.Chore) {
	t.Helper()
	ctx := t.Context()
	var err error
	admin, err = e.userRepo.Create(ctx, fmt.Sprintf("admin-corr-%s@example.com", suffix), "hash", "Admin", nil, nil)
	require.NoError(t, err)
	member, err = e.userRepo.Create(ctx, fmt.Sprintf("member-corr-%s@example.com", suffix), "hash", "Member", nil, nil)
	require.NoError(t, err)
	group, err = e.groupRepo.Create(ctx, "Family "+suffix, admin.ID, models.CurrencyINR)
	require.NoError(t, err)
	_, err = e.groupRepo.AddMember(ctx, group.ID, admin.ID, models.RoleAdmin)
	require.NoError(t, err)
	_, err = e.groupRepo.AddMember(ctx, group.ID, member.ID, models.RoleMember)
	require.NoError(t, err)
	chore, err = e.choreRepo.Create(ctx, group.ID, "Dishes", nil, 1000)
	require.NoError(t, err)
	return
}

func (e *correctionsTestEnv) enableFlag(t *testing.T, groupID uuid.UUID) {
	t.Helper()
	require.NoError(t, e.groupRepo.SetChoreSubmissionEnabled(t.Context(), groupID, true))
}

func (e *correctionsTestEnv) backdateJoin(t *testing.T, groupID, userID uuid.UUID, period string) {
	t.Helper()
	_, err := e.pool.Exec(t.Context(),
		`UPDATE group_members SET joined_at = $1::timestamptz WHERE group_id = $2 AND user_id = $3`,
		period+"-01T00:00:00Z", groupID, userID)
	require.NoError(t, err)
}

func (e *correctionsTestEnv) backdateEntry(t *testing.T, entryID uuid.UUID, period string) {
	t.Helper()
	_, err := e.pool.Exec(t.Context(),
		`UPDATE ledger_entries SET created_at = $1::timestamptz WHERE id = $2`,
		period+"-15T12:00:00Z", entryID)
	require.NoError(t, err)
}

// auditRow is the subset of an entry_audit row the tests assert on.
type auditRow struct {
	action    string
	actor     uuid.UUID
	amount    int64
	entryType string
	entryID   *uuid.UUID
}

// readAudit returns every entry_audit row (there is no read API — tests peek at
// the table directly). old_row is queried via jsonb accessors so the assertions
// match the model's json tags.
func (e *correctionsTestEnv) readAudit(t *testing.T) []auditRow {
	t.Helper()
	rows, err := e.pool.Query(t.Context(), `
		SELECT action, actor, (old_row->>'amount')::bigint, old_row->>'entry_type', entry_id
		FROM entry_audit ORDER BY at`)
	require.NoError(t, err)
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var a auditRow
		require.NoError(t, rows.Scan(&a.action, &a.actor, &a.amount, &a.entryType, &a.entryID))
		out = append(out, a)
	}
	require.NoError(t, rows.Err())
	return out
}

func (e *correctionsTestEnv) getStatement(t *testing.T, groupID, callerID uuid.UUID, period string) handlers.StatementResponse {
	t.Helper()
	w := doRequest(e.router, http.MethodGet,
		fmt.Sprintf("/groups/%s/statement?period=%s", groupID, period), nil, bearerToken(t, callerID))
	require.Equal(t, http.StatusOK, w.Code)
	var resp handlers.StatementResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// seedApprovedManual creates an approved manual entry directly via the repo (so
// tests control type/amount/direction) and returns it.
func (e *correctionsTestEnv) seedApprovedManual(t *testing.T, group *models.Group, userID, adminID uuid.UUID,
	amount int64, etype models.LedgerEntryType, dir models.LedgerDirection) *models.LedgerEntry {
	t.Helper()
	entry, err := e.ledgerRepo.Create(t.Context(), group.ID, userID, nil, adminID, amount,
		etype, dir, models.StatusApproved, nil, &adminID, ptrTime(time.Now()))
	require.NoError(t, err)
	return entry
}

// --- edit recomputes statement + preserves invariant --------------------------

func TestCorrections_EditRecomputesAndPreservesInvariant(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	admin, member, group, _ := env.seed(t, "editinv")

	m, mNext := monthsBack(1), monthsBack(0)

	// Allowance from m so both m and m+1 carry rows; join backdated to m.
	env.backdateJoin(t, group.ID, member.ID, m)
	_, err := env.allowanceRepo.SetAllowance(ctx, group.ID, member.ID, 1000, m, admin.ID)
	require.NoError(t, err)

	// An approved chore credit in month m (backdated).
	chore := env.seedApprovedManual(t, group, member.ID, admin.ID, 500, models.EntryTypeChore, models.DirectionCredit)
	env.backdateEntry(t, chore.ID, m)

	// Pre-edit: record closing(m) and opening(m+1); assert equal.
	preClosing := memberRow(t, env.getStatement(t, group.ID, admin.ID, m), member.ID).ClosingBalance.Value
	preNextOpening := memberRow(t, env.getStatement(t, group.ID, admin.ID, mNext), member.ID).OpeningBalance.Value
	require.Equal(t, preClosing, preNextOpening, "invariant must hold before the edit")

	// Edit the chore amount 500 → 800 (Δ = +300).
	w := doRequest(env.router, http.MethodPut, "/ledger/"+chore.ID.String(),
		map[string]interface{}{"amount": inr(800)}, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var updated handlers.LedgerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	assert.Equal(t, int64(800), updated.Amount.Value)

	// Post-edit: month figure moved by Δ; both invariant sides moved equally.
	postClosing := memberRow(t, env.getStatement(t, group.ID, admin.ID, m), member.ID).ClosingBalance.Value
	postNextOpening := memberRow(t, env.getStatement(t, group.ID, admin.ID, mNext), member.ID).OpeningBalance.Value
	assert.Equal(t, preClosing+300, postClosing, "closing(m) shifts by +Δ")
	assert.Equal(t, preNextOpening+300, postNextOpening, "opening(m+1) shifts by +Δ")
	assert.Equal(t, postClosing, postNextOpening, "invariant still holds after the edit")
}

func TestCorrections_DeleteRecomputesStatement(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	admin, member, group, _ := env.seed(t, "delinv")

	m, mNext := monthsBack(1), monthsBack(0)
	env.backdateJoin(t, group.ID, member.ID, m)
	_, err := env.allowanceRepo.SetAllowance(ctx, group.ID, member.ID, 1000, m, admin.ID)
	require.NoError(t, err)

	// A settlement debit (−300) in month m.
	settle := env.seedApprovedManual(t, group, member.ID, admin.ID, 300, models.EntryTypeSettlement, models.DirectionDebit)
	env.backdateEntry(t, settle.ID, m)

	preClosing := memberRow(t, env.getStatement(t, group.ID, admin.ID, m), member.ID).ClosingBalance.Value
	preNextOpening := memberRow(t, env.getStatement(t, group.ID, admin.ID, mNext), member.ID).OpeningBalance.Value
	require.Equal(t, preClosing, preNextOpening)

	// Delete the settlement → both sides rise by +300 (removing a −300 debit).
	w := doRequest(env.router, http.MethodDelete, "/ledger/"+settle.ID.String(), nil, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusNoContent, w.Code)

	postClosing := memberRow(t, env.getStatement(t, group.ID, admin.ID, m), member.ID).ClosingBalance.Value
	postNextOpening := memberRow(t, env.getStatement(t, group.ID, admin.ID, mNext), member.ID).OpeningBalance.Value
	assert.Equal(t, preClosing+300, postClosing, "closing(m) rises by the removed debit magnitude")
	assert.Equal(t, postClosing, postNextOpening, "invariant still holds after the delete")

	// The row is gone.
	_, err = env.ledgerRepo.GetByID(ctx, settle.ID)
	assert.ErrorIs(t, err, db.ErrNotFound)
}

// --- audit captures prior values ---------------------------------------------

func TestCorrections_EditWritesAuditWithPriorValues(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	admin, member, group, _ := env.seed(t, "editaudit")
	entry := env.seedApprovedManual(t, group, member.ID, admin.ID, 500, models.EntryTypeChore, models.DirectionCredit)

	w := doRequest(env.router, http.MethodPut, "/ledger/"+entry.ID.String(),
		map[string]interface{}{"amount": inr(750), "note": "corrected"}, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusOK, w.Code)

	audit := env.readAudit(t)
	require.Len(t, audit, 1)
	assert.Equal(t, models.AuditActionEdit, audit[0].action)
	assert.Equal(t, admin.ID, audit[0].actor)
	assert.Equal(t, int64(500), audit[0].amount, "old_row must carry the PRE-edit amount")
	assert.Equal(t, string(models.EntryTypeChore), audit[0].entryType)
	require.NotNil(t, audit[0].entryID)
	assert.Equal(t, entry.ID, *audit[0].entryID)
}

func TestCorrections_DeleteWritesAuditWithPriorValues(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	admin, member, group, _ := env.seed(t, "delaudit")
	entry := env.seedApprovedManual(t, group, member.ID, admin.ID, 420, models.EntryTypeAdjustment, models.DirectionCredit)

	w := doRequest(env.router, http.MethodDelete, "/ledger/"+entry.ID.String(), nil, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusNoContent, w.Code)

	audit := env.readAudit(t)
	require.Len(t, audit, 1)
	assert.Equal(t, models.AuditActionDelete, audit[0].action)
	assert.Equal(t, admin.ID, audit[0].actor)
	assert.Equal(t, int64(420), audit[0].amount, "old_row must carry the original amount after delete")
	assert.Equal(t, string(models.EntryTypeAdjustment), audit[0].entryType)
	// entry_id FK is ON DELETE SET NULL → the parent row is gone, audit survives.
	assert.Nil(t, audit[0].entryID, "entry_id nulled after the parent hard-delete")
}

// --- system-entry edit/delete rejected ---------------------------------------

func TestCorrections_EditSystemEntryRejected(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	admin, member, group, _ := env.seed(t, "sysreject")

	// Materialize an allowance for a past month via the posting engine, then fetch it.
	m := monthsBack(1)
	env.backdateJoin(t, group.ID, member.ID, m)
	_, err := env.allowanceRepo.SetAllowance(ctx, group.ID, member.ID, 1000, m, admin.ID)
	require.NoError(t, err)
	// Trigger PostDue by hitting the balance endpoint.
	w := doRequest(env.router, http.MethodGet, fmt.Sprintf("/groups/%s/balance", group.ID), nil, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusOK, w.Code)

	allowance := models.EntryTypeAllowance
	sysEntries, err := env.ledgerRepo.ListForGroupWithUser(ctx, group.ID, nil, &member.ID, &allowance, nil)
	require.NoError(t, err)
	require.NotEmpty(t, sysEntries, "posting must have materialized an allowance")
	sysID := sysEntries[0].ID

	// Also seed an emi entry directly (system type).
	emi, err := env.ledgerRepo.Create(ctx, group.ID, member.ID, nil, admin.ID, 200,
		models.EntryTypeEMI, models.DirectionDebit, models.StatusApproved, nil, nil, nil)
	require.NoError(t, err)

	for _, id := range []uuid.UUID{sysID, emi.ID} {
		wPut := doRequest(env.router, http.MethodPut, "/ledger/"+id.String(),
			map[string]interface{}{"amount": inr(999)}, bearerToken(t, admin.ID))
		assert.Equal(t, http.StatusForbidden, wPut.Code, "PUT system entry must be 403")
		wDel := doRequest(env.router, http.MethodDelete, "/ledger/"+id.String(), nil, bearerToken(t, admin.ID))
		assert.Equal(t, http.StatusForbidden, wDel.Code, "DELETE system entry must be 403")
	}

	// No audit rows written for the rejected attempts.
	assert.Empty(t, env.readAudit(t), "system-entry rejections must not write audit rows")

	// The system rows are unchanged.
	after, err := env.ledgerRepo.GetByID(ctx, sysID)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), after.Amount)
}

// --- D6: a member cannot edit or delete --------------------------------------

func TestCorrections_MemberCannotEditOrDelete(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	admin, member, group, _ := env.seed(t, "memberdenied")
	entry := env.seedApprovedManual(t, group, member.ID, admin.ID, 500, models.EntryTypeChore, models.DirectionCredit)

	wPut := doRequest(env.router, http.MethodPut, "/ledger/"+entry.ID.String(),
		map[string]interface{}{"amount": inr(600)}, bearerToken(t, member.ID))
	assert.Equal(t, http.StatusForbidden, wPut.Code)
	wDel := doRequest(env.router, http.MethodDelete, "/ledger/"+entry.ID.String(), nil, bearerToken(t, member.ID))
	assert.Equal(t, http.StatusForbidden, wDel.Code)
	assert.Empty(t, env.readAudit(t))
}

// --- validation: currency / amount / direction -------------------------------

func TestCorrections_EditRejectsMismatchedCurrency(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	admin, member, group, _ := env.seed(t, "curr")
	entry := env.seedApprovedManual(t, group, member.ID, admin.ID, 500, models.EntryTypeChore, models.DirectionCredit)

	w := doRequest(env.router, http.MethodPut, "/ledger/"+entry.ID.String(),
		map[string]interface{}{"amount": money(models.CurrencyUSD, 600)}, bearerToken(t, admin.ID))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCorrections_EditAmountBelowOneRejected(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	admin, member, group, _ := env.seed(t, "below1")
	entry := env.seedApprovedManual(t, group, member.ID, admin.ID, 500, models.EntryTypeChore, models.DirectionCredit)

	w := doRequest(env.router, http.MethodPut, "/ledger/"+entry.ID.String(),
		map[string]interface{}{"amount": inr(0)}, bearerToken(t, admin.ID))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCorrections_EditAdjustmentDirectionFlip(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	admin, member, group, _ := env.seed(t, "flip")
	// Adjustment credit +200.
	entry := env.seedApprovedManual(t, group, member.ID, admin.ID, 200, models.EntryTypeAdjustment, models.DirectionCredit)

	// Balance before: +200.
	balBefore := memberBalance(t, env, group.ID, admin.ID, member.ID)
	require.Equal(t, int64(200), balBefore)

	// Flip to debit, same magnitude → contribution −200 (Δ = −400 to balance).
	w := doRequest(env.router, http.MethodPut, "/ledger/"+entry.ID.String(),
		map[string]interface{}{"amount": inr(200), "direction": "debit"}, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var updated handlers.LedgerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	assert.Equal(t, models.DirectionDebit, updated.Direction)

	balAfter := memberBalance(t, env, group.ID, admin.ID, member.ID)
	assert.Equal(t, int64(-200), balAfter)
	assert.Equal(t, balBefore-400, balAfter, "direction flip moves balance by −2×amount")
}

func TestCorrections_DirectionForbiddenOnChoreSettlement(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	admin, member, group, _ := env.seed(t, "dirforbidden")
	choreEntry := env.seedApprovedManual(t, group, member.ID, admin.ID, 500, models.EntryTypeChore, models.DirectionCredit)
	settleEntry := env.seedApprovedManual(t, group, member.ID, admin.ID, 300, models.EntryTypeSettlement, models.DirectionDebit)

	for _, id := range []uuid.UUID{choreEntry.ID, settleEntry.ID} {
		w := doRequest(env.router, http.MethodPut, "/ledger/"+id.String(),
			map[string]interface{}{"amount": inr(400), "direction": "credit"}, bearerToken(t, admin.ID))
		assert.Equal(t, http.StatusBadRequest, w.Code, "direction in body for chore/settlement must be 400")
	}

	// An adjustment WITHOUT direction is also a 400.
	adj := env.seedApprovedManual(t, group, member.ID, admin.ID, 200, models.EntryTypeAdjustment, models.DirectionCredit)
	w := doRequest(env.router, http.MethodPut, "/ledger/"+adj.ID.String(),
		map[string]interface{}{"amount": inr(250)}, bearerToken(t, admin.ID))
	assert.Equal(t, http.StatusBadRequest, w.Code, "adjustment edit without direction must be 400")
}

// --- D2 flag: submission endpoints inert when OFF ----------------------------

func TestChoreSubmission_MemberSubmitForbiddenWhenFlagOff(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	_, member, group, chore := env.seed(t, "submitoff")

	// Flag defaults OFF: a member self-submitting a chore → 403.
	w := doRequest(env.router, http.MethodPost, fmt.Sprintf("/groups/%s/ledger", group.ID),
		map[string]interface{}{"entry_type": "chore", "chore_id": chore.ID}, bearerToken(t, member.ID))
	require.Equal(t, http.StatusForbidden, w.Code)
	var errResp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "member chore submission is disabled for this group", errResp["error"])
}

func TestChoreSubmission_ApproveRejectForbiddenWhenFlagOff(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	admin, member, group, chore := env.seed(t, "approveoff")

	// A pending entry can only exist while the flag was ON — seed one directly, then
	// turn the flag OFF, and assert approve/reject are inert.
	pending, err := env.ledgerRepo.Create(ctx, group.ID, member.ID, &chore.ID, member.ID, 1000,
		models.EntryTypeChore, models.DirectionCredit, models.StatusPendingApproval, nil, nil, nil)
	require.NoError(t, err)

	for _, action := range []string{"approve", "reject"} {
		w := doRequest(env.router, http.MethodPost,
			fmt.Sprintf("/ledger/%s/%s", pending.ID, action), nil, bearerToken(t, admin.ID))
		assert.Equal(t, http.StatusForbidden, w.Code, "%s must be 403 when flag OFF", action)
	}
	// The entry is still pending (neither approved nor rejected).
	after, err := env.ledgerRepo.GetByID(ctx, pending.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusPendingApproval, after.Status)
}

// --- D2 flag: functional when ON ---------------------------------------------

func TestChoreSubmission_MemberSubmitAndApproveWhenFlagOn(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	admin, member, group, chore := env.seed(t, "submiton")
	env.enableFlag(t, group.ID)

	// Member submits → 201 pending.
	w := doRequest(env.router, http.MethodPost, fmt.Sprintf("/groups/%s/ledger", group.ID),
		map[string]interface{}{"entry_type": "chore", "chore_id": chore.ID}, bearerToken(t, member.ID))
	require.Equal(t, http.StatusCreated, w.Code)
	var pending handlers.LedgerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pending))
	require.Equal(t, models.StatusPendingApproval, pending.Status)

	// Admin approves → 200 approved.
	w = doRequest(env.router, http.MethodPost, fmt.Sprintf("/ledger/%s/approve", pending.ID), nil, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var approved handlers.LedgerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &approved))
	assert.Equal(t, models.StatusApproved, approved.Status)
}

// --- D2 admin toggle + flag surfaced in responses ----------------------------

func TestGroupConfig_SetFlagAdminOnly(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	admin, member, group, _ := env.seed(t, "toggle")

	// Member PATCH → 403.
	w := doRequest(env.router, http.MethodPatch, "/groups/"+group.ID.String(),
		map[string]interface{}{"member_chore_submission_enabled": true}, bearerToken(t, member.ID))
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Admin PATCH true → 200, flag reflected.
	w = doRequest(env.router, http.MethodPatch, "/groups/"+group.ID.String(),
		map[string]interface{}{"member_chore_submission_enabled": true}, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var resp handlers.GroupResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.MemberChoreSubmissionEnabled)

	// Persisted.
	enabled, err := env.groupRepo.GetChoreSubmissionEnabled(t.Context(), group.ID)
	require.NoError(t, err)
	assert.True(t, enabled)

	// Admin PATCH false → 200, flag off.
	w = doRequest(env.router, http.MethodPatch, "/groups/"+group.ID.String(),
		map[string]interface{}{"member_chore_submission_enabled": false}, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.MemberChoreSubmissionEnabled)
}

func TestGroupConfig_FlagSurfacedInGroupResponse(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	admin, _, group, _ := env.seed(t, "surfaced")

	// GET reflects default OFF.
	w := doRequest(env.router, http.MethodGet, "/groups/"+group.ID.String(), nil, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var detail handlers.GroupDetailResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &detail))
	assert.False(t, detail.MemberChoreSubmissionEnabled)

	// Enable, then GET reflects ON.
	env.enableFlag(t, group.ID)
	w = doRequest(env.router, http.MethodGet, "/groups/"+group.ID.String(), nil, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &detail))
	assert.True(t, detail.MemberChoreSubmissionEnabled)
}

// --- chore description edit route (§4) ---------------------------------------

func TestChore_UpdateDescriptionRoute(t *testing.T) {
	env := setupCorrectionsTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	admin, _, group, chore := env.seed(t, "choredesc")

	newDesc := "Wash all the dishes"
	w := doRequest(env.router, http.MethodPatch, "/chores/"+chore.ID.String(),
		map[string]interface{}{"description": newDesc}, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var resp handlers.ChoreResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Description)
	assert.Equal(t, newDesc, *resp.Description)

	// A system chore (the default Settlement chore) cannot be edited → 403.
	sysChore, err := env.choreRepo.CreateWithSystem(ctx, group.ID, "Settlement", ptrString("payout"), 0, true)
	require.NoError(t, err)
	w = doRequest(env.router, http.MethodPatch, "/chores/"+sysChore.ID.String(),
		map[string]interface{}{"description": "hacked"}, bearerToken(t, admin.ID))
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// memberBalance fetches a single member's balance via GET /balance (admin caller).
func memberBalance(t *testing.T, env *correctionsTestEnv, groupID, adminID, memberID uuid.UUID) int64 {
	t.Helper()
	w := doRequest(env.router, http.MethodGet, fmt.Sprintf("/groups/%s/balance", groupID), nil, bearerToken(t, adminID))
	require.Equal(t, http.StatusOK, w.Code)
	var balances []handlers.BalanceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &balances))
	for _, b := range balances {
		if b.UserID == memberID {
			return b.Balance.Value
		}
	}
	t.Fatalf("member %s not found in balance response", memberID)
	return 0
}

func ptrString(s string) *string { return &s }
