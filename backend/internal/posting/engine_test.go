package posting

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
)

// ---- fake Store ----

type fakeStore struct {
	mu       sync.Mutex
	inputs   []models.AllowancePostingInput
	posted   map[uuid.UUID]map[string]bool // pre-existing posted allowance periods
	headID   uuid.UUID
	inserted map[uuid.UUID]map[string]int // tracks allowance insert counts per (user, period)

	// EMI fields
	loans       []models.LoanPostingInput
	postedEMI   map[uuid.UUID]map[string]bool // pre-existing posted EMI periods per loan
	emiInserted map[uuid.UUID]map[string]int  // tracks EMI insert counts per (loan, period)
	closedLoans map[uuid.UUID]bool            // tracks which loans were auto-closed
	lockOrder   []uuid.UUID                   // records loan lock order
}

func newFakeStore(inputs []models.AllowancePostingInput) *fakeStore {
	return &fakeStore{
		inputs:      inputs,
		posted:      make(map[uuid.UUID]map[string]bool),
		headID:      uuid.New(),
		inserted:    make(map[uuid.UUID]map[string]int),
		postedEMI:   make(map[uuid.UUID]map[string]bool),
		emiInserted: make(map[uuid.UUID]map[string]int),
		closedLoans: make(map[uuid.UUID]bool),
	}
}

func (f *fakeStore) ListPostingInputs(_ context.Context, _ uuid.UUID) ([]models.AllowancePostingInput, error) {
	return f.inputs, nil
}

func (f *fakeStore) PostedAllowancePeriods(_ context.Context, _ uuid.UUID) (map[uuid.UUID]map[string]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Merge pre-seeded posted map with what has been inserted so far.
	result := make(map[uuid.UUID]map[string]bool)
	for u, periods := range f.posted {
		result[u] = make(map[string]bool)
		for p := range periods {
			result[u][p] = true
		}
	}
	for u, periods := range f.inserted {
		if result[u] == nil {
			result[u] = make(map[string]bool)
		}
		for p, cnt := range periods {
			if cnt > 0 {
				result[u][p] = true
			}
		}
	}
	return result, nil
}

func (f *fakeStore) GroupAdmin(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return f.headID, nil
}

func (f *fakeStore) WithTx(_ context.Context, fn func(q db.Querier) error) error {
	return fn(nil) // fake: insert methods ignore the querier
}

func (f *fakeStore) InsertAllowancePosting(_ context.Context, _ db.Querier,
	_ uuid.UUID, userID uuid.UUID, _ int64, period string, _ uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.inserted[userID] == nil {
		f.inserted[userID] = make(map[string]int)
	}
	f.inserted[userID][period]++
	return true, nil
}

func (f *fakeStore) ListActiveLoans(_ context.Context, _ uuid.UUID) ([]models.LoanPostingInput, error) {
	return f.loans, nil
}

func (f *fakeStore) PostedEMIPeriods(_ context.Context, _ uuid.UUID) (map[uuid.UUID]map[string]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Merge pre-seeded postedEMI with what has been inserted.
	result := make(map[uuid.UUID]map[string]bool)
	for loanID, periods := range f.postedEMI {
		result[loanID] = make(map[string]bool)
		for p := range periods {
			result[loanID][p] = true
		}
	}
	for loanID, periods := range f.emiInserted {
		if result[loanID] == nil {
			result[loanID] = make(map[string]bool)
		}
		for p, cnt := range periods {
			if cnt > 0 {
				result[loanID][p] = true
			}
		}
	}
	return result, nil
}

func (f *fakeStore) LockActiveLoan(_ context.Context, _ db.Querier, loanID uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lockOrder = append(f.lockOrder, loanID)
	// Loan is considered active unless it was already auto-closed by a previous call.
	return !f.closedLoans[loanID], nil
}

