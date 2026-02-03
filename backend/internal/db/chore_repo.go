package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/srjn45/pocket-money/backend/internal/models"
)

// ChoreRepo handles database operations for chores
type ChoreRepo struct {
	pool *pgxpool.Pool
}

// NewChoreRepo creates a new ChoreRepo
func NewChoreRepo(pool *pgxpool.Pool) *ChoreRepo {
	return &ChoreRepo{pool: pool}
}

// Create inserts a new chore into the database
func (r *ChoreRepo) Create(ctx context.Context, groupID uuid.UUID, name string, description *string, amount float64) (*models.Chore, error) {
	chore := &models.Chore{
		ID:          uuid.New(),
		GroupID:     groupID,
		Name:        name,
		Description: description,
		Amount:      amount,
	}

	query := `
		INSERT INTO chores (id, group_id, name, description, amount)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at
	`

	err := r.pool.QueryRow(ctx, query, chore.ID, groupID, name, description, amount).Scan(&chore.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create chore: %w", err)
	}

	return chore, nil
}

// GetByID retrieves a chore by ID
func (r *ChoreRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Chore, error) {
	chore := &models.Chore{}

	query := `
		SELECT id, group_id, name, description, amount, created_at
		FROM chores
		WHERE id = $1
	`

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&chore.ID,
		&chore.GroupID,
		&chore.Name,
		&chore.Description,
		&chore.Amount,
		&chore.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get chore by id: %w", err)
	}

	return chore, nil
}

// ListForGroup retrieves all chores for a group
func (r *ChoreRepo) ListForGroup(ctx context.Context, groupID uuid.UUID) ([]*models.Chore, error) {
	query := `
		SELECT id, group_id, name, description, amount, created_at
		FROM chores
		WHERE group_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to list chores: %w", err)
	}
	defer rows.Close()

	var chores []*models.Chore
	for rows.Next() {
		chore := &models.Chore{}
		if err := rows.Scan(
			&chore.ID,
			&chore.GroupID,
			&chore.Name,
			&chore.Description,
			&chore.Amount,
			&chore.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan chore: %w", err)
		}
		chores = append(chores, chore)
	}

	return chores, nil
}

// Update updates a chore
func (r *ChoreRepo) Update(ctx context.Context, id uuid.UUID, name *string, description *string, amount *float64) (*models.Chore, error) {
	// Build dynamic update query
	query := `
		UPDATE chores
		SET name = COALESCE($2, name),
		    description = COALESCE($3, description),
		    amount = COALESCE($4, amount)
		WHERE id = $1
		RETURNING id, group_id, name, description, amount, created_at
	`

	chore := &models.Chore{}
	err := r.pool.QueryRow(ctx, query, id, name, description, amount).Scan(
		&chore.ID,
		&chore.GroupID,
		&chore.Name,
		&chore.Description,
		&chore.Amount,
		&chore.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to update chore: %w", err)
	}

	return chore, nil
}

// Delete deletes a chore
func (r *ChoreRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM chores WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete chore: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
