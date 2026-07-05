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
	posted   map[uuid.UUID]map[string]bool // pre-existing posted periods
	headID   uuid.UUID
	inserted map[uuid.UUID]map[string]int // tracks insert counts per (user, period)
}

func newFakeStore(inputs []models.AllowancePostingInput) *fakeStore {
	return &fakeStore{
		inputs:   inputs,
		posted:   make(map[uuid.UUID]map[string]bool),
		headID:   uuid.New(),
		inserted: make(map[uuid.UUID]map[string]int),
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

func (f *fakeStore) GroupHead(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return f.headID, nil
}

func (f *fakeStore) WithTx(_ context.Context, fn func(q db.Querier) error) error {
	return fn(nil) // fake: InsertAllowancePosting ignores the querier
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

// insertCount returns how many times (userID, period) was inserted.
func (f *fakeStore) insertCount(userID uuid.UUID, period string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.inserted[userID][period]
}

// totalInserts returns total insert calls across all users/periods.
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

// ---- PostDue tests ----

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
