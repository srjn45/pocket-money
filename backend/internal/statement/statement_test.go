package statement

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/srjn45/pocket-money/backend/internal/models"
)

// --- test builders -------------------------------------------------------

// sysEntry builds an approved system entry (allowance/emi) with an explicit
// Period. CreatedAt is set so Entries ordering can be exercised too.
func sysEntry(t models.LedgerEntryType, dir models.LedgerDirection, amount int64, period string, created time.Time) *models.LedgerEntry {
	p := period
	return &models.LedgerEntry{
		EntryType: t,
		Direction: dir,
		Amount:    amount,
		Period:    &p,
		Status:    models.StatusApproved,
		CreatedAt: created,
	}
}

// manualEntry builds an approved manual entry (chore/settlement/adjustment) with
// Period == nil, so its month is derived from CreatedAt (local).
func manualEntry(t models.LedgerEntryType, dir models.LedgerDirection, amount int64, created time.Time) *models.LedgerEntry {
	return &models.LedgerEntry{
		EntryType: t,
		Direction: dir,
		Amount:    amount,
		Period:    nil,
		Status:    models.StatusApproved,
		CreatedAt: created,
	}
}

// day returns noon local time on the given YYYY-MM-DD, a stable in-month instant.
func day(year int, month time.Month, d int) time.Time {
	return time.Date(year, month, d, 12, 0, 0, 0, time.Local)
}

// --- TestCompute_ClosingEqualsNextOpening (property, the invariant) -------

func TestCompute_ClosingEqualsNextOpening(t *testing.T) {
	// A ledger spanning 5 consecutive months mixing all types, including an
	// empty month (2026-03), a partial-type month (2026-05), and both a debit
	// and a credit adjustment.
	entries := []*models.LedgerEntry{
		// 2026-01
		sysEntry(models.EntryTypeAllowance, models.DirectionCredit, 1000, "2026-01", day(2026, 1, 1)),
		manualEntry(models.EntryTypeChore, models.DirectionCredit, 500, day(2026, 1, 10)),
		manualEntry(models.EntryTypeSettlement, models.DirectionDebit, 300, day(2026, 1, 20)),
		// 2026-02
		sysEntry(models.EntryTypeAllowance, models.DirectionCredit, 1000, "2026-02", day(2026, 2, 1)),
		sysEntry(models.EntryTypeEMI, models.DirectionDebit, 400, "2026-02", day(2026, 2, 1)),
		manualEntry(models.EntryTypeAdjustment, models.DirectionDebit, 200, day(2026, 2, 15)),
		// 2026-03 — empty
		// 2026-04
		sysEntry(models.EntryTypeAllowance, models.DirectionCredit, 1000, "2026-04", day(2026, 4, 1)),
		manualEntry(models.EntryTypeAdjustment, models.DirectionCredit, 250, day(2026, 4, 5)),
		manualEntry(models.EntryTypeChore, models.DirectionCredit, 150, day(2026, 4, 18)),
		// 2026-05 — partial (only a payment and an emi, no allowance/chore)
		sysEntry(models.EntryTypeEMI, models.DirectionDebit, 400, "2026-05", day(2026, 5, 1)),
		manualEntry(models.EntryTypeSettlement, models.DirectionDebit, 600, day(2026, 5, 22)),
	}

	months := []string{"2026-01", "2026-02", "2026-03", "2026-04", "2026-05", "2026-06"}
	for i := 0; i < len(months); i++ {
		m := months[i]
		s := Compute(entries, m)

		// Per-month figure identities (§2.3).
		assert.Equal(t, s.Opening+s.Base+s.Chores+s.Adjustments-s.EMI, s.TotalDue, "total_due identity for %s", m)
		assert.Equal(t, s.TotalDue-s.Cleared, s.Closing, "closing identity for %s", m)

		// The invariant: closing(M) == opening(M+1).
		if i+1 < len(months) {
			next := Compute(entries, months[i+1])
			assert.Equal(t, s.Closing, next.Opening, "closing(%s) must equal opening(%s)", m, months[i+1])
		}
	}

	// Sanity: the running closing equals Σ signed over all entries by the final
	// covered month.
	final := Compute(entries, "2026-05")
	// 1000+500-300 +1000-400-200 +1000+250+150 -400-600 = 2000
	assert.Equal(t, int64(2000), final.Closing)
}

