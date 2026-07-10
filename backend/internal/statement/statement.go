// Package statement computes a member's derived monthly statement from their
// approved ledger entries. It is the single source of truth for the statement
// math (master-plan-v3 §3.9): opening/base/chores/adjustments/emi/total_due/
// cleared/closing. Pure and dependency-free (only models, time, sort) so the
// closing(M) == opening(M+1) invariant and every edge case are provable in unit
// tests without a database (§9.3).
package statement

import (
	"sort"
	"time"

	"github.com/srjn45/pocket-money/backend/internal/models"
)

// MemberStatement is one member's derived figures for a month, in int64 minor
// units (currency is applied at the handler boundary — Compute is currency-blind,
// D7). Signed fields (Opening, Adjustments, TotalDue, Closing) may be < 0;
// magnitude fields (Base, Chores, EMI, Cleared) are ≥ 0.
type MemberStatement struct {
	Opening     int64 // Σ signed(e), effMonth < M       (signed)
	Base        int64 // Σ allowance credit in M          (≥ 0)
	Chores      int64 // Σ chore credit in M              (≥ 0)
	Adjustments int64 // Σ signed adjustment in M         (signed)
	EMI         int64 // Σ emi debit magnitude in M       (≥ 0)
	TotalDue    int64 // Opening + Base + Chores + Adjustments − EMI  (signed)
	Cleared     int64 // Σ settlement debit magnitude in M (≥ 0)
	Closing     int64 // TotalDue − Cleared                (signed)

	Entries []*models.LedgerEntry // month M rows for this member, created_at ASC
}

// FormatPeriodLocal renders a timestamp as a YYYY-MM period in server-local time.
//
// pgx returns timestamptz (created_at) as a time.Time in UTC, while PostDue
// derives system-entry periods from time.Now() (server-local). To bucket manual
// entries into the same calendar the system entries use, we MUST convert to
// local before formatting — formatting the raw UTC value would mis-bucket entries
// created near a month boundary and could break the closing==next-opening
// invariant (§2.1). This helper centralizes that conversion; do not inline a raw
// Format elsewhere. The handler reuses it for the current-month default and for
// join-month flooring (§3.2, §3.5) so every "month" uses one calendar.
func FormatPeriodLocal(t time.Time) string {
	return t.Local().Format("2006-01")
}

// signed returns +Amount for a credit entry and −Amount for a debit entry.
// Amount is always a non-negative magnitude (matches the API contract:
// "value is always positive; direction indicates credit/debit").
func signed(e *models.LedgerEntry) int64 {
	if e.Direction == models.DirectionDebit {
		return -e.Amount
	}
	return e.Amount
}

// effMonth returns the single month (YYYY-MM) an entry belongs to (§2.1):
//   - system entries (allowance, emi) carry an explicit Period → that wins;
//   - manual entries (chore, settlement, adjustment) have Period == nil → the
//     calendar month of created_at in server-local time.
//
// This one function is used identically for the opening (< M) and in-month (== M)
// buckets, which is what makes the closing==next-opening invariant structural.
func effMonth(e *models.LedgerEntry) string {
	if e.Period != nil {
		return *e.Period
	}
	return FormatPeriodLocal(e.CreatedAt)
}

// Compute derives one member's statement for month (YYYY-MM) from ALL of that
// member's approved ledger entries (any month, any type). Pure: no I/O, no clock
// beyond time.Time.Local() on each entry's CreatedAt. Deterministic.
//
// Non-approved entries are ignored defensively (the handler already filters to
// approved). Entries holds the effMonth==month subset sorted created_at ASC
// (chronological passbook order); the caller may pass any order.
func Compute(entries []*models.LedgerEntry, month string) MemberStatement {
	var s MemberStatement
	var monthEntries []*models.LedgerEntry

	for _, e := range entries {
		if e.Status != models.StatusApproved {
			continue // defensive; handler filters to approved
		}
		m := effMonth(e)
		switch {
		case m < month:
			// D1 carryover: everything before M rolls into the opening balance.
			s.Opening += signed(e)
		case m == month:
			monthEntries = append(monthEntries, e)
			switch e.EntryType {
			case models.EntryTypeAllowance:
				s.Base += e.Amount
			case models.EntryTypeChore:
				s.Chores += e.Amount
			case models.EntryTypeAdjustment:
				s.Adjustments += signed(e)
			case models.EntryTypeEMI:
				s.EMI += e.Amount
			case models.EntryTypeSettlement:
				s.Cleared += e.Amount
			}
		default:
			// effMonth > month: belongs to a future month; excluded from M.
		}
	}

	// total_due and closing follow §2.3 exactly. Because opening, adjustments,
	// and the settlement debit (cleared) are all expressed via the same signed()
	// accumulation, closing == Σ signed over effMonth ≤ M == opening(M+1) (§2.4).
	s.TotalDue = s.Opening + s.Base + s.Chores + s.Adjustments - s.EMI
	s.Closing = s.TotalDue - s.Cleared

	sort.SliceStable(monthEntries, func(i, j int) bool {
		return monthEntries[i].CreatedAt.Before(monthEntries[j].CreatedAt)
	})
	s.Entries = monthEntries

	return s
}