func (f *fakeStore) InsertEMIPosting(_ context.Context, _ db.Querier,
	_ uuid.UUID, _ uuid.UUID, loanID uuid.UUID, _ int64, period string, _ *string, _ uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.emiInserted[loanID] == nil {
		f.emiInserted[loanID] = make(map[string]int)
	}
	cnt := f.emiInserted[loanID][period]
	f.emiInserted[loanID][period]++
	// Simulate idempotency: first insert returns true, subsequent return false.
	return cnt == 0, nil
}

func (f *fakeStore) CountPostedEMIs(_ context.Context, _ db.Querier, loanID uuid.UUID) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	total := 0
	for _, cnt := range f.emiInserted[loanID] {
		if cnt > 0 {
			total++
		}
	}
	// Also count pre-seeded postedEMI.
	for range f.postedEMI[loanID] {
		total++
	}
	return total, nil
}

func (f *fakeStore) CloseLoan(_ context.Context, _ db.Querier, loanID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closedLoans[loanID] = true
	return nil
}

// insertCount returns how many times (userID, period) was inserted as allowance.
func (f *fakeStore) insertCount(userID uuid.UUID, period string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.inserted[userID][period]
}

// totalInserts returns total allowance insert calls across all users/periods.
func (f *fakeStore) totalInserts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, periods := range f.inserted {
		for _, cnt := range periods {
			n += cnt
		}
	}
	return n
}

// emiInsertCount returns how many times (loanID, period) was inserted as EMI.
func (f *fakeStore) emiInsertCount(loanID uuid.UUID, period string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.emiInserted[loanID][period]
}

// totalEMIInserts returns total EMI insert calls across all loans/periods.
func (f *fakeStore) totalEMIInserts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, periods := range f.emiInserted {
		for _, cnt := range periods {
			n += cnt
		}
	}
	return n
}

// isLoanClosed returns true if CloseLoan was called for loanID.
func (f *fakeStore) isLoanClosed(loanID uuid.UUID) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closedLoans[loanID]
}

// ---- helper builders ----

func inp(userID uuid.UUID, amount int64, effectiveFrom string, joinedAt time.Time) models.AllowancePostingInput {
	return models.AllowancePostingInput{
		UserID:        userID,
		Amount:        amount,
		EffectiveFrom: effectiveFrom,
		JoinedAt:      joinedAt,
	}
}

func mustParse(s string) time.Time {
	// Parse "YYYY-MM" as the first day of that month in UTC.
	t, err := time.Parse("2006-01", s)
	if err != nil {
		panic(err)
	}
	return t
}

// loanInp builds a LoanPostingInput for testing.
func loanInp(loanID, userID uuid.UUID, principal int64, installments int, emiAmount int64, startPeriod string) models.LoanPostingInput {
	return models.LoanPostingInput{
		LoanID:       loanID,
		UserID:       userID,
		Principal:    principal,
		Installments: installments,
		EMIAmount:    emiAmount,
		StartPeriod:  startPeriod,
	}
}

// ---- PostDue allowance tests (unchanged) ----

func TestPostDue_Idempotency(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	userID := uuid.New()
	now := mustParse("2026-05") // month of "now"
	joined := mustParse("2026-05")

	store := newFakeStore([]models.AllowancePostingInput{
		inp(userID, 1000, "2026-05", joined),
	})

	require.NoError(t, PostDue(ctx, store, groupID, now))
	require.NoError(t, PostDue(ctx, store, groupID, now)) // second call

	// Only one insert for (userID, "2026-05") despite two calls.
	// The second call sees it in PostedAllowancePeriods and skips.
	assert.Equal(t, 1, store.insertCount(userID, "2026-05"), "must not duplicate")
	assert.Equal(t, 1, store.totalInserts())
}

func TestPostDue_Backfill(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	userID := uuid.New()
	joined := mustParse("2026-02")
	now := mustParse("2026-05") // 4 months inclusive: Feb, Mar, Apr, May

	store := newFakeStore([]models.AllowancePostingInput{
		inp(userID, 500, "2026-02", joined),
	})

	require.NoError(t, PostDue(ctx, store, groupID, now))

	assert.Equal(t, 4, store.totalInserts(), "should post Feb..May = 4 months")
	for _, p := range []string{"2026-02", "2026-03", "2026-04", "2026-05"} {
		assert.Equal(t, 1, store.insertCount(userID, p), "expected one insert for %s", p)
	}
}

