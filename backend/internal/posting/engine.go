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
// Calling it any number of times, or concurrently, converges to that exact set.
func PostDue(ctx context.Context, store Store, groupID uuid.UUID, now time.Time) error {
	currentPeriod := formatPeriod(now)

	inputs, err := store.ListPostingInputs(ctx, groupID)
	if err != nil {
		return fmt.Errorf("list posting inputs: %w", err)
	}
	if len(inputs) == 0 {
		return nil // no allowances configured — genuine no-op
	}

	posted, err := store.PostedAllowancePeriods(ctx, groupID)
	if err != nil {
		return fmt.Errorf("posted allowance periods: %w", err)
	}

	head, err := store.GroupHead(ctx, groupID)
	if err != nil {
		return fmt.Errorf("group head: %w", err)
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
				if _, err := store.InsertAllowancePosting(ctx, q, groupID, userID, amt, p, head); err != nil {
					return fmt.Errorf("insert allowance posting (user=%s period=%s): %w", userID, p, err)
				}
			}
			// EMI posting: WP-3.1 slots loan iteration here (out of scope).
		}
		return nil
	})
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
