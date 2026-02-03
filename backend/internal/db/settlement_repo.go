package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srjn45/pocket-money/backend/internal/models"
)

// SettlementRepo handles database operations for settlements
type SettlementRepo struct {
	pool *pgxpool.Pool
}

// NewSettlementRepo creates a new SettlementRepo
func NewSettlementRepo(pool *pgxpool.Pool) *SettlementRepo {
	return &SettlementRepo{pool: pool}
}

// Create inserts a new settlement
func (r *SettlementRepo) Create(ctx context.Context, groupID, userID uuid.UUID, amount float64, date time.Time, note *string) (*models.Settlement, error) {
	settlement := &models.Settlement{
		ID:      uuid.New(),
		GroupID: groupID,
		UserID:  userID,
		Amount:  amount,
		Date:    date,
		Note:    note,
	}

	query := `
		INSERT INTO settlements (id, group_id, user_id, amount, date, note)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at
	`

	err := r.pool.QueryRow(ctx, query,
		settlement.ID, groupID, userID, amount, date, note,
	).Scan(&settlement.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create settlement: %w", err)
	}

	return settlement, nil
}

// ListForGroup retrieves all settlements for a group
func (r *SettlementRepo) ListForGroup(ctx context.Context, groupID uuid.UUID) ([]*models.Settlement, error) {
	query := `
		SELECT id, group_id, user_id, amount, date, note, created_at
		FROM settlements
		WHERE group_id = $1
		ORDER BY date DESC, created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to list settlements: %w", err)
	}
	defer rows.Close()

	var settlements []*models.Settlement
	for rows.Next() {
		settlement := &models.Settlement{}
		if err := rows.Scan(
			&settlement.ID,
			&settlement.GroupID,
			&settlement.UserID,
			&settlement.Amount,
			&settlement.Date,
			&settlement.Note,
			&settlement.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan settlement: %w", err)
		}
		settlements = append(settlements, settlement)
	}

	return settlements, nil
}
