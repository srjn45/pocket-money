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

// statementTestEnv wires the statement handler for integration tests. It exposes
// the pool and the allowance/loan repos so tests can seed backdated multi-month
// scenarios (PostDue floors at join/start month, so joined_at/effective_from/
// start_period must be moved into the past — §9.11).
type statementTestEnv struct {
	router        *gin.Engine
	pool          *pgxpool.Pool
	userRepo      *db.UserRepo
	groupRepo     *db.GroupRepo
	ledgerRepo    *db.LedgerRepo
	allowanceRepo *db.AllowanceRepo
	loanRepo      *db.LoanRepo
	cleanup       func()
}

func setupStatementTestEnv(t *testing.T) *statementTestEnv {
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

	postingSvc := posting.NewService(allowanceRepo, ledgerRepo, loanRepo, groupRepo, pool)

	lh := handlers.NewLedgerHandler(ledgerRepo, groupRepo, choreRepo, postingSvc, pool, db.NewAuditRepo(pool), db.NewNotificationRepo(pool))

	router := gin.New()
	router.Use(auth.AuthMiddleware(testJWTSecret))
	router.GET("/groups/:id/statement", lh.GetStatement)

	return &statementTestEnv{
		router:        router,
		pool:          pool,
		userRepo:      userRepo,
		groupRepo:     groupRepo,
		ledgerRepo:    ledgerRepo,
		allowanceRepo: allowanceRepo,
		loanRepo:      loanRepo,
		cleanup: func() {
			testutil.CleanupTestDB(pool)
			pool.Close()
		},
	}
}

// seedGroup creates an admin + member and a group in the given currency, adding
// both to group_members (admin explicitly, per §9.10).
func (e *statementTestEnv) seedGroup(t *testing.T, suffix, currency string) (admin, member *models.User, group *models.Group) {
	t.Helper()
	ctx := t.Context()
	var err error
	admin, err = e.userRepo.Create(ctx, fmt.Sprintf("admin-%s@example.com", suffix), "hash", "Admin", nil, nil)
	require.NoError(t, err)
	member, err = e.userRepo.Create(ctx, fmt.Sprintf("member-%s@example.com", suffix), "hash", "Member", nil, nil)
	require.NoError(t, err)
	group, err = e.groupRepo.Create(ctx, "Family "+suffix, admin.ID, currency)
	require.NoError(t, err)
	_, err = e.groupRepo.AddMember(ctx, group.ID, admin.ID, models.RoleAdmin)
	require.NoError(t, err)
	_, err = e.groupRepo.AddMember(ctx, group.ID, member.ID, models.RoleMember)
	require.NoError(t, err)
	return
}

// backdateJoin moves a member's joined_at to the first day of the given month.
func (e *statementTestEnv) backdateJoin(t *testing.T, groupID, userID uuid.UUID, period string) {
	t.Helper()
	_, err := e.pool.Exec(t.Context(),
		`UPDATE group_members SET joined_at = $1::timestamptz WHERE group_id = $2 AND user_id = $3`,
		period+"-01T00:00:00Z", groupID, userID)
	require.NoError(t, err)
}

// backdateEntry sets a manual entry's created_at to mid-month of the given period
// (mid-month avoids any TZ boundary ambiguity), so it buckets into that month via
// effMonth-on-created_at — exercising the full pgx UTC round-trip.
func (e *statementTestEnv) backdateEntry(t *testing.T, entryID uuid.UUID, period string) {
	t.Helper()
	_, err := e.pool.Exec(t.Context(),
		`UPDATE ledger_entries SET created_at = $1::timestamptz WHERE id = $2`,
		period+"-15T12:00:00Z", entryID)
	require.NoError(t, err)
}

// getStatement issues an authenticated GET and returns the decoded response.
func (e *statementTestEnv) getStatement(t *testing.T, groupID, callerID uuid.UUID, query string) (int, handlers.StatementResponse) {
	t.Helper()
	path := fmt.Sprintf("/groups/%s/statement", groupID)
	if query != "" {
		path += "?" + query
	}
	w := doRequest(e.router, http.MethodGet, path, nil, bearerToken(t, callerID))
	var resp handlers.StatementResponse
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	}
	return w.Code, resp
}

// monthsBack returns the YYYY-MM for n months before the current server-local
// month, normalized to the first of the month so day-of-month overflow can't
// skew the arithmetic.
func monthsBack(n int) string {
	now := time.Now()
	base := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	return base.AddDate(0, -n, 0).Format("2006-01")
}

