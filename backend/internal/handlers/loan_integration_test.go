//go:build integration

package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
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

// loanTestEnv holds wired-up deps for loan handler integration tests.
type loanTestEnv struct {
	router        *gin.Engine
	pool          *pgxpool.Pool
	userRepo      *db.UserRepo
	groupRepo     *db.GroupRepo
	ledgerRepo    *db.LedgerRepo
	allowanceRepo *db.AllowanceRepo
	loanRepo      *db.LoanRepo
	postingSvc    *posting.Service
	cleanup       func()
}

func setupLoanTestEnv(t *testing.T) *loanTestEnv {
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
	ledgerRepo := db.NewLedgerRepo(pool)
	allowanceRepo := db.NewAllowanceRepo(pool)
	loanRepo := db.NewLoanRepo(pool)

	postingSvc := posting.NewService(allowanceRepo, ledgerRepo, loanRepo, groupRepo, pool)

	lh := handlers.NewLedgerHandler(ledgerRepo, groupRepo, nil, postingSvc)
	loansH := handlers.NewLoanHandler(loanRepo, ledgerRepo, groupRepo, pool)

	router := gin.New()
	authMw := auth.AuthMiddleware(testJWTSecret)
	router.Use(authMw)
	router.GET("/groups/:id/balance", lh.GetBalance)
	router.GET("/groups/:id/ledger", lh.ListLedger)
	router.GET("/groups/:id/loans", loansH.ListLoans)
	router.POST("/groups/:id/loans", loansH.CreateLoan)
	router.POST("/loans/:id/approve", loansH.ApproveLoan)
	router.POST("/loans/:id/reject", loansH.RejectLoan)
	router.POST("/loans/:id/close", loansH.CloseLoan)

	return &loanTestEnv{
		router:        router,
		pool:          pool,
		userRepo:      userRepo,
		groupRepo:     groupRepo,
		ledgerRepo:    ledgerRepo,
		allowanceRepo: allowanceRepo,
		loanRepo:      loanRepo,
		postingSvc:    postingSvc,
		cleanup: func() {
			testutil.CleanupTestDB(pool)
			pool.Close()
		},
	}
}

// seedGroup creates head+member users and a group, adding both to group_members.
func (e *loanTestEnv) seedGroup(t *testing.T, suffix string) (head, member *models.User, group *models.Group) {
	t.Helper()
	ctx := t.Context()
	var err error
	head, err = e.userRepo.Create(ctx, fmt.Sprintf("head-%s@example.com", suffix), "hash", "Head", nil, nil)
	require.NoError(t, err)
	member, err = e.userRepo.Create(ctx, fmt.Sprintf("member-%s@example.com", suffix), "hash", "Member", nil, nil)
	require.NoError(t, err)
	group, err = e.groupRepo.Create(ctx, "Family "+suffix, head.ID)
	require.NoError(t, err)
	_, err = e.groupRepo.AddMember(ctx, group.ID, head.ID, models.RoleHead)
	require.NoError(t, err)
	_, err = e.groupRepo.AddMember(ctx, group.ID, member.ID, models.RoleMember)
	require.NoError(t, err)
	return
}

// backdateLoanStart updates a loan's start_period to the given period.
// Used to exercise EMI backfill (start_period drives EMI due periods, not joined_at).
func (e *loanTestEnv) backdateLoanStart(t *testing.T, loanID uuid.UUID, period string) {
	t.Helper()
	_, err := e.pool.Exec(t.Context(),
		`UPDATE loans SET start_period = $1 WHERE id = $2`,
		period, loanID)
	require.NoError(t, err)
}

// backdateJoin sets a member's group_members.joined_at (for allowance leg).
func (e *loanTestEnv) backdateJoin(t *testing.T, groupID, userID uuid.UUID, period string) {
	t.Helper()
	_, err := e.pool.Exec(t.Context(),
		`UPDATE group_members SET joined_at = $1::timestamptz WHERE group_id = $2 AND user_id = $3`,
		period+"-01T00:00:00Z", groupID, userID)
	require.NoError(t, err)
}

// triggerPosting hits GET /balance which fires PostDue.
func (e *loanTestEnv) triggerPosting(t *testing.T, groupID uuid.UUID, callerID uuid.UUID) {
	t.Helper()
	path := fmt.Sprintf("/groups/%s/balance", groupID)
	w := doRequest(e.router, http.MethodGet, path, nil, bearerToken(t, callerID))
	require.Equal(t, http.StatusOK, w.Code)
}

