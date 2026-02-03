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
	var query string
	var args []interface{}

	if status != nil {
		query = `
			SELECT id, group_id, user_id, chore_id, amount, status, created_by_user_id, approved_by_user_id, rejected_by_user_id, created_at
			FROM ledger_entries
			WHERE group_id = $1 AND status = $2
			ORDER BY created_at DESC
		`
		args = []interface{}{groupID, *status}
	} else {
		query = `
			SELECT id, group_id, user_id, chore_id, amount, status, created_by_user_id, approved_by_user_id, rejected_by_user_id, created_at
			FROM ledger_entries
			WHERE group_id = $1
			ORDER BY created_at DESC
		`
		args = []interface{}{groupID}
	}

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
// Balance = sum(approved ledger entries) - sum(settlements)
func (r *LedgerRepo) GetBalanceForGroup(ctx context.Context, groupID uuid.UUID) ([]*models.Balance, error) {
	query := `
		WITH ledger_totals AS (
			SELECT user_id, COALESCE(SUM(amount), 0) as total
			FROM ledger_entries
			WHERE group_id = $1 AND status = 'approved'
			GROUP BY user_id
		),
		settlement_totals AS (
			SELECT user_id, COALESCE(SUM(amount), 0) as total
			FROM settlements
			WHERE group_id = $1
			GROUP BY user_id
		),
		all_members AS (
			SELECT gm.user_id, u.name
			FROM group_members gm
			INNER JOIN users u ON gm.user_id = u.id
			WHERE gm.group_id = $1
		)
		SELECT 
			am.user_id, 
			am.name,
			COALESCE(lt.total, 0) - COALESCE(st.total, 0) as balance
		FROM all_members am
		LEFT JOIN ledger_totals lt ON am.user_id = lt.user_id
		LEFT JOIN settlement_totals st ON am.user_id = st.user_id
		ORDER BY am.name
	`

	rows, err := r.pool.Query(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}
	defer rows.Close()

	var balances []*models.Balance
	for rows.Next() {
		balance := &models.Balance{}
		if err := rows.Scan(&balance.UserID, &balance.Name, &balance.Balance); err != nil {
			return nil, fmt.Errorf("failed to scan balance: %w", err)
		}
		balances = append(balances, balance)
	}

	return balances, nil
}