// memberRow finds the row for userID in a statement response.
func memberRow(t *testing.T, resp handlers.StatementResponse, userID uuid.UUID) handlers.MemberStatementResponse {
	t.Helper()
	for _, m := range resp.Members {
		if m.UserID == userID {
			return m
		}
	}
	t.Fatalf("member %s not found in statement", userID)
	return handlers.MemberStatementResponse{}
}

// --- TestStatement_ClosingEqualsNextOpening_Backfill (the §8 gate) -------

func TestStatement_ClosingEqualsNextOpening_Backfill(t *testing.T) {
	env := setupStatementTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	admin, member, group := env.seedGroup(t, "backfill", models.CurrencyINR)

	m0, m1, m2 := monthsBack(2), monthsBack(1), monthsBack(0)

	// Allowance 1000/month from m0; join backdated to m0 so PostDue backfills
	// m0, m1, m2.
	env.backdateJoin(t, group.ID, member.ID, m0)
	_, err := env.allowanceRepo.SetAllowance(ctx, group.ID, member.ID, 1000, m0, admin.ID)
	require.NoError(t, err)

	// A chore credit landing in m1 and a payment landing in m2 (manual entries
	// bucketed by created_at → backdate them).
	chore, err := env.ledgerRepo.Create(ctx, group.ID, member.ID, nil, admin.ID, 500,
		models.EntryTypeAdjustment, models.DirectionCredit, models.StatusApproved, nil, &admin.ID, ptrTime(time.Now()))
	require.NoError(t, err)
	env.backdateEntry(t, chore.ID, m1)

	pay, err := env.ledgerRepo.Create(ctx, group.ID, member.ID, nil, admin.ID, 300,
		models.EntryTypeSettlement, models.DirectionDebit, models.StatusApproved, nil, &admin.ID, ptrTime(time.Now()))
	require.NoError(t, err)
	env.backdateEntry(t, pay.ID, m2)

	code0, s0 := env.getStatement(t, group.ID, admin.ID, "period="+m0)
	require.Equal(t, http.StatusOK, code0)
	code1, s1 := env.getStatement(t, group.ID, admin.ID, "period="+m1)
	require.Equal(t, http.StatusOK, code1)
	code2, s2 := env.getStatement(t, group.ID, admin.ID, "period="+m2)
	require.Equal(t, http.StatusOK, code2)

	r0 := memberRow(t, s0, member.ID)
	r1 := memberRow(t, s1, member.ID)
	r2 := memberRow(t, s2, member.ID)

	// The core invariant across consecutive months.
	assert.Equal(t, r0.ClosingBalance.Value, r1.OpeningBalance.Value, "closing(m0) == opening(m1)")
	assert.Equal(t, r1.ClosingBalance.Value, r2.OpeningBalance.Value, "closing(m1) == opening(m2)")

	// Same currency stamped everywhere.
	assert.Equal(t, models.CurrencyINR, s0.Currency)
	assert.Equal(t, models.CurrencyINR, r0.OpeningBalance.Currency)
	assert.Equal(t, models.CurrencyINR, r2.ClosingBalance.Currency)
}

// --- TestStatement_MidMonthJoinFlooring ---------------------------------

func TestStatement_MidMonthJoinFlooring(t *testing.T) {
	env := setupStatementTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	admin, member, group := env.seedGroup(t, "join", models.CurrencyINR)

	joinMonth := monthsBack(1)
	preJoin := monthsBack(2)
	env.backdateJoin(t, group.ID, member.ID, joinMonth)
	_, err := env.allowanceRepo.SetAllowance(ctx, group.ID, member.ID, 1000, joinMonth, admin.ID)
	require.NoError(t, err)

	// Pre-join month: member absent from the admin view.
	code, adminPre := env.getStatement(t, group.ID, admin.ID, "period="+preJoin)
	require.Equal(t, http.StatusOK, code)
	for _, m := range adminPre.Members {
		assert.NotEqual(t, member.ID, m.UserID, "member must be absent before their join month")
	}

	// Pre-join month, member's own call: 200 with an empty members list (no row).
	code, memberPre := env.getStatement(t, group.ID, member.ID, "period="+preJoin)
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, memberPre.Members, "member sees no row before joining")

	// Join month: row present with opening == 0.
	code, joinResp := env.getStatement(t, group.ID, member.ID, "period="+joinMonth)
	require.Equal(t, http.StatusOK, code)
	row := memberRow(t, joinResp, member.ID)
	assert.Equal(t, int64(0), row.OpeningBalance.Value, "first (join) month opening is 0")
}

// --- TestStatement_LoanFinalPartialEMI ----------------------------------