// countEMIRows counts emi ledger entries for a loan.
func (e *loanTestEnv) countEMIRows(t *testing.T, loanID uuid.UUID) int {
	t.Helper()
	var count int
	err := e.pool.QueryRow(t.Context(),
		`SELECT COUNT(*) FROM ledger_entries WHERE loan_id = $1 AND entry_type = 'emi'`,
		loanID).Scan(&count)
	require.NoError(t, err)
	return count
}

// loanStatus returns the current status of a loan.
func (e *loanTestEnv) loanStatus(t *testing.T, loanID uuid.UUID) models.LoanStatus {
	t.Helper()
	var status string
	err := e.pool.QueryRow(t.Context(), `SELECT status FROM loans WHERE id = $1`, loanID).Scan(&status)
	require.NoError(t, err)
	return models.LoanStatus(status)
}

// TestLoan_EMI_IdempotencyDoubleTrigger verifies that triggering PostDue twice
// produces exactly one EMI entry per due period.
func TestLoan_EMI_IdempotencyDoubleTrigger(t *testing.T) {
	env := setupLoanTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group := env.seedGroup(t, "idem")

	// Create a pre-approved active loan then backdate its start_period 2 months back.
	start := time.Now().AddDate(0, 1, 0).Format("2006-01") // next month (default)
	decidedAt := time.Now()
	emiAmt := int64(334)
	loan, err := env.loanRepo.Create(ctx, group.ID, member.ID, 1000, 3, emiAmt,
		models.LoanStatusActive, &start, nil, &head.ID, &decidedAt)
	require.NoError(t, err)

	twoMonthsAgo := time.Now().AddDate(0, -2, 0).Format("2006-01")
	env.backdateLoanStart(t, loan.ID, twoMonthsAgo)

	// Trigger twice.
	env.triggerPosting(t, group.ID, head.ID)
	env.triggerPosting(t, group.ID, head.ID)

	// Exactly 3 EMI rows (twoMonthsAgo, lastMonth, currentMonth).
	assert.Equal(t, 3, env.countEMIRows(t, loan.ID), "exactly 3 EMI rows after double trigger")
}

// TestLoan_EMI_ConcurrentRace verifies exactly-once under concurrent PostDue.
func TestLoan_EMI_ConcurrentRace(t *testing.T) {
	env := setupLoanTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group := env.seedGroup(t, "race")

	start := time.Now().AddDate(0, 1, 0).Format("2006-01")
	decidedAt := time.Now()
	loan, err := env.loanRepo.Create(ctx, group.ID, member.ID, 900, 3, 300,
		models.LoanStatusActive, &start, nil, &head.ID, &decidedAt)
	require.NoError(t, err)

	twoMonthsAgo := time.Now().AddDate(0, -2, 0).Format("2006-01")
	env.backdateLoanStart(t, loan.ID, twoMonthsAgo)

	now := time.Now()
	var wg sync.WaitGroup
	errs := make([]error, 3)
	for i := 0; i < 3; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = env.postingSvc.PostDue(ctx, group.ID, now)
		}()
	}
	wg.Wait()

	for _, e := range errs {
		assert.NoError(t, e, "concurrent PostDue must not error")
	}
	assert.Equal(t, 3, env.countEMIRows(t, loan.ID), "exactly 3 EMI rows despite concurrent triggers")
}

// TestLoan_EMI_UniqueConflict verifies InsertEMIPosting returns false on duplicate.
func TestLoan_EMI_UniqueConflict(t *testing.T) {
	env := setupLoanTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group := env.seedGroup(t, "conflict")

	start := time.Now().AddDate(0, 1, 0).Format("2006-01")
	decidedAt := time.Now()
	loan, err := env.loanRepo.Create(ctx, group.ID, member.ID, 1000, 3, 334,
		models.LoanStatusActive, &start, nil, &head.ID, &decidedAt)
	require.NoError(t, err)

	period := time.Now().Format("2006-01")

	// First insert: should succeed.
	inserted, err := env.ledgerRepo.InsertEMIPosting(ctx, env.pool,
		group.ID, member.ID, loan.ID, 334, period, nil, head.ID)
	require.NoError(t, err)
	assert.True(t, inserted, "first insert must succeed")

	// Second insert: same (group,user,emi,period,loan_id) — must no-op.
	inserted2, err := env.ledgerRepo.InsertEMIPosting(ctx, env.pool,
		group.ID, member.ID, loan.ID, 334, period, nil, head.ID)
	require.NoError(t, err)
	assert.False(t, inserted2, "second insert must be idempotent (ON CONFLICT DO NOTHING)")

	// Exactly one row.
	assert.Equal(t, 1, env.countEMIRows(t, loan.ID))
}