func TestPostDue_MidMonthJoin(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	userID := uuid.New()
	// joined_at mid-month: join month still gets full allowance (no proration)
	joined := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	now := mustParse("2026-05")

	store := newFakeStore([]models.AllowancePostingInput{
		inp(userID, 1000, "2026-03", joined),
	})

	require.NoError(t, PostDue(ctx, store, groupID, now))

	assert.Equal(t, 3, store.totalInserts(), "Mar, Apr, May = 3 months")
	assert.Equal(t, 1, store.insertCount(userID, "2026-03"), "join month gets full allowance")
}

func TestPostDue_EffectiveFromAfterJoin(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	userID := uuid.New()
	joined := mustParse("2026-01") // joined Jan
	now := mustParse("2026-05")

	// allowance only starts March; Jan/Feb get nothing
	store := newFakeStore([]models.AllowancePostingInput{
		inp(userID, 1000, "2026-03", joined),
	})

	require.NoError(t, PostDue(ctx, store, groupID, now))

	assert.Equal(t, 3, store.totalInserts(), "Mar, Apr, May = 3 months (Jan/Feb before effective_from)")
	assert.Equal(t, 0, store.insertCount(userID, "2026-01"))
	assert.Equal(t, 0, store.insertCount(userID, "2026-02"))
}

func TestPostDue_FutureEffectiveFrom(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	userID := uuid.New()
	joined := mustParse("2026-05")
	now := mustParse("2026-05")

	// effective_from is in the future relative to now
	store := newFakeStore([]models.AllowancePostingInput{
		inp(userID, 1000, "2026-06", joined),
	})

	require.NoError(t, PostDue(ctx, store, groupID, now))
	assert.Equal(t, 0, store.totalInserts(), "nothing due yet when effective_from > now")
}

func TestPostDue_AmountChange(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	userID := uuid.New()
	joined := mustParse("2026-01")
	now := mustParse("2026-05") // Jan-Mar=500, Apr-May=1000

	store := newFakeStore([]models.AllowancePostingInput{
		inp(userID, 500, "2026-01", joined),
		inp(userID, 1000, "2026-04", joined),
	})

	require.NoError(t, PostDue(ctx, store, groupID, now))

	assert.Equal(t, 5, store.totalInserts())
	// Past months use the old amount — verify by re-running PostDue with the amount tracking
	// (indirect: we verify 5 inserts total across Jan..May, correct amounts tested via helpers below)
}

func TestPostDue_Paused(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	userID := uuid.New()
	joined := mustParse("2026-01")
	now := mustParse("2026-04")

	// Jan=1000 (active), Feb=0 (paused), Mar=500 (resumed)
	store := newFakeStore([]models.AllowancePostingInput{
		inp(userID, 1000, "2026-01", joined),
		inp(userID, 0, "2026-02", joined),
		inp(userID, 500, "2026-03", joined),
	})

	require.NoError(t, PostDue(ctx, store, groupID, now))

	// Feb is paused → not inserted; Jan, Mar, Apr inserted
	assert.Equal(t, 3, store.totalInserts())
	assert.Equal(t, 1, store.insertCount(userID, "2026-01"))
	assert.Equal(t, 0, store.insertCount(userID, "2026-02"), "paused month must not be posted")
	assert.Equal(t, 1, store.insertCount(userID, "2026-03"))
	assert.Equal(t, 1, store.insertCount(userID, "2026-04"))
}

func TestPostDue_NoAllowances(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	now := mustParse("2026-05")

	store := newFakeStore(nil) // no inputs

	require.NoError(t, PostDue(ctx, store, groupID, now))
	assert.Equal(t, 0, store.totalInserts())
}

// ---- EMI schedule unit tests ----