// --- TestCompute_EmptyMonth_OpeningEqualsClosing -------------------------

func TestCompute_EmptyMonth_OpeningEqualsClosing(t *testing.T) {
	entries := []*models.LedgerEntry{
		sysEntry(models.EntryTypeAllowance, models.DirectionCredit, 1000, "2026-01", day(2026, 1, 1)),
	}
	s := Compute(entries, "2026-02") // carryover-only month
	assert.Equal(t, int64(1000), s.Opening)
	assert.Equal(t, int64(0), s.Base)
	assert.Equal(t, int64(0), s.Chores)
	assert.Equal(t, int64(0), s.Adjustments)
	assert.Equal(t, int64(0), s.EMI)
	assert.Equal(t, int64(0), s.Cleared)
	assert.Equal(t, s.Opening, s.Closing)
	assert.Empty(t, s.Entries)
}

// --- TestCompute_FirstMonth_OpeningZero ---------------------------------

func TestCompute_FirstMonth_OpeningZero(t *testing.T) {
	entries := []*models.LedgerEntry{
		sysEntry(models.EntryTypeAllowance, models.DirectionCredit, 1000, "2026-01", day(2026, 1, 1)),
		manualEntry(models.EntryTypeSettlement, models.DirectionDebit, 400, day(2026, 1, 15)),
	}
	s := Compute(entries, "2026-01")
	assert.Equal(t, int64(0), s.Opening, "no prior entries ⇒ opening 0")
	assert.Equal(t, int64(1000), s.Base)
	assert.Equal(t, int64(400), s.Cleared)
	assert.Equal(t, int64(1000), s.TotalDue)
	assert.Equal(t, int64(600), s.Closing) // earned − cleared
}

// --- TestCompute_PartialTypes -------------------------------------------

func TestCompute_PartialTypes(t *testing.T) {
	// Only a base credit and a payment — no chores/emi/adjustments.
	entries := []*models.LedgerEntry{
		sysEntry(models.EntryTypeAllowance, models.DirectionCredit, 800, "2026-03", day(2026, 3, 1)),
		manualEntry(models.EntryTypeSettlement, models.DirectionDebit, 300, day(2026, 3, 9)),
	}
	s := Compute(entries, "2026-03")
	assert.Equal(t, int64(800), s.Base)
	assert.Equal(t, int64(0), s.Chores)
	assert.Equal(t, int64(0), s.Adjustments)
	assert.Equal(t, int64(0), s.EMI)
	assert.Equal(t, int64(300), s.Cleared)
	assert.Equal(t, int64(800), s.TotalDue)
	assert.Equal(t, int64(500), s.Closing)
}

// --- TestCompute_AdjustmentSigns ----------------------------------------

func TestCompute_AdjustmentSigns(t *testing.T) {
	credit := Compute([]*models.LedgerEntry{
		manualEntry(models.EntryTypeAdjustment, models.DirectionCredit, 250, day(2026, 4, 3)),
	}, "2026-04")
	assert.Equal(t, int64(250), credit.Adjustments)
	assert.Equal(t, int64(250), credit.TotalDue)

	debit := Compute([]*models.LedgerEntry{
		manualEntry(models.EntryTypeAdjustment, models.DirectionDebit, 250, day(2026, 4, 3)),
	}, "2026-04")
	assert.Equal(t, int64(-250), debit.Adjustments, "debit adjustment is negative")
	assert.Equal(t, int64(-250), debit.TotalDue, "reduces total_due")
}

// --- TestCompute_FutureMonthProjectsBalance -----------------------------

func TestCompute_FutureMonthProjectsBalance(t *testing.T) {
	entries := []*models.LedgerEntry{
		sysEntry(models.EntryTypeAllowance, models.DirectionCredit, 1000, "2026-01", day(2026, 1, 1)),
		manualEntry(models.EntryTypeSettlement, models.DirectionDebit, 400, day(2026, 1, 15)),
	}
	s := Compute(entries, "2026-09") // month after every entry
	assert.Equal(t, int64(600), s.Opening)
	assert.Equal(t, int64(0), s.Base)
	assert.Equal(t, int64(0), s.Chores)
	assert.Equal(t, int64(0), s.Adjustments)
	assert.Equal(t, int64(0), s.EMI)
	assert.Equal(t, int64(0), s.Cleared)
	assert.Equal(t, s.Opening, s.Closing)
	assert.Empty(t, s.Entries)
}