// TestLoan_EMI_AllowanceCoexistence verifies allowance credit and EMI debit coexist
// for the same user in the same period (different entry_type+loan_id on the index).
func TestLoan_EMI_AllowanceCoexistence(t *testing.T) {
	env := setupLoanTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group := env.seedGroup(t, "coexist")

	currentMonth := time.Now().Format("2006-01")

	// Set allowance effective current month; backdate join so it qualifies.
	env.backdateJoin(t, group.ID, member.ID, currentMonth)
	_, err := env.allowanceRepo.SetAllowance(ctx, group.ID, member.ID, 500, currentMonth, head.ID)
	require.NoError(t, err)

	// Create an active loan with start_period = current month.
	decidedAt := time.Now()
	loan, err := env.loanRepo.Create(ctx, group.ID, member.ID, 300, 1, 300,
		models.LoanStatusActive, &currentMonth, nil, &head.ID, &decidedAt)
	require.NoError(t, err)

	// Trigger posting.
	env.triggerPosting(t, group.ID, head.ID)

	// Both an allowance credit and an EMI debit must exist for (member, currentMonth).
	var allowanceCount, emiCount int
	err = env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ledger_entries WHERE group_id=$1 AND user_id=$2 AND entry_type='allowance' AND period=$3`,
		group.ID, member.ID, currentMonth).Scan(&allowanceCount)
	require.NoError(t, err)
	assert.Equal(t, 1, allowanceCount, "allowance credit must exist for current month")

	err = env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ledger_entries WHERE group_id=$1 AND user_id=$2 AND entry_type='emi' AND period=$3`,
		group.ID, member.ID, currentMonth).Scan(&emiCount)
	require.NoError(t, err)
	assert.Equal(t, 1, emiCount, "EMI debit must exist for current month")

	// Balance = allowance - EMI = 500 - 300 = 200.
	balPath := fmt.Sprintf("/groups/%s/balance", group.ID)
	w := doRequest(env.router, http.MethodGet, balPath, nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var balances []handlers.BalanceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &balances))
	var found bool
	for _, b := range balances {
		if b.UserID == member.ID {
			assert.Equal(t, int64(200), b.Balance, "balance = 500 allowance - 300 EMI")
			found = true
		}
	}
	assert.True(t, found, "member must appear in balance")

	_ = loan // referenced above via loan.ID in countEMIRows implicitly
}

