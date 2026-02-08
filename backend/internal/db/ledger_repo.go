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

// LedgerRepo handles database operations for ledger entries
type LedgerRepo struct {
	pool *pgxpool.Pool
}

// NewLedgerRepo creates a new LedgerRepo
func NewLedgerRepo(pool *pgxpool.Pool) *LedgerRepo {
	return &LedgerRepo{pool: pool}
}

// Create inserts a new ledger entry
func (r *LedgerRepo) Create(ctx context.Context, groupID, userID, choreID, createdByUserID uuid.UUID, amount float64, status models.LedgerStatus, approvedByUserID *uuid.UUID) (*models.LedgerEntry, error) {
	entry := &models.LedgerEntry{
		ID:               uuid.New(),
		GroupID:          groupID,
		UserID:           userID,
		ChoreID:          choreID,
		Amount:           amount,
		Status:           status,
		CreatedByUserID:  createdByUserID,
		ApprovedByUserID: approvedByUserID,
	}

	query := `
		INSERT INTO ledger_entries (id, group_id, user_id, chore_id, amount, status, created_by_user_id, approved_by_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at
	`

	err := r.pool.QueryRow(ctx, query,
		entry.ID, groupID, userID, choreID, amount, status, createdByUserID, approvedByUserID,
	).Scan(&entry.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create ledger entry: %w", err)
	}

	return entry, nil
}

// GetByID retrieves a ledger entry by ID
func (r *LedgerRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.LedgerEntry, error) {
	entry := &models.LedgerEntry{}

	query := `
		SELECT id, group_id, user_id, chore_id, amount, status, created_by_user_id, approved_by_user_id, rejected_by_user_id, created_at
		FROM ledger_entries
		WHERE id = $1
	`

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&entry.ID,
		&entry.GroupID,
		&entry.UserID,
		&entry.ChoreID,
		&entry.Amount,
		&entry.Status,
		&entry.CreatedByUserID,
		&entry.ApprovedByUserID,
		&entry.RejectedByUserID,
		&entry.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get ledger entry by id: %w", err)
	}

	return entry, nil
}

// ListForGroup retrieves all ledger entries for a group with optional status filter
func (r *LedgerRepo) ListForGroup(ctx context.Context, groupID uuid.UUID, status *models.LedgerStatus) ([]*models.LedgerEntry, error) {
	return r.ListForGroupWithUser(ctx, groupID, status, nil)
}

// ListForGroupWithUser retrieves ledger entries for a group with optional status and user filters
func (r *LedgerRepo) ListForGroupWithUser(ctx context.Context, groupID uuid.UUID, status *models.LedgerStatus, userID *uuid.UUID) ([]*models.LedgerEntry, error) {
	query := `
		SELECT id, group_id, user_id, chore_id, amount, status, created_by_user_id, approved_by_user_id, rejected_by_user_id, created_at
		FROM ledger_entries
		WHERE group_id = $1
	`
	args := []interface{}{groupID}
	argNum := 2

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, *status)
		argNum++
	}

	if userID != nil {
		query += fmt.Sprintf(" AND user_id = $%d", argNum)
		args = append(args, *userID)
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list ledger entries: %w", err)
	}
	defer rows.Close()

	var entries []*models.LedgerEntry
	for rows.Next() {
		entry := &models.LedgerEntry{}
		if err := rows.Scan(
			&entry.ID,
			&entry.GroupID,
			&entry.UserID,
			&entry.ChoreID,
			&entry.Amount,
			&entry.Status,
			&entry.CreatedByUserID,
			&entry.ApprovedByUserID,
			&entry.RejectedByUserID,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ledger entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// UpdateStatus updates the status of a ledger entry
func (r *LedgerRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status models.LedgerStatus, approvedByUserID, rejectedByUserID *uuid.UUID) (*models.LedgerEntry, error) {
	query := `
		UPDATE ledger_entries
		SET status = $2, approved_by_user_id = $3, rejected_by_user_id = $4
		WHERE id = $1
		RETURNING id, group_id, user_id, chore_id, amount, status, created_by_user_id, approved_by_user_id, rejected_by_user_id, created_at
	`

	entry := &models.LedgerEntry{}
	err := r.pool.QueryRow(ctx, query, id, status, approvedByUserID, rejectedByUserID).Scan(
		&entry.ID,
		&entry.GroupID,
		&entry.UserID,
		&entry.ChoreID,
		&entry.Amount,
		&entry.Status,
		&entry.CreatedByUserID,
		&entry.ApprovedByUserID,
		&entry.RejectedByUserID,
		&entry.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to update ledger entry status: %w", err)
	}

	return entry, nil
}

// GetBalanceForGroup calculates the balance for each member in a group
// Balance = sum(approved regular chore entries) - sum(approved settlement entries)
// Settlement entries are identified by chore.is_system = true
func (r *LedgerRepo) GetBalanceForGroup(ctx context.Context, groupID uuid.UUID) ([]*models.Balance, error) {
	query := `
		WITH ledger_totals AS (
			SELECT 
				le.user_id,
				COALESCE(SUM(CASE WHEN c.is_system = false THEN le.amount ELSE 0 END), 0) as earned,
				COALESCE(SUM(CASE WHEN c.is_system = true THEN le.amount ELSE 0 END), 0) as settled
			FROM ledger_entries le
			INNER JOIN chores c ON le.chore_id = c.id
			WHERE le.group_id = $1 AND le.status = 'approved'
			GROUP BY le.user_id
		),
		all_members AS (
			SELECT gm.user_id, u.name, gm.role
			FROM group_members gm
			INNER JOIN users u ON gm.user_id = u.id
			WHERE gm.group_id = $1
		)
		SELECT 
			am.user_id, 
			am.name,
			am.role,
			COALESCE(lt.earned, 0) - COALESCE(lt.settled, 0) as balance
		FROM all_members am
		LEFT JOIN ledger_totals lt ON am.user_id = lt.user_id
		ORDER BY am.role DESC, am.name
	`

	rows, err := r.pool.Query(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}
	defer rows.Close()

	var balances []*models.Balance
	for rows.Next() {
		balance := &models.Balance{}
		var role string
		if err := rows.Scan(&balance.UserID, &balance.Name, &role, &balance.Balance); err != nil {
			return nil, fmt.Errorf("failed to scan balance: %w", err)
		}
		// Skip head user from balance list (head doesn't have balance)
		if role != "head" {
			balances = append(balances, balance)
		}
	}

	return balances, nil
}
