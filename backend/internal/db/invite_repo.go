package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srjn45/pocket-money/backend/internal/models"
)

// InviteRepo handles database operations for invite tokens
type InviteRepo struct {
	pool *pgxpool.Pool
}

// NewInviteRepo creates a new InviteRepo
func NewInviteRepo(pool *pgxpool.Pool) *InviteRepo {
	return &InviteRepo{pool: pool}
}

// Create inserts a new invite token
func (r *InviteRepo) Create(ctx context.Context, groupID uuid.UUID, token string, expiresAt time.Time) (*models.InviteToken, error) {
	invite := &models.InviteToken{
		ID:        uuid.New(),
		GroupID:   groupID,
		Token:     token,
		ExpiresAt: expiresAt,
	}

	query := `
		INSERT INTO invite_tokens (id, group_id, token, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`

	err := r.pool.QueryRow(ctx, query, invite.ID, groupID, token, expiresAt).Scan(&invite.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create invite token: %w", err)
	}

	return invite, nil
}

// GetByToken retrieves an invite token by its token string
func (r *InviteRepo) GetByToken(ctx context.Context, token string) (*models.InviteToken, error) {
	invite := &models.InviteToken{}

	query := `
		SELECT id, group_id, token, expires_at, created_at
		FROM invite_tokens
		WHERE token = $1
	`

	err := r.pool.QueryRow(ctx, query, token).Scan(
		&invite.ID,
		&invite.GroupID,
		&invite.Token,
		&invite.ExpiresAt,
		&invite.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get invite token: %w", err)
	}

	return invite, nil
}

// Delete removes an invite token
func (r *InviteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM invite_tokens WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete invite token: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteExpired removes all expired invite tokens
func (r *InviteRepo) DeleteExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM invite_tokens WHERE expires_at < $1`

	result, err := r.pool.Exec(ctx, query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired invite tokens: %w", err)
	}

	return result.RowsAffected(), nil
}