// TestLoan_NegativeBalance verifies disbursement posts no ledger entry and
// that balance goes negative when EMIs exceed credits.
func TestLoan_NegativeBalance(t *testing.T) {
	env := setupLoanTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group := env.seedGroup(t, "negbal")

	// Create a loan — disbursement should not produce any ledger entry.
	start := time.Now().AddDate(0, 1, 0).Format("2006-01")
	decidedAt := time.Now()
	loan, err := env.loanRepo.Create(ctx, group.ID, member.ID, 3000, 3, 1000,
		models.LoanStatusActive, &start, nil, &head.ID, &decidedAt)
	require.NoError(t, err)

	// No ledger entries after creation (disbursement is not a ledger event).
	var totalRows int
	err = env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ledger_entries WHERE group_id=$1 AND user_id=$2`,
		group.ID, member.ID).Scan(&totalRows)
	require.NoError(t, err)
	assert.Equal(t, 0, totalRows, "disbursement must not create any ledger entry")

	// Backdate start_period 2 months back → 3 EMIs due.
	twoMonthsAgo := time.Now().AddDate(0, -2, 0).Format("2006-01")
	env.backdateLoanStart(t, loan.ID, twoMonthsAgo)

	env.triggerPosting(t, group.ID, head.ID)

	// 3 EMI debits posted (no credits), balance = -3000 (negative).
	balPath := fmt.Sprintf("/groups/%s/balance", group.ID)
	w := doRequest(env.router, http.MethodGet, balPath, nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var balances []handlers.BalanceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &balances))
	var found bool
	for _, b := range balances {
		if b.UserID == member.ID {
			assert.Equal(t, int64(-3000), b.Balance, "balance must be negative when EMIs exceed credits")
			found = true
		}
	}
	assert.True(t, found, "member must appear in balance")
}

// TestLoan_FinalInstallmentAndAutoClose verifies 1000/3 → [334,334,332] and auto-close.
func TestLoan_FinalInstallmentAndAutoClose(t *testing.T) {
	env := setupLoanTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group := env.seedGroup(t, "autoclose")

	// Create a 3-installment loan.
	start := time.Now().AddDate(0, 1, 0).Format("2006-01")
	decidedAt := time.Now()
	loan, err := env.loanRepo.Create(ctx, group.ID, member.ID, 1000, 3, 334,
		models.LoanStatusActive, &start, nil, &head.ID, &decidedAt)
	require.NoError(t, err)

	// Backdate start_period 2 months back (so 3 months are due: -2, -1, current).
	twoMonthsAgo := time.Now().AddDate(0, -2, 0).Format("2006-01")
	env.backdateLoanStart(t, loan.ID, twoMonthsAgo)

	env.triggerPosting(t, group.ID, head.ID)

	// Verify 3 EMI rows.
	assert.Equal(t, 3, env.countEMIRows(t, loan.ID))

	// Verify amounts: first 2 = 334, last = 332.
	rows, err := env.pool.Query(ctx,
		`SELECT amount FROM ledger_entries WHERE loan_id = $1 AND entry_type = 'emi' ORDER BY period ASC`,
		loan.ID)
	require.NoError(t, err)
	defer rows.Close()
	var amounts []int64
	for rows.Next() {
		var amt int64
		require.NoError(t, rows.Scan(&amt))
		amounts = append(amounts, amt)
	}
	assert.Equal(t, []int64{334, 334, 332}, amounts)

	var total int64
	for _, a := range amounts {
		total += a
	}
	assert.Equal(t, int64(1000), total, "sum must equal principal")

	// Loan must be auto-closed.
	assert.Equal(t, models.LoanStatusClosed, env.loanStatus(t, loan.ID))

	// GET /groups/:id/loans → installments_posted=3, outstanding=0.
	loansPath := fmt.Sprintf("/groups/%s/loans", group.ID)
	w := doRequest(env.router, http.MethodGet, loansPath, nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var loanResp []handlers.LoanResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &loanResp))
	require.Len(t, loanResp, 1)
	assert.Equal(t, 3, loanResp[0].InstallmentsPosted)
	assert.Equal(t, int64(0), loanResp[0].Outstanding)
	assert.Equal(t, models.LoanStatusClosed, loanResp[0].Status)
}

// TestLoan_EarlyPayoff verifies the close endpoint posts the exact outstanding
// and closes the loan.
func TestLoan_EarlyPayoff(t *testing.T) {
	env := setupLoanTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group := env.seedGroup(t, "payoff")

	// 1000/3 loan; post 1 installment via posting engine, then close early.
	start := time.Now().AddDate(0, 1, 0).Format("2006-01")
	decidedAt := time.Now()
	loan, err := env.loanRepo.Create(ctx, group.ID, member.ID, 1000, 3, 334,
		models.LoanStatusActive, &start, nil, &head.ID, &decidedAt)
	require.NoError(t, err)

	// Set start_period to the current month → exactly 1 installment due (the next
	// installment falls in the following month, which is not yet due).
	currentMonth := time.Now().Format("2006-01")
	env.backdateLoanStart(t, loan.ID, currentMonth)
	env.triggerPosting(t, group.ID, head.ID)
	assert.Equal(t, 1, env.countEMIRows(t, loan.ID))

	// Close early → one final EMI for outstanding = 1000 - 334 = 666.
	closePath := fmt.Sprintf("/loans/%s/close", loan.ID)
	w := doRequest(env.router, http.MethodPost, closePath, nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)

	var resp handlers.LoanResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, models.LoanStatusClosed, resp.Status)
	assert.Equal(t, int64(0), resp.Outstanding)
	assert.Equal(t, 2, resp.InstallmentsPosted)

	// Σ of all EMIs must equal principal.
	var total int64
	err = env.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0)::bigint FROM ledger_entries WHERE loan_id=$1 AND entry_type='emi'`,
		loan.ID).Scan(&total)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), total, "sum of all EMIs must equal principal")

	// Second close → 409.
	w2 := doRequest(env.router, http.MethodPost, closePath, nil, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusConflict, w2.Code)
}

