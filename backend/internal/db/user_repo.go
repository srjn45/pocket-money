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

// ErrNotFound is returned when a resource is not found
var ErrNotFound = errors.New("not found")

// ErrDuplicateEmail is returned when email already exists
var ErrDuplicateEmail = errors.New("email already exists")

// UserRepo handles database operations for users
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo creates a new UserRepo
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// Create inserts a new registered user into the database. Signature is
// unchanged (all existing callers pass a real hash string); status is set to
// 'registered' explicitly.
func (r *UserRepo) Create(ctx context.Context, email, passwordHash, name string, dob *string, sex *string) (*models.User, error) {
	user := &models.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: &passwordHash,
		Name:         name,
		Status:       models.UserStatusRegistered,
	}

	query := `
		INSERT INTO users (id, email, password_hash, name, status, dob, sex)
		VALUES ($1, $2, $3, $4, 'registered', $5, $6)
		RETURNING created_at
	`

	err := r.pool.QueryRow(ctx, query, user.ID, email, passwordHash, name, dob, sex).Scan(&user.CreatedAt)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrDuplicateEmail
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// CreateShadow creates a shadow user (no password) inside the caller's
// transaction (§3.1 case 3). Returns ErrDuplicateEmail on the unique violation
// (defensive; the caller already checked under FOR UPDATE).
func (r *UserRepo) CreateShadow(ctx context.Context, q Querier, email, name string) (*models.User, error) {
	user := &models.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: nil,
		Name:         name,
		Status:       models.UserStatusShadow,
	}

	query := `
		INSERT INTO users (id, email, password_hash, name, status)
		VALUES ($1, $2, NULL, $3, 'shadow')
		RETURNING created_at
	`

	err := q.QueryRow(ctx, query, user.ID, email, name).Scan(&user.CreatedAt)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrDuplicateEmail
		}
		return nil, fmt.Errorf("failed to create shadow user: %w", err)
	}

	return user, nil
}

// GetByEmailForUpdate reads a user by email FOR UPDATE on the caller's tx,
// locking the row so a concurrent register-claim cannot race an add-by-email.
// ErrNotFound when absent.
func (r *UserRepo) GetByEmailForUpdate(ctx context.Context, q Querier, email string) (*models.User, error) {
	user := &models.User{}

	query := `
		SELECT id, email, password_hash, name, status, claimed_at, dob, sex, created_at
		FROM users
		WHERE email = $1
		FOR UPDATE
	`

	err := q.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.Status,
		&user.ClaimedAt,
		&user.DOB,
		&user.Sex,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by email for update: %w", err)
	}

	return user, nil
}

// ClaimShadow upgrades a shadow row in place (same id): sets the password,
// status='registered', claimed_at=now(). Returns ErrNotFound if no shadow row
// with that id exists (already claimed / not a shadow) so the caller can fall
// through to the duplicate-email path safely.
func (r *UserRepo) ClaimShadow(ctx context.Context, q Querier, userID uuid.UUID, passwordHash string) error {
	tag, err := q.Exec(ctx,
		`UPDATE users
		 SET password_hash = $2, status = 'registered', claimed_at = now()
		 WHERE id = $1 AND status = 'shadow'`,
		userID, passwordHash)
	if err != nil {
		return fmt.Errorf("failed to claim shadow user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByID retrieves a user by ID
func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user := &models.User{}

	query := `
		SELECT id, email, password_hash, name, status, claimed_at, dob, sex, created_at
		FROM users
		WHERE id = $1
	`

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.Status,
		&user.ClaimedAt,
		&user.DOB,
		&user.Sex,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}

	return user, nil
}

// GetByEmail retrieves a user by email
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}

	query := `
		SELECT id, email, password_hash, name, status, claimed_at, dob, sex, created_at
		FROM users
		WHERE email = $1
	`

	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.Status,
		&user.ClaimedAt,
		&user.DOB,
		&user.Sex,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return user, nil
}

// UpdatePassword replaces a user's password hash. Returns ErrNotFound if the user
// row is gone (should not happen for an authenticated caller).
func (r *UserRepo) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash = $2 WHERE id = $1`, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// isDuplicateKeyError checks if the error is a duplicate key violation
func isDuplicateKeyError(err error) bool {
	return err != nil && (contains(err.Error(), "duplicate key") || contains(err.Error(), "23505"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