func TestStatement_LoanFinalPartialEMI(t *testing.T) {
	env := setupStatementTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	admin, member, group := env.seedGroup(t, "loan", models.CurrencyINR)

	m0, m1, m2, m3 := monthsBack(2), monthsBack(1), monthsBack(0), monthsBack(-1)
	env.backdateJoin(t, group.ID, member.ID, m0)

	// principal 1000, 3 installments, emi 334 → 334, 334, and a final partial 332.
	start := m0
	decidedAt := time.Now()
	_, err := env.loanRepo.Create(ctx, group.ID, member.ID, 1000, 3, 334,
		models.LoanStatusActive, &start, nil, &admin.ID, &decidedAt)
	require.NoError(t, err)

	code0, s0 := env.getStatement(t, group.ID, admin.ID, "period="+m0)
	require.Equal(t, http.StatusOK, code0)
	code1, s1 := env.getStatement(t, group.ID, admin.ID, "period="+m1)
	require.Equal(t, http.StatusOK, code1)
	code2, s2 := env.getStatement(t, group.ID, admin.ID, "period="+m2)
	require.Equal(t, http.StatusOK, code2)
	code3, s3 := env.getStatement(t, group.ID, admin.ID, "period="+m3)
	require.Equal(t, http.StatusOK, code3)

	r0 := memberRow(t, s0, member.ID)
	r1 := memberRow(t, s1, member.ID)
	r2 := memberRow(t, s2, member.ID)
	r3 := memberRow(t, s3, member.ID)

	assert.Equal(t, int64(334), r0.EMI.Value)
	assert.Equal(t, int64(334), r1.EMI.Value)
	assert.Equal(t, int64(332), r2.EMI.Value, "final installment is the partial remainder")
	assert.Equal(t, int64(0), r3.EMI.Value, "no EMI after the loan is fully repaid")

	// Closing carries unchanged into the loan-free next month.
	assert.Equal(t, r2.ClosingBalance.Value, r3.OpeningBalance.Value)
	assert.Equal(t, r3.OpeningBalance.Value, r3.ClosingBalance.Value, "loan-free month: no movement")
	// Total repaid equals the principal.
	assert.Equal(t, int64(-1000), r2.ClosingBalance.Value)
}

// --- TestStatement_MemberSeesOwnRowOnly ---------------------------------

func TestStatement_MemberSeesOwnRowOnly(t *testing.T) {
	env := setupStatementTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	admin, memberA, group := env.seedGroup(t, "own", models.CurrencyINR)
	memberB, err := env.userRepo.Create(ctx, "memberb-own@example.com", "hash", "MemberB", nil, nil)
	require.NoError(t, err)
	_, err = env.groupRepo.AddMember(ctx, group.ID, memberB.ID, models.RoleMember)
	require.NoError(t, err)

	month := monthsBack(0)
	env.backdateJoin(t, group.ID, memberA.ID, month)
	env.backdateJoin(t, group.ID, memberB.ID, month)

	// Give each member a distinct approved credit this month.
	a, err := env.ledgerRepo.Create(ctx, group.ID, memberA.ID, nil, admin.ID, 700,
		models.EntryTypeAdjustment, models.DirectionCredit, models.StatusApproved, nil, &admin.ID, ptrTime(time.Now()))
	require.NoError(t, err)
	env.backdateEntry(t, a.ID, month)
	b, err := env.ledgerRepo.Create(ctx, group.ID, memberB.ID, nil, admin.ID, 400,
		models.EntryTypeAdjustment, models.DirectionCredit, models.StatusApproved, nil, &admin.ID, ptrTime(time.Now()))
	require.NoError(t, err)
	env.backdateEntry(t, b.ID, month)

	// Member caller → exactly their own row, no group_total.
	code, memberResp := env.getStatement(t, group.ID, memberA.ID, "period="+month)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, memberResp.Members, 1)
	assert.Equal(t, memberA.ID, memberResp.Members[0].UserID)
	assert.Nil(t, memberResp.GroupTotal, "member never sees a group total")

	// Member requesting another member → 403.
	path := fmt.Sprintf("/groups/%s/statement?period=%s&user_id=%s", group.ID, month, memberB.ID)
	w := doRequest(env.router, http.MethodGet, path, nil, bearerToken(t, memberA.ID))
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Admin caller → both member rows and a non-null group total = element-wise sum.
	code, adminResp := env.getStatement(t, group.ID, admin.ID, "period="+month)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, adminResp.Members, 2)
	require.NotNil(t, adminResp.GroupTotal)
	ra := memberRow(t, adminResp, memberA.ID)
	rb := memberRow(t, adminResp, memberB.ID)
	assert.Equal(t, ra.ClosingBalance.Value+rb.ClosingBalance.Value, adminResp.GroupTotal.ClosingBalance.Value)
	assert.Equal(t, ra.Adjustments.Value+rb.Adjustments.Value, adminResp.GroupTotal.Adjustments.Value)

	// Admin narrowing via user_id → single row, group_total omitted.
	code, narrowed := env.getStatement(t, group.ID, admin.ID, "period="+month+"&user_id="+memberA.ID.String())
	require.Equal(t, http.StatusOK, code)
	require.Len(t, narrowed.Members, 1)
	assert.Equal(t, memberA.ID, narrowed.Members[0].UserID)
	assert.Nil(t, narrowed.GroupTotal, "narrowed admin view omits the group total")
}

