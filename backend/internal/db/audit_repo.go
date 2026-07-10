package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditRepo writes the invisible entry_audit log for manual-entry corrections
// (V3-3.2 §2). INSERT-ONLY in this WP — there is no read API (D3). One row is
// written, capturing the full prior ledger row as jsonb, before every edit and
// delete.
type AuditRepo struct {
	pool *pgxpool.Pool
}

// NewAuditRepo creates a new AuditRepo.
func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{pool: pool}
}

// Insert writes one entry_audit row. q lets it join the caller's transaction so
// the audit row is atomic with the edit/delete mutation it records. oldRow is the
// JSON snapshot (json.Marshal of the pre-change *models.LedgerEntry) taken FOR
// UPDATE in the same tx; action is models.AuditActionEdit or AuditActionDelete;
// actor is the admin performing the correction.
func (r *AuditRepo) Insert(ctx context.Context, q Querier,
	entryID uuid.UUID, oldRow []byte, action string, actor uuid.UUID) error {
	_, err := q.Exec(ctx, `
		INSERT INTO entry_audit (id, entry_id, old_row, action, actor)
		VALUES (gen_random_uuid(), $1, $2, $3, $4)`,
		entryID, oldRow, action, actor)
	if err != nil {
		return fmt.Errorf("failed to insert entry audit: %w", err)
	}
	return nil
}
