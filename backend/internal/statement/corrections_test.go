package statement

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/srjn45/pocket-money/backend/internal/models"
)

// V3-3.2 §5: an edit mutates only amount/direction/note (never created_at/period/
// entry_type), so the entry stays in the same effMonth. A magnitude change of Δ
// moves closing(M) and opening(M+1) by the SAME ±Δ, so closing(M)==opening(M+1)
// still holds. These are pure Compute-level proofs (no DB) mirroring the
// integration proofs (§7).

// invariantHolds asserts closing(m) == opening(m+1) across every consecutive
// month pair in months, returning the closing of the first month for delta checks.
func assertInvariant(t *testing.T, entries []*models.LedgerEntry, months []string) {
	t.Helper()
	for i := 0; i+1 < len(months); i++ {
		cur := Compute(entries, months[i])
		next := Compute(entries, months[i+1])
		assert.Equal(t, cur.Closing, next.Opening,
			"closing(%s) must equal opening(%s)", months[i], months[i+1])
	}
}

func TestCompute_EditedAmountPreservesInvariant(t *testing.T) {
	// Month M = 2026-01 holds an approved chore credit; M+1 = 2026-02 holds an
	// allowance so both months have rows and a real opening carries across.
	chore := manualEntry(models.EntryTypeChore, models.DirectionCredit, 500, day(2026, 1, 10))
	entries := []*models.LedgerEntry{
		sysEntry(models.EntryTypeAllowance, models.DirectionCredit, 1000, "2026-01", day(2026, 1, 1)),
		chore,
		sysEntry(models.EntryTypeAllowance, models.DirectionCredit, 1000, "2026-02", day(2026, 2, 1)),
	}
	months := []string{"2026-01", "2026-02", "2026-03"}

	// Pre-edit: invariant holds and closing(M) reflects the original chore amount.
	assertInvariant(t, entries, months)
	preM := Compute(entries, "2026-01")
	preNextOpening := Compute(entries, "2026-02").Opening
	assert.Equal(t, preM.Closing, preNextOpening)
	assert.Equal(t, int64(500), preM.Chores)

	// Edit: bump the chore amount 500 → 800 (Δ = +300), the only mutation.
	chore.Amount = 800

	postM := Compute(entries, "2026-01")
	postNextOpening := Compute(entries, "2026-02").Opening

	// The month figure moved by exactly Δ; both sides of the invariant moved equally.
	assert.Equal(t, int64(800), postM.Chores)
	assert.Equal(t, preM.Closing+300, postM.Closing, "closing(M) shifts by +Δ")
	assert.Equal(t, preNextOpening+300, postNextOpening, "opening(M+1) shifts by +Δ")
	assertInvariant(t, entries, months)

	// An adjustment direction flip (credit → debit) also keeps the invariant.
	adj := manualEntry(models.EntryTypeAdjustment, models.DirectionCredit, 200, day(2026, 1, 20))
	entries = append(entries, adj)
	assertInvariant(t, entries, months)
	beforeFlip := Compute(entries, "2026-01").Closing
	adj.Direction = models.DirectionDebit // signed contribution goes +200 → −200 (Δ = −400)
	afterFlip := Compute(entries, "2026-01").Closing
	assert.Equal(t, beforeFlip-400, afterFlip, "direction flip moves closing by −2×amount")
	assertInvariant(t, entries, months)
}

func TestCompute_DeletedEntryPreservesInvariant(t *testing.T) {
	// A settlement debit in M plus allowances in M and M+1.
	entries := []*models.LedgerEntry{
		sysEntry(models.EntryTypeAllowance, models.DirectionCredit, 1000, "2026-01", day(2026, 1, 1)),
		manualEntry(models.EntryTypeSettlement, models.DirectionDebit, 300, day(2026, 1, 12)),
		sysEntry(models.EntryTypeAllowance, models.DirectionCredit, 1000, "2026-02", day(2026, 2, 1)),
	}
	months := []string{"2026-01", "2026-02", "2026-03"}

	assertInvariant(t, entries, months)
	preM := Compute(entries, "2026-01")
	preNextOpening := Compute(entries, "2026-02").Opening

	// Delete the settlement (signed = −300). Removing it raises closing(M) and
	// opening(M+1) by +300 each.
	deleted := entries[1]
	entries = append(entries[:1], entries[2:]...)

	postM := Compute(entries, "2026-01")
	postNextOpening := Compute(entries, "2026-02").Opening

	assert.Equal(t, int64(0), postM.Cleared, "deleted settlement no longer counts as cleared")
	assert.Equal(t, preM.Closing-signed(deleted), postM.Closing, "closing(M) drops by signed(deleted)")
	assert.Equal(t, preNextOpening-signed(deleted), postNextOpening, "opening(M+1) drops by signed(deleted)")
	assertInvariant(t, entries, months)
}