// TestLoan_LifecycleGuards verifies 409 for invalid lifecycle transitions.
func TestLoan_LifecycleGuards(t *testing.T) {
	env := setupLoanTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group := env.seedGroup(t, "guards")

	// Create a requested loan.
	start := time.Now().AddDate(0, 1, 0).Format("2006-01")
	decidedAt := time.Now()
	activeLoan, err := env.loanRepo.Create(ctx, group.ID, member.ID, 500, 2, 250,
		models.LoanStatusActive, &start, nil, &head.ID, &decidedAt)
	require.NoError(t, err)

	reqLoan, err := env.loanRepo.Create(ctx, group.ID, member.ID, 200, 2, 100,
		models.LoanStatusRequested, nil, nil, nil, nil)
	require.NoError(t, err)

	// Approve an already-active loan → 409.
	w := doRequest(env.router, http.MethodPost,
		fmt.Sprintf("/loans/%s/approve", activeLoan.ID), nil, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusConflict, w.Code)

	// Reject an already-active loan → 409.
	w = doRequest(env.router, http.MethodPost,
		fmt.Sprintf("/loans/%s/reject", activeLoan.ID), nil, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusConflict, w.Code)

	// Close a requested (not active) loan → 409.
	w = doRequest(env.router, http.MethodPost,
		fmt.Sprintf("/loans/%s/close", reqLoan.ID), nil, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusConflict, w.Code)
}

// TestLoan_AuthZ verifies the authorization matrix.
func TestLoan_AuthZ(t *testing.T) {
	env := setupLoanTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group := env.seedGroup(t, "authz")
	extra, err := env.userRepo.Create(ctx, "extra-loan@example.com", "hash", "Extra", nil, nil)
	require.NoError(t, err)
	_, err = env.groupRepo.AddMember(ctx, group.ID, extra.ID, models.RoleMember)
	require.NoError(t, err)

	loansPath := fmt.Sprintf("/groups/%s/loans", group.ID)

	// Member POST naming another user → 400.
	w := doRequest(env.router, http.MethodPost, loansPath, map[string]interface{}{
		"user_id": extra.ID, "principal": 1000, "installments": 3,
	}, bearerToken(t, member.ID))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Member creates their own loan → 201.
	w = doRequest(env.router, http.MethodPost, loansPath, map[string]interface{}{
		"principal": 1000, "installments": 3,
	}, bearerToken(t, member.ID))
	require.Equal(t, http.StatusCreated, w.Code)
	var loanResp handlers.LoanResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &loanResp))
	loanID := loanResp.ID
	assert.Equal(t, models.LoanStatusRequested, loanResp.Status)

	// Member approve → 403.
	w = doRequest(env.router, http.MethodPost,
		fmt.Sprintf("/loans/%s/approve", loanID), nil, bearerToken(t, member.ID))
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Member reject → 403.
	w = doRequest(env.router, http.MethodPost,
		fmt.Sprintf("/loans/%s/reject", loanID), nil, bearerToken(t, member.ID))
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Head GET → sees all loans.
	w = doRequest(env.router, http.MethodGet, loansPath, nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var headLoans []handlers.LoanResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &headLoans))
	assert.GreaterOrEqual(t, len(headLoans), 1)

	// Member GET → only own loans.
	w = doRequest(env.router, http.MethodGet, loansPath, nil, bearerToken(t, member.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var memberLoans []handlers.LoanResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &memberLoans))
	for _, l := range memberLoans {
		assert.Equal(t, member.ID, l.UserID, "member must only see own loans")
	}

	// Head POST targeting head as borrower → 400.
	w = doRequest(env.router, http.MethodPost, loansPath, map[string]interface{}{
		"user_id": head.ID, "principal": 500, "installments": 2,
	}, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Head creates pre-approved active loan for member.
	w = doRequest(env.router, http.MethodPost, loansPath, map[string]interface{}{
		"user_id": member.ID, "principal": 600, "installments": 2,
	}, bearerToken(t, head.ID))
	require.Equal(t, http.StatusCreated, w.Code)
	var activeResp handlers.LoanResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &activeResp))
	assert.Equal(t, models.LoanStatusActive, activeResp.Status)
	assert.NotNil(t, activeResp.StartPeriod)

	// Non-member close → 403.
	nonMember, err := env.userRepo.Create(ctx, "outsider-loan@example.com", "hash", "Out", nil, nil)
	require.NoError(t, err)
	w = doRequest(env.router, http.MethodPost,
		fmt.Sprintf("/loans/%s/close", activeResp.ID), nil, bearerToken(t, nonMember.ID))
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Head approve the member's requested loan.
	w = doRequest(env.router, http.MethodPost,
		fmt.Sprintf("/loans/%s/approve", loanID), nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var approvedResp handlers.LoanResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &approvedResp))
	assert.Equal(t, models.LoanStatusActive, approvedResp.Status)
	assert.NotNil(t, approvedResp.StartPeriod)
}

