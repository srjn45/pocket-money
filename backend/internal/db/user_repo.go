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

// Create inserts a new user into the database
func (r *UserRepo) Create(ctx context.Context, email, passwordHash, name string, dob *string, sex *string) (*models.User, error) {
	user := &models.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: passwordHash,
		Name:         name,
	}

	query := `
		INSERT INTO users (id, email, password_hash, name, dob, sex)
		VALUES ($1, $2, $3, $4, $5, $6)
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

// GetByID retrieves a user by ID
func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user := &models.User{}

	query := `
		SELECT id, email, password_hash, name, dob, sex, created_at
		FROM users
		WHERE id = $1
	`

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
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
		SELECT id, email, password_hash, name, dob, sex, created_at
		FROM users
		WHERE email = $1
	`

	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
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