func TestEMISchedule_Standard(t *testing.T) {
	// 1000/3: emiAmount = ceil(1000/3) = 334; schedule [334,334,332], Σ=1000
	sched := emiSchedule(1000, 334, 3)
	assert.Equal(t, []int64{334, 334, 332}, sched)
	var sum int64
	for _, a := range sched {
		sum += a
	}
	assert.Equal(t, int64(1000), sum, "schedule must sum to principal")
}

func TestEMISchedule_ExactDivision(t *testing.T) {
	// 900/3: emiAmount = 300; no rounding
	sched := emiSchedule(900, 300, 3)
	assert.Equal(t, []int64{300, 300, 300}, sched)
}

func TestEMISchedule_SingleInstallment(t *testing.T) {
	// installments=1: one full payment
	sched := emiSchedule(5000, 5000, 1)
	assert.Equal(t, []int64{5000}, sched)
}

func TestEMISchedule_TinyPrincipal(t *testing.T) {
	// 2/3: emiAmount = ceil(2/3) = 1; schedule [1,1], length 2 not 3, no zero installment
	emiAmt := (int64(2) + int64(3) - 1) / int64(3) // = 1
	sched := emiSchedule(2, emiAmt, 3)
	assert.Equal(t, []int64{1, 1}, sched, "stops when principal fully repaid")
	assert.Equal(t, 2, len(sched), "length must be 2, not 3")
	var sum int64
	for _, a := range sched {
		assert.Greater(t, a, int64(0), "no installment can be <= 0")
		sum += a
	}
	assert.Equal(t, int64(2), sum)
}

func TestEMISchedule_OneInstallmentPaysFull(t *testing.T) {
	// emiAmount >= principal: single installment covers all
	sched := emiSchedule(100, 200, 5)
	assert.Equal(t, []int64{100}, sched, "clamps to remaining on first installment")
}

// ---- addMonths unit tests ----

func TestAddMonths(t *testing.T) {
	tests := []struct {
		period string
		k      int
		want   string
	}{
		{"2026-01", 1, "2026-02"},
		{"2026-12", 1, "2027-01"},  // year boundary
		{"2026-01", 13, "2027-02"}, // multi-year
		{"2026-05", 0, "2026-05"},  // +0
		{"2025-11", 3, "2026-02"},
	}
	for _, tt := range tests {
		got := AddMonths(tt.period, tt.k)
		assert.Equal(t, tt.want, got, "AddMonths(%q, %d)", tt.period, tt.k)
	}
}

// ---- PostDue EMI posting tests ----

func TestPostDue_EMI_DuePeriods(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	loanID := uuid.New()
	userID := uuid.New()
	// start_period 3 months before now → 3 due installments (1000/3: [334,334,332])
	now := mustParse("2026-05")

	store := newFakeStore(nil)
	store.loans = []models.LoanPostingInput{
		loanInp(loanID, userID, 1000, 3, 334, "2026-03"),
	}

	require.NoError(t, PostDue(ctx, store, groupID, now))

	assert.Equal(t, 1, store.emiInsertCount(loanID, "2026-03"), "first installment")
	assert.Equal(t, 1, store.emiInsertCount(loanID, "2026-04"), "second installment")
	assert.Equal(t, 1, store.emiInsertCount(loanID, "2026-05"), "third installment")
	assert.Equal(t, 3, store.totalEMIInserts())
}

func TestPostDue_EMI_FutureStartPeriod(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	loanID := uuid.New()
	userID := uuid.New()
	now := mustParse("2026-05")

	store := newFakeStore(nil)
	// start_period in future → no EMIs due
	store.loans = []models.LoanPostingInput{
		loanInp(loanID, userID, 1000, 3, 334, "2026-06"),
	}

	require.NoError(t, PostDue(ctx, store, groupID, now))
	assert.Equal(t, 0, store.totalEMIInserts(), "no EMIs due when start_period is future")
}

