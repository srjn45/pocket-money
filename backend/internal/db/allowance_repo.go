package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srjn45/pocket-money/backend/internal/models"
)

// AllowanceRepo handles database operations for allowance configuration.
type AllowanceRepo struct {
	pool *pgxpool.Pool
}

// NewAllowanceRepo creates a new AllowanceRepo.
func NewAllowanceRepo(pool *pgxpool.Pool) *AllowanceRepo {
	return &AllowanceRepo{pool: pool}
}

// SetAllowance upserts a member's monthly allowance for a given effective_from month.
// Re-setting the same month updates the amount (correction). Returns the row.
func (r *AllowanceRepo) SetAllowance(ctx context.Context,
	groupID, userID uuid.UUID, amount int64, effectiveFrom string, createdBy uuid.UUID) (*models.Allowance, error) {

	a := &models.Allowance{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO allowances (id, group_id, user_id, amount, effective_from, created_by)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)
		ON CONFLICT (group_id, user_id, effective_from)
		DO UPDATE SET amount = EXCLUDED.amount, created_by = EXCLUDED.created_by
		RETURNING id, group_id, user_id, amount, effective_from, created_by, created_at`,
		groupID, userID, amount, effectiveFrom, createdBy,
	).Scan(&a.ID, &a.GroupID, &a.UserID, &a.Amount, &a.EffectiveFrom, &a.CreatedBy, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to set allowance: %w", err)
	}
	return a, nil
}

// ListForGroup returns all allowance rows for a group (head view), ordered by user then month.
func (r *AllowanceRepo) ListForGroup(ctx context.Context, groupID uuid.UUID) ([]*models.Allowance, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, group_id, user_id, amount, effective_from, created_by, created_at
		FROM allowances
		WHERE group_id = $1
		ORDER BY user_id, effective_from`,
		groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to list allowances for group: %w", err)
	}
	defer rows.Close()

	var result []*models.Allowance
	for rows.Next() {
		a := &models.Allowance{}
		if err := rows.Scan(&a.ID, &a.GroupID, &a.UserID, &a.Amount, &a.EffectiveFrom, &a.CreatedBy, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan allowance: %w", err)
		}
		result = append(result, a)
	}
	return result, nil
}

// ListForUser returns one member's allowance rows (member view), ordered by month.
func (r *AllowanceRepo) ListForUser(ctx context.Context, groupID, userID uuid.UUID) ([]*models.Allowance, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, group_id, user_id, amount, effective_from, created_by, created_at
		FROM allowances
		WHERE group_id = $1 AND user_id = $2
		ORDER BY effective_from`,
		groupID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list allowances for user: %w", err)
	}
	defer rows.Close()

	var result []*models.Allowance
	for rows.Next() {
		a := &models.Allowance{}
		if err := rows.Scan(&a.ID, &a.GroupID, &a.UserID, &a.Amount, &a.EffectiveFrom, &a.CreatedBy, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan allowance: %w", err)
		}
		result = append(result, a)
	}
	return result, nil
}

// ListPostingInputs returns each member's allowance rows joined with their join date,
// filtered to role='member'. Ordered by user then effective_from for the posting engine.
func (r *AllowanceRepo) ListPostingInputs(ctx context.Context, groupID uuid.UUID) ([]models.AllowancePostingInput, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.user_id, a.amount, a.effective_from, gm.joined_at
		FROM allowances a
		JOIN group_members gm ON gm.group_id = a.group_id AND gm.user_id = a.user_id
		WHERE a.group_id = $1 AND gm.role = 'member'
		ORDER BY a.user_id, a.effective_from`,
		groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to list posting inputs: %w", err)
	}
	defer rows.Close()

	var result []models.AllowancePostingInput
	for rows.Next() {
		var inp models.AllowancePostingInput
		if err := rows.Scan(&inp.UserID, &inp.Amount, &inp.EffectiveFrom, &inp.JoinedAt); err != nil {
			return nil, fmt.Errorf("failed to scan posting input: %w", err)
		}
		result = append(result, inp)
	}
	return result, nil
}
