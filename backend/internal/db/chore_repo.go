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

// ErrSystemChore is returned when trying to modify a system chore
var ErrSystemChore = errors.New("cannot modify system chore")

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
	return r.CreateWithSystem(ctx, groupID, name, description, amount, false)
}

// CreateWithSystem inserts a new chore with system flag
func (r *ChoreRepo) CreateWithSystem(ctx context.Context, groupID uuid.UUID, name string, description *string, amount float64, isSystem bool) (*models.Chore, error) {
	chore := &models.Chore{
		ID:          uuid.New(),
		GroupID:     groupID,
		Name:        name,
		Description: description,
		Amount:      amount,
		IsSystem:    isSystem,
	}

	query := `
		INSERT INTO chores (id, group_id, name, description, amount, is_system)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at
	`

	err := r.pool.QueryRow(ctx, query, chore.ID, groupID, name, description, amount, isSystem).Scan(&chore.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create chore: %w", err)
	}

	return chore, nil
}

// GetByID retrieves a chore by ID (including soft-deleted)
func (r *ChoreRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Chore, error) {
	chore := &models.Chore{}

	query := `
		SELECT id, group_id, name, description, amount, is_system, deleted_at, created_at
		FROM chores
		WHERE id = $1
	`

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&chore.ID,
		&chore.GroupID,
		&chore.Name,
		&chore.Description,
		&chore.Amount,
		&chore.IsSystem,
		&chore.DeletedAt,
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

// ListForGroup retrieves all active (non-deleted) chores for a group
func (r *ChoreRepo) ListForGroup(ctx context.Context, groupID uuid.UUID) ([]*models.Chore, error) {
	query := `
		SELECT id, group_id, name, description, amount, is_system, deleted_at, created_at
		FROM chores
		WHERE group_id = $1 AND deleted_at IS NULL
		ORDER BY is_system DESC, created_at DESC
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
			&chore.IsSystem,
			&chore.DeletedAt,
			&chore.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan chore: %w", err)
		}
		chores = append(chores, chore)
	}

	return chores, nil
}

// Update updates a chore (cannot update system chores)
func (r *ChoreRepo) Update(ctx context.Context, id uuid.UUID, name *string, description *string, amount *float64) (*models.Chore, error) {
	// First check if it's a system chore
	existing, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.IsSystem {
		return nil, ErrSystemChore
	}

	query := `
		UPDATE chores
		SET name = COALESCE($2, name),
		    description = COALESCE($3, description),
		    amount = COALESCE($4, amount)
		WHERE id = $1 AND deleted_at IS NULL AND is_system = false
		RETURNING id, group_id, name, description, amount, is_system, deleted_at, created_at
	`

	chore := &models.Chore{}
	err = r.pool.QueryRow(ctx, query, id, name, description, amount).Scan(
		&chore.ID,
		&chore.GroupID,
		&chore.Name,
		&chore.Description,
		&chore.Amount,
		&chore.IsSystem,
		&chore.DeletedAt,
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

// Delete soft-deletes a chore (cannot delete system chores)
func (r *ChoreRepo) Delete(ctx context.Context, id uuid.UUID) error {
	// First check if it's a system chore
	existing, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.IsSystem {
		return ErrSystemChore
	}

	query := `
		UPDATE chores 
		SET deleted_at = now() 
		WHERE id = $1 AND deleted_at IS NULL AND is_system = false
	`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete chore: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// GetSystemChoreForGroup returns the system (Settlement) chore for a group
func (r *ChoreRepo) GetSystemChoreForGroup(ctx context.Context, groupID uuid.UUID) (*models.Chore, error) {
	chore := &models.Chore{}

	query := `
		SELECT id, group_id, name, description, amount, is_system, deleted_at, created_at
		FROM chores
		WHERE group_id = $1 AND is_system = true AND deleted_at IS NULL
		LIMIT 1
	`

	err := r.pool.QueryRow(ctx, query, groupID).Scan(
		&chore.ID,
		&chore.GroupID,
		&chore.Name,
		&chore.Description,
		&chore.Amount,
		&chore.IsSystem,
		&chore.DeletedAt,
		&chore.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get system chore: %w", err)
	}

	return chore, nil
}
