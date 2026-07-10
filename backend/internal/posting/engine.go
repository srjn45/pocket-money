package posting

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
)

// PostDue makes the ledger contain exactly one approved allowance credit per
// (member, month) for every month from max(effective_from, join-month) through
// the month of now, at the amount in force that month, skipping paused months.
// It also posts one approved emi debit per due installment period for every
// active loan in the group. Calling it any number of times, or concurrently,
// converges to the exact expected set.
//
// Determinism invariant: both allowance and EMI inserts iterate users/loans in
// a fixed SQL-ordered sequence so concurrent PostDue calls attempt their inserts
// in identical order. Iterating a Go map to derive this order would randomize it
// and could cause deadlocks under concurrent transactions — never do that here.
func PostDue(ctx context.Context, store Store, groupID uuid.UUID, now time.Time) error {
	currentPeriod := formatPeriod(now)

	inputs, err := store.ListPostingInputs(ctx, groupID)
	if err != nil {
		return fmt.Errorf("list posting inputs: %w", err)
	}

	loans, err := store.ListActiveLoans(ctx, groupID)
	if err != nil {
		return fmt.Errorf("list active loans: %w", err)
	}

	if len(inputs) == 0 && len(loans) == 0 {
		return nil // no allowances and no active loans — genuine no-op
	}

	posted, err := store.PostedAllowancePeriods(ctx, groupID)
	if err != nil {
		return fmt.Errorf("posted allowance periods: %w", err)
	}

	postedEMI, err := store.PostedEMIPeriods(ctx, groupID)
	if err != nil {
		return fmt.Errorf("posted emi periods: %w", err)
	}

	admin, err := store.GroupAdmin(ctx, groupID)
	if err != nil {
		return fmt.Errorf("group admin: %w", err)
	}

	// Group inputs by user; rows are already sorted asc by user then effective_from.
	// userOrder preserves the SQL user_id ordering so that concurrent PostDue calls
	// attempt their inserts in an identical, deterministic order. This turns a
	// potential cross-row deadlock (two txns locking the same rows in opposite
	// order) into a plain block-then-no-op — the loser waits, then ON CONFLICT
	// DO NOTHING finds the committed row. Iterating a map here would randomize the
	// order and could deadlock a multi-member group under concurrent triggers.
	byUser, userOrder := groupByUser(inputs)

	return store.WithTx(ctx, func(q db.Querier) error {
		// First loop: allowance posting (unchanged WP-2.1 logic).
		for _, userID := range userOrder {
			rows := byUser[userID]
			joinMonth := formatPeriod(rows[0].JoinedAt) // all rows share the member's joined_at
			firstEff := rows[0].EffectiveFrom
			start := maxPeriod(firstEff, joinMonth)

			for _, p := range monthsInclusive(start, currentPeriod) {
				if posted[userID][p] {
					continue // fast-path: already have this period
				}
				amt := amountInForce(rows, p)
				if amt == 0 {
					continue // paused month — post nothing
				}
				if _, err := store.InsertAllowancePosting(ctx, q, groupID, userID, amt, p, admin); err != nil {
					return fmt.Errorf("insert allowance posting (user=%s period=%s): %w", userID, p, err)
				}
			}
		}

		// Second loop: EMI posting. loans is already in (user_id, start_period, id)
		// SQL order — deterministic across concurrent calls, preserving the
		// no-deadlock property (loans are locked in exactly this order via FOR UPDATE).
		for _, L := range loans {
			if err := postLoanEMIs(ctx, store, q, groupID, L, currentPeriod, postedEMI[L.LoanID], admin); err != nil {
				return err
			}
		}

		return nil
	})
}

// postLoanEMIs posts all due EMI installments for one active loan and auto-closes
// the loan when all scheduled installments have been committed. The FOR UPDATE
// lock on the loan row serializes this against concurrent PostDue calls and
// against the early-payoff close endpoint (§4.5).
func postLoanEMIs(ctx context.Context, store Store, q db.Querier,
	groupID uuid.UUID, L models.LoanPostingInput,
	currentPeriod string, postedForLoan map[string]bool, admin uuid.UUID) error {

	sched := emiSchedule(L.Principal, L.EMIAmount, L.Installments)

	// Fast path: skip entirely if all due periods are already covered.
	// No lock acquired in this case — preserves cheap no-op for caught-up loans.
	if allDuePostedAlready(sched, L.StartPeriod, currentPeriod, postedForLoan) {
		return nil
	}

	active, err := store.LockActiveLoan(ctx, q, L.LoanID)
	if err != nil {
		return fmt.Errorf("lock active loan %s: %w", L.LoanID, err)
	}
	if !active {
		return nil // loan was concurrently closed or rejected — nothing to post
	}

	for i, amt := range sched {
		p := AddMonths(L.StartPeriod, i)
		if p > currentPeriod {
			break // installment not yet due
		}
		if postedForLoan[p] {
			continue // fast-path: already posted this period
		}
		if _, err := store.InsertEMIPosting(ctx, q, groupID, L.UserID, L.LoanID, amt, p, nil, admin); err != nil {
			return fmt.Errorf("insert emi posting (loan=%s period=%s): %w", L.LoanID, p, err)
		}
	}

	// Auto-close: recount under the FOR UPDATE lock — authoritative after inserts.
	count, err := store.CountPostedEMIs(ctx, q, L.LoanID)
	if err != nil {
		return fmt.Errorf("count posted emis (loan=%s): %w", L.LoanID, err)
	}
	if count >= len(sched) {
		if err := store.CloseLoan(ctx, q, L.LoanID); err != nil {
			return fmt.Errorf("close loan %s: %w", L.LoanID, err)
		}
	}

	return nil
}

