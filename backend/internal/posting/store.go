package posting

import (
	"context"

	"github.com/google/uuid"

	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
)

// Store is the dependency inversion boundary for the posting engine.
// Unit tests supply a fake; the concrete adapter wraps the real repos.
type Store interface {
	// ListPostingInputs returns every member allowance row (+ that member's join date)
	// for the group, ordered by user then effective_from.
	ListPostingInputs(ctx context.Context, groupID uuid.UUID) ([]models.AllowancePostingInput, error)

	// PostedAllowancePeriods returns, per user, the set of periods already posted
	// as allowance entries in this group — the fast-path guard (§3.5).
	PostedAllowancePeriods(ctx context.Context, groupID uuid.UUID) (map[uuid.UUID]map[string]bool, error)

	// GroupHead returns the group's head user id (created_by for machine posts).
	GroupHead(ctx context.Context, groupID uuid.UUID) (uuid.UUID, error)

	// WithTx runs fn inside a single transaction (one tx per group, §5.4).
	WithTx(ctx context.Context, fn func(q db.Querier) error) error

	// InsertAllowancePosting is the idempotent allowance insert (§2.3), bound to the tx's Querier.
	InsertAllowancePosting(ctx context.Context, q db.Querier,
		groupID, userID uuid.UUID, amount int64, period string, createdBy uuid.UUID) (bool, error)

	// --- EMI posting additions (WP-3.1) ---

	// ListActiveLoans returns all active loans for the group in deterministic
	// (user_id, start_period, id) order — required for no-deadlock property (§4.2).
	ListActiveLoans(ctx context.Context, groupID uuid.UUID) ([]models.LoanPostingInput, error)

	// PostedEMIPeriods returns, per loan, the set of periods already posted as EMI
	// entries in this group — the fast-path guard (§4.4).
	PostedEMIPeriods(ctx context.Context, groupID uuid.UUID) (map[uuid.UUID]map[string]bool, error)

	// LockActiveLoan acquires FOR UPDATE on the loan row and returns whether it is
	// currently active. Serializes the engine against the early-payoff close endpoint (§4.5).
	LockActiveLoan(ctx context.Context, q db.Querier, loanID uuid.UUID) (bool, error)

	// InsertEMIPosting is the idempotent EMI debit insert (§2.3), bound to the tx's Querier.
	InsertEMIPosting(ctx context.Context, q db.Querier,
		groupID, userID, loanID uuid.UUID,
		amount int64, period string, note *string, createdBy uuid.UUID) (bool, error)

	// CountPostedEMIs counts committed EMI entries for a loan on the tx querier (§4.5).
	CountPostedEMIs(ctx context.Context, q db.Querier, loanID uuid.UUID) (int, error)

	// CloseLoan sets the loan status to closed on the tx querier (§4.5).
	CloseLoan(ctx context.Context, q db.Querier, loanID uuid.UUID) error
}
