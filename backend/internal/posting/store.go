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
}