// --- TestCompute_ExcludesNonApproved ------------------------------------

func TestCompute_ExcludesNonApproved(t *testing.T) {
	pending := manualEntry(models.EntryTypeChore, models.DirectionCredit, 500, day(2026, 5, 2))
	pending.Status = models.StatusPendingApproval
	rejected := manualEntry(models.EntryTypeChore, models.DirectionCredit, 700, day(2026, 5, 3))
	rejected.Status = models.StatusRejected

	entries := []*models.LedgerEntry{
		sysEntry(models.EntryTypeAllowance, models.DirectionCredit, 1000, "2026-05", day(2026, 5, 1)),
		pending,
		rejected,
	}
	s := Compute(entries, "2026-05")
	assert.Equal(t, int64(1000), s.Base)
	assert.Equal(t, int64(0), s.Chores, "pending/rejected chores excluded")
	assert.Len(t, s.Entries, 1, "only the approved entry appears in the passbook")
}

// --- TestCompute_EntriesOrderedAsc --------------------------------------

func TestCompute_EntriesOrderedAsc(t *testing.T) {
	// Pass entries out of order; expect created_at ASC, and only effMonth==M.
	entries := []*models.LedgerEntry{
		manualEntry(models.EntryTypeChore, models.DirectionCredit, 300, day(2026, 6, 20)),
		sysEntry(models.EntryTypeAllowance, models.DirectionCredit, 1000, "2026-06", day(2026, 6, 1)),
		manualEntry(models.EntryTypeSettlement, models.DirectionDebit, 100, day(2026, 6, 10)),
		manualEntry(models.EntryTypeChore, models.DirectionCredit, 999, day(2026, 7, 1)), // other month
	}
	s := Compute(entries, "2026-06")
	require.Len(t, s.Entries, 3)
	assert.True(t, s.Entries[0].CreatedAt.Before(s.Entries[1].CreatedAt))
	assert.True(t, s.Entries[1].CreatedAt.Before(s.Entries[2].CreatedAt))
	for _, e := range s.Entries {
		assert.Equal(t, "2026-06", effMonth(e))
	}
}

// --- TestEffMonth_SystemUsesPeriod --------------------------------------

func TestEffMonth_SystemUsesPeriod(t *testing.T) {
	// Period wins even when created_at is in a different month.
	e := sysEntry(models.EntryTypeAllowance, models.DirectionCredit, 1000, "2026-02", day(2026, 5, 9))
	assert.Equal(t, "2026-02", effMonth(e))

	// Manual entry falls back to created_at's local month.
	m := manualEntry(models.EntryTypeChore, models.DirectionCredit, 100, day(2026, 5, 9))
	assert.Equal(t, "2026-05", effMonth(m))
}

// --- TestEffMonth_BoundaryAndTZ -----------------------------------------

func TestEffMonth_BoundaryAndTZ(t *testing.T) {
	// Deterministic local boundary: last second of July vs first second of Aug.
	lastJul := manualEntry(models.EntryTypeChore, models.DirectionCredit, 100,
		time.Date(2026, 7, 31, 23, 59, 59, 0, time.Local))
	firstAug := manualEntry(models.EntryTypeChore, models.DirectionCredit, 100,
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local))
	assert.Equal(t, "2026-07", effMonth(lastJul))
	assert.Equal(t, "2026-08", effMonth(firstAug))
	assert.NotEqual(t, effMonth(lastJul), effMonth(firstAug))

	// TZ conversion: a created_at stored in a non-local zone must be bucketed by
	// its LOCAL month (the §2.1 hazard). effMonth must equal the .Local() month,
	// which for a zone-straddling instant differs from formatting the raw zone.
	ist := time.FixedZone("IST", 5*3600+30*60)
	crosser := manualEntry(models.EntryTypeChore, models.DirectionCredit, 100,
		time.Date(2026, 8, 1, 2, 0, 0, 0, ist)) // 2026-07-31 20:30 UTC
	assert.Equal(t, crosser.CreatedAt.Local().Format("2006-01"), effMonth(crosser))
}