// TestLoan_Migration011_UpDown verifies migration 011 up/down and FK behavior.
// This test runs after the env is already migrated (loans table exists).
func TestLoan_Migration011_UpDown(t *testing.T) {
	env := setupLoanTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group := env.seedGroup(t, "mig011")

	// Verify the loans table and FK are live by creating a loan and an EMI entry.
	start := time.Now().AddDate(0, 1, 0).Format("2006-01")
	decidedAt := time.Now()
	loan, err := env.loanRepo.Create(ctx, group.ID, member.ID, 500, 1, 500,
		models.LoanStatusActive, &start, nil, &head.ID, &decidedAt)
	require.NoError(t, err)

	period := time.Now().Format("2006-01")
	inserted, err := env.ledgerRepo.InsertEMIPosting(ctx, env.pool,
		group.ID, member.ID, loan.ID, 500, period, nil, head.ID)
	require.NoError(t, err)
	assert.True(t, inserted, "EMI row must insert against the loans FK")

	// Bogus loan_id must fail the FK constraint.
	bogusID := uuid.New()
	_, fkErr := env.pool.Exec(ctx, `
		INSERT INTO ledger_entries
			(id, group_id, user_id, amount, status, entry_type, direction,
			 loan_id, period, created_by_user_id)
		VALUES (gen_random_uuid(), $1, $2, 500, 'approved', 'emi', 'debit', $3, $4, $5)`,
		group.ID, member.ID, bogusID, period+"X", head.ID)
	assert.Error(t, fkErr, "bogus loan_id must fail FK constraint")

	// RunMigrationsDown rolls back the ENTIRE chain, not just 011. An earlier
	// down step (009) restores ledger_entries.chore_id NOT NULL, which the emi row
	// seeded above (chore_id NULL) would violate — down migrations are schema
	// rollbacks, not data-safe (§3.2). Clear the ledger rows before rolling back so
	// the full down/up round-trip exercises 011's down cleanly.
	_, err = env.pool.Exec(ctx, `DELETE FROM ledger_entries`)
	require.NoError(t, err)

	// Run migrations down and re-up.
	dbURL := testutil.GetTestDatabaseURL()
	require.NoError(t, db.RunMigrationsDown(dbURL))
	require.NoError(t, db.RunMigrations(dbURL))

	// loans table must exist after re-up.
	var exists bool
	err = env.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT FROM information_schema.tables
		               WHERE table_schema = 'public' AND table_name = 'loans')`,
	).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "loans table must exist after migration re-up")

	// loan_status type must exist.
	var typeExists bool
	err = env.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'loan_status')`).Scan(&typeExists)
	require.NoError(t, err)
	assert.True(t, typeExists, "loan_status type must exist after migration re-up")
}

// TestLoan_ApproveWithOverrides verifies that approve can override principal/installments.
func TestLoan_ApproveWithOverrides(t *testing.T) {
	env := setupLoanTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group := env.seedGroup(t, "override")

	// Member requests 1000/3.
	loansPath := fmt.Sprintf("/groups/%s/loans", group.ID)
	w := doRequest(env.router, http.MethodPost, loansPath, map[string]interface{}{
		"principal": 1000, "installments": 3,
	}, bearerToken(t, member.ID))
	require.Equal(t, http.StatusCreated, w.Code)
	var loanResp handlers.LoanResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &loanResp))

	// Head approves with different terms: 1200/4.
	w = doRequest(env.router, http.MethodPost,
		fmt.Sprintf("/loans/%s/approve", loanResp.ID),
		map[string]interface{}{"principal": 1200, "installments": 4},
		bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var approved handlers.LoanResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &approved))
	assert.Equal(t, int64(1200), approved.Principal)
	assert.Equal(t, 4, approved.Installments)
	assert.Equal(t, int64(300), approved.EMIAmount) // ceil(1200/4) = 300
	assert.Equal(t, models.LoanStatusActive, approved.Status)

	_ = ctx
	_ = member
}