// allDuePostedAlready returns true when every installment due on or before
// currentPeriod is already present in the posted guard. This enables a zero-lock
// fast path for loans that are fully caught up (§4.5).
func allDuePostedAlready(sched []int64, startPeriod, currentPeriod string, posted map[string]bool) bool {
	for i := range sched {
		p := AddMonths(startPeriod, i)
		if p > currentPeriod {
			return true // remaining installments are future — nothing more due
		}
		if !posted[p] {
			return false
		}
	}
	return true // all installments are covered
}

// emiSchedule returns the amount of each installment in order. The last real
// installment absorbs rounding so the sum equals principal exactly (§5.3).
// Robust to pathological tiny principals: it stops as soon as the loan is fully
// repaid, so no installment is ever <= 0.
func emiSchedule(principal, emiAmount int64, installments int) []int64 {
	out := make([]int64, 0, installments)
	var paid int64
	for i := 0; i < installments; i++ {
		remaining := principal - paid
		if remaining <= 0 {
			break // fully repaid before all installments (only for tiny principals)
		}
		amt := emiAmount
		if amt > remaining {
			amt = remaining // final installment: absorbs rounding / clamps to remaining
		}
		out = append(out, amt)
		paid += amt
	}
	return out
}

// AddMonths returns the YYYY-MM period that is k months after the given period.
// Uses integer month arithmetic to avoid DST/time-zone surprises (same approach
// as monthsInclusive). k may be 0 (returns the same period).
// Exported so handlers can compute start_period (next month) without time.AddDate.
func AddMonths(period string, k int) string {
	y, m := parsePeriod(period)
	n := y*12 + (m - 1) + k
	ry := n / 12
	rm := n%12 + 1
	return fmt.Sprintf("%04d-%02d", ry, rm)
}

// groupByUser groups AllowancePostingInput rows by UserID, returning both the
// grouping and a slice of user IDs in first-appearance (SQL user_id) order. The
// ordered slice gives callers a deterministic iteration order independent of Go
// map randomization — required so concurrent transactions lock rows in the same
// order and cannot deadlock (see PostDue).
func groupByUser(inputs []models.AllowancePostingInput) (map[uuid.UUID][]models.AllowancePostingInput, []uuid.UUID) {
	m := make(map[uuid.UUID][]models.AllowancePostingInput)
	var order []uuid.UUID
	for _, inp := range inputs {
		if _, seen := m[inp.UserID]; !seen {
			order = append(order, inp.UserID)
		}
		m[inp.UserID] = append(m[inp.UserID], inp)
	}
	return m, order
}

// formatPeriod returns the YYYY-MM string for t in server-local time.
func formatPeriod(t time.Time) string {
	return t.Format("2006-01")
}

// monthsInclusive returns an ascending list of YYYY-MM strings from start to end
// inclusive. Returns empty slice if start > end (e.g. future effective_from).
// Uses integer month arithmetic to avoid DST/time-zone surprises.
func monthsInclusive(start, end string) []string {
	if start > end {
		return nil
	}
	sy, sm := parsePeriod(start)
	ey, em := parsePeriod(end)
	startN := sy*12 + (sm - 1)
	endN := ey*12 + (em - 1)

	result := make([]string, 0, endN-startN+1)
	for n := startN; n <= endN; n++ {
		y := n / 12
		m := n%12 + 1
		result = append(result, fmt.Sprintf("%04d-%02d", y, m))
	}
	return result
}

// parsePeriod parses "YYYY-MM" into (year, month) ints. Assumes valid format.
func parsePeriod(p string) (int, int) {
	y := int(p[0]-'0')*1000 + int(p[1]-'0')*100 + int(p[2]-'0')*10 + int(p[3]-'0')
	m := int(p[5]-'0')*10 + int(p[6]-'0')
	return y, m
}

// amountInForce returns the amount from the row with the greatest effective_from <= period.
// rows must be sorted ascending by effective_from. There is always such a row
// because start >= firstEff is enforced by the caller.
func amountInForce(rows []models.AllowancePostingInput, period string) int64 {
	var amount int64
	for _, r := range rows {
		if r.EffectiveFrom > period {
			break
		}
		amount = r.Amount
	}
	return amount
}

// maxPeriod returns the lexicographically greater of two YYYY-MM strings.
// Lexical comparison is exact for zero-padded YYYY-MM.
func maxPeriod(a, b string) string {
	if a >= b {
		return a
	}
	return b
}