// --- TestStatement_CurrencyFromGroupOnly --------------------------------

func TestStatement_CurrencyFromGroupOnly(t *testing.T) {
	env := setupStatementTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	month := monthsBack(0)

	adminINR, memberINR, groupINR := env.seedGroup(t, "cur-inr", models.CurrencyINR)
	adminEUR, memberEUR, groupEUR := env.seedGroup(t, "cur-eur", models.CurrencyEUR)
	env.backdateJoin(t, groupINR.ID, memberINR.ID, month)
	env.backdateJoin(t, groupEUR.ID, memberEUR.ID, month)

	iEntry, err := env.ledgerRepo.Create(ctx, groupINR.ID, memberINR.ID, nil, adminINR.ID, 1000,
		models.EntryTypeAdjustment, models.DirectionCredit, models.StatusApproved, nil, &adminINR.ID, ptrTime(time.Now()))
	require.NoError(t, err)
	env.backdateEntry(t, iEntry.ID, month)
	eEntry, err := env.ledgerRepo.Create(ctx, groupEUR.ID, memberEUR.ID, nil, adminEUR.ID, 2000,
		models.EntryTypeAdjustment, models.DirectionCredit, models.StatusApproved, nil, &adminEUR.ID, ptrTime(time.Now()))
	require.NoError(t, err)
	env.backdateEntry(t, eEntry.ID, month)

	code, inrResp := env.getStatement(t, groupINR.ID, adminINR.ID, "period="+month)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, models.CurrencyINR, inrResp.Currency)
	riNR := memberRow(t, inrResp, memberINR.ID)
	assert.Equal(t, models.CurrencyINR, riNR.Adjustments.Currency)
	assert.Equal(t, models.CurrencyINR, riNR.ClosingBalance.Currency)

	code, eurResp := env.getStatement(t, groupEUR.ID, adminEUR.ID, "period="+month)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, models.CurrencyEUR, eurResp.Currency)
	rEUR := memberRow(t, eurResp, memberEUR.ID)
	assert.Equal(t, models.CurrencyEUR, rEUR.Adjustments.Currency)
	// No cross-group bleed: the EUR group has one member row and INR value never appears.
	assert.Equal(t, int64(2000), rEUR.Adjustments.Value)
}

// --- TestStatement_NonMemberForbidden -----------------------------------

func TestStatement_NonMemberForbidden(t *testing.T) {
	env := setupStatementTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	_, _, group := env.seedGroup(t, "nonmember", models.CurrencyINR)
	outsider, err := env.userRepo.Create(ctx, "outsider-nonmember@example.com", "hash", "Outsider", nil, nil)
	require.NoError(t, err)

	code, _ := env.getStatement(t, group.ID, outsider.ID, "period="+monthsBack(0))
	assert.Equal(t, http.StatusForbidden, code)
}

// --- TestStatement_DefaultPeriodIsCurrentMonth --------------------------

func TestStatement_DefaultPeriodIsCurrentMonth(t *testing.T) {
	env := setupStatementTestEnv(t)
	defer env.cleanup()

	admin, _, group := env.seedGroup(t, "default", models.CurrencyINR)

	code, resp := env.getStatement(t, group.ID, admin.ID, "")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, monthsBack(0), resp.Period, "omitting period defaults to the current server-local month")
}

// --- TestStatement_BadPeriod400 -----------------------------------------

func TestStatement_BadPeriod400(t *testing.T) {
	env := setupStatementTestEnv(t)
	defer env.cleanup()

	admin, _, group := env.seedGroup(t, "badperiod", models.CurrencyINR)

	for _, bad := range []string{"2026-13", "2026/07", "202607", "not-a-date"} {
		path := fmt.Sprintf("/groups/%s/statement?period=%s", group.ID, bad)
		w := doRequest(env.router, http.MethodGet, path, nil, bearerToken(t, admin.ID))
		assert.Equal(t, http.StatusBadRequest, w.Code, "period %q must be rejected", bad)
	}
}

// ptrTime returns a pointer to t (for the decided_at ledger create arg).
func ptrTime(t time.Time) *time.Time { return &t }