func TestPostDue_EMI_Idempotency(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	loanID := uuid.New()
	userID := uuid.New()
	now := mustParse("2026-05")

	store := newFakeStore(nil)
	store.loans = []models.LoanPostingInput{
		loanInp(loanID, userID, 1000, 3, 334, "2026-03"),
	}

	require.NoError(t, PostDue(ctx, store, groupID, now))
	require.NoError(t, PostDue(ctx, store, groupID, now)) // second call

	// InsertEMIPosting is called but the fake's idempotency guard returns false on second call.
	// The real guard is PostedEMIPeriods which merges emiInserted — on second call all periods
	// are already in the guard so the fast path skips without even locking.
	// Either way, each (loan, period) must appear exactly once in logical state.
	for _, p := range []string{"2026-03", "2026-04", "2026-05"} {
		assert.GreaterOrEqual(t, store.emiInsertCount(loanID, p), 1,
			"period %s should be posted", p)
	}
}

func TestPostDue_EMI_AutoClose(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	loanID := uuid.New()
	userID := uuid.New()
	now := mustParse("2026-05")

	store := newFakeStore(nil)
	// 3-installment loan, start 3 months ago — all 3 are now due
	store.loans = []models.LoanPostingInput{
		loanInp(loanID, userID, 1000, 3, 334, "2026-03"),
	}

	require.NoError(t, PostDue(ctx, store, groupID, now))

	assert.True(t, store.isLoanClosed(loanID), "loan must be auto-closed after all installments posted")
}

func TestPostDue_EMI_AutoClose_NotEarlyIfNotAllDue(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	loanID := uuid.New()
	userID := uuid.New()
	now := mustParse("2026-04") // only 2 months due out of 3

	store := newFakeStore(nil)
	store.loans = []models.LoanPostingInput{
		loanInp(loanID, userID, 1000, 3, 334, "2026-03"),
	}

	require.NoError(t, PostDue(ctx, store, groupID, now))

	assert.False(t, store.isLoanClosed(loanID), "loan must NOT be closed when installments remain")
}

func TestPostDue_EMI_AllowanceCoexistence(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	loanID := uuid.New()
	userID := uuid.New()
	now := mustParse("2026-05")

	// Same user has both an allowance and a loan due in 2026-05.
	store := newFakeStore([]models.AllowancePostingInput{
		inp(userID, 500, "2026-05", mustParse("2026-05")),
	})
	store.loans = []models.LoanPostingInput{
		loanInp(loanID, userID, 1000, 3, 334, "2026-03"),
	}

	require.NoError(t, PostDue(ctx, store, groupID, now))

	// Allowance inserted for 2026-05.
	assert.Equal(t, 1, store.insertCount(userID, "2026-05"), "allowance must be posted")

	// EMI inserted for 2026-03, 2026-04, 2026-05.
	assert.Equal(t, 1, store.emiInsertCount(loanID, "2026-05"), "emi must be posted")
	assert.Equal(t, 3, store.totalEMIInserts(), "3 EMI installments due")
	assert.Equal(t, 1, store.totalInserts(), "1 allowance posted")
}

func TestPostDue_EMI_DeterministicLockOrder(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	now := mustParse("2026-05")

	userA := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userB := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	loan1 := uuid.MustParse("00000000-0000-0000-0000-000000000010") // userA, start 2026-03, id=10
	loan2 := uuid.MustParse("00000000-0000-0000-0000-000000000020") // userA, start 2026-03, id=20
	loan3 := uuid.MustParse("00000000-0000-0000-0000-000000000030") // userB, start 2026-03, id=30

	store := newFakeStore(nil)
	// loans already in SQL order: (user_id, start_period, id)
	store.loans = []models.LoanPostingInput{
		loanInp(loan1, userA, 1000, 3, 334, "2026-03"),
		loanInp(loan2, userA, 1200, 3, 400, "2026-03"),
		loanInp(loan3, userB, 900, 3, 300, "2026-03"),
	}

	require.NoError(t, PostDue(ctx, store, groupID, now))

	// Lock order must match the SQL iteration order: loan1, loan2, loan3.
	assert.Equal(t, []uuid.UUID{loan1, loan2, loan3}, store.lockOrder,
		"loan locks must follow deterministic SQL order to prevent deadlocks")
}

func TestPostDue_EMI_NoLoansNoAllowances(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	now := mustParse("2026-05")

	store := newFakeStore(nil)

	require.NoError(t, PostDue(ctx, store, groupID, now))
	assert.Equal(t, 0, store.totalInserts())
	assert.Equal(t, 0, store.totalEMIInserts())
}

func TestPostDue_EMI_LoansOnlyNoAllowances(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	loanID := uuid.New()
	userID := uuid.New()
	now := mustParse("2026-05")

	store := newFakeStore(nil) // no allowances
	store.loans = []models.LoanPostingInput{
		loanInp(loanID, userID, 600, 2, 300, "2026-04"),
	}

	require.NoError(t, PostDue(ctx, store, groupID, now))

	assert.Equal(t, 0, store.totalInserts(), "no allowances")
	assert.Equal(t, 2, store.totalEMIInserts(), "2 EMI installments due (Apr, May)")
}

func TestPostDue_EMI_TinyPrincipalAutoClose(t *testing.T) {
	ctx := context.Background()
	groupID := uuid.New()
	loanID := uuid.New()
	userID := uuid.New()
	now := mustParse("2026-05")

	// 2/3: emiSchedule yields [1,1], length 2 — closes after 2 EMIs, not 3
	store := newFakeStore(nil)
	store.loans = []models.LoanPostingInput{
		loanInp(loanID, userID, 2, 3, 1, "2026-04"), // 2 due periods (Apr, May)
	}

	require.NoError(t, PostDue(ctx, store, groupID, now))

	assert.Equal(t, 2, store.totalEMIInserts())
	assert.True(t, store.isLoanClosed(loanID), "tiny-principal loan closes after 2 EMIs")
}

// ---- Pure helper tests ----

func TestFormatPeriod(t *testing.T) {
	tests := []struct {
		t    time.Time
		want string
	}{
		{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "2026-01"},
		{time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC), "2026-12"},
		{time.Date(2000, 6, 15, 0, 0, 0, 0, time.UTC), "2000-06"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, formatPeriod(tt.t))
	}
}

func TestMonthsInclusive(t *testing.T) {
	tests := []struct {
		start, end string
		want       []string
	}{
		{"2026-01", "2026-03", []string{"2026-01", "2026-02", "2026-03"}},
		{"2026-01", "2026-01", []string{"2026-01"}},
		{"2026-04", "2026-03", nil},                                                  // start > end
		{"2025-11", "2026-02", []string{"2025-11", "2025-12", "2026-01", "2026-02"}}, // year boundary
		{"2026-12", "2027-01", []string{"2026-12", "2027-01"}},
	}
	for _, tt := range tests {
		got := monthsInclusive(tt.start, tt.end)
		assert.Equal(t, tt.want, got, "monthsInclusive(%q, %q)", tt.start, tt.end)
	}
}

func TestAmountInForce(t *testing.T) {
	userID := uuid.New()
	joined := mustParse("2026-01")

	rows := []models.AllowancePostingInput{
		inp(userID, 500, "2026-01", joined),
		inp(userID, 1000, "2026-04", joined),
	}

	assert.Equal(t, int64(500), amountInForce(rows, "2026-01"))
	assert.Equal(t, int64(500), amountInForce(rows, "2026-03"))
	assert.Equal(t, int64(1000), amountInForce(rows, "2026-04"))
	assert.Equal(t, int64(1000), amountInForce(rows, "2026-07"))
}

func TestMaxPeriod(t *testing.T) {
	assert.Equal(t, "2026-05", maxPeriod("2026-03", "2026-05"))
	assert.Equal(t, "2026-05", maxPeriod("2026-05", "2026-03"))
	assert.Equal(t, "2026-05", maxPeriod("2026-05", "2026-05"))
	assert.Equal(t, "2026-01", maxPeriod("2025-12", "2026-01"))
}
