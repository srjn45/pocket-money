package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srjn45/pocket-money/backend/internal/models"
)

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx, allowing repo methods
// to be called within or outside a transaction. Query is needed for
// FOR UPDATE multi-row selects (WP-4.7 loan lock); QueryRow for FOR UPDATE
// single-row reads and SUM reads on the EMI posting path (WP-3.1).
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// LedgerRepo handles database operations for ledger entries
type LedgerRepo struct {
	pool *pgxpool.Pool
}

// NewLedgerRepo creates a new LedgerRepo
func NewLedgerRepo(pool *pgxpool.Pool) *LedgerRepo {
	return &LedgerRepo{pool: pool}
}

// rowScanner is satisfied by both *pgx.Row (from QueryRow) and pgx.Rows (from Query).
type rowScanner interface {
	Scan(dest ...any) error
}

// scanEntry scans into a LedgerEntry in the canonical column order used by all SELECT queries.
// Column order: id, group_id, user_id, chore_id, amount, status, entry_type, direction,
//
//	loan_id, period, note, created_by_user_id, decided_by, decided_at, created_at
func scanEntry(row rowScanner) (*models.LedgerEntry, error) {
	entry := &models.LedgerEntry{}
	err := row.Scan(
		&entry.ID,
		&entry.GroupID,
		&entry.UserID,
		&entry.ChoreID,
		&entry.Amount,
		&entry.Status,
		&entry.EntryType,
		&entry.Direction,
		&entry.LoanID,
		&entry.Period,
		&entry.Note,
		&entry.CreatedByUserID,
		&entry.DecidedBy,
		&entry.DecidedAt,
		&entry.CreatedAt,
	)
	return entry, err
}

const selectLedgerColumns = `
	id, group_id, user_id, chore_id, amount, status, entry_type, direction,
	loan_id, period, note, created_by_user_id, decided_by, decided_at, created_at`

// createTx is the shared insert. occurredAt overrides the entry's created_at
// (its effective date, used for month grouping); nil falls back to the DB now().
// period and loan_id are always NULL on the API-create path.
func (r *LedgerRepo) createTx(ctx context.Context, q Querier, groupID, userID uuid.UUID,
	choreID *uuid.UUID, createdBy uuid.UUID, amount int64, entryType models.LedgerEntryType,
	direction models.LedgerDirection, status models.LedgerStatus, note *string,
	decidedBy *uuid.UUID, decidedAt *time.Time, occurredAt *time.Time) (*models.LedgerEntry, error) {

	query := `
		INSERT INTO ledger_entries
			(id, group_id, user_id, chore_id, amount, status, entry_type, direction, note,
			 created_by_user_id, decided_by, decided_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, COALESCE($13, now()))
		RETURNING ` + selectLedgerColumns

	entry, err := scanEntry(q.QueryRow(ctx, query,
		uuid.New(), groupID, userID, choreID, amount, status, entryType, direction, note,
		createdBy, decidedBy, decidedAt, occurredAt,
	))
	if err != nil {
		return nil, fmt.Errorf("failed to create ledger entry: %w", err)
	}
	return entry, nil
}

// CreateTx inserts a new ledger entry on the given Querier (pool or tx), dated now().
func (r *LedgerRepo) CreateTx(ctx context.Context, q Querier, groupID, userID uuid.UUID,
	choreID *uuid.UUID, createdBy uuid.UUID, amount int64, entryType models.LedgerEntryType,
	direction models.LedgerDirection, status models.LedgerStatus, note *string,
	decidedBy *uuid.UUID, decidedAt *time.Time) (*models.LedgerEntry, error) {
	return r.createTx(ctx, q, groupID, userID, choreID, createdBy, amount, entryType,
		direction, status, note, decidedBy, decidedAt, nil)
}

// CreateAtTx is CreateTx with an explicit occurred date (created_at override).
func (r *LedgerRepo) CreateAtTx(ctx context.Context, q Querier, groupID, userID uuid.UUID,
	choreID *uuid.UUID, createdBy uuid.UUID, amount int64, entryType models.LedgerEntryType,
	direction models.LedgerDirection, status models.LedgerStatus, note *string,
	decidedBy *uuid.UUID, decidedAt *time.Time, occurredAt *time.Time) (*models.LedgerEntry, error) {
	return r.createTx(ctx, q, groupID, userID, choreID, createdBy, amount, entryType,
		direction, status, note, decidedBy, decidedAt, occurredAt)
}

// Create inserts a new ledger entry on the pool (non-transactional convenience wrapper).
// period and loan_id are always NULL on the API-create path (set by posting engine in WP-2.1/3.1).
func (r *LedgerRepo) Create(ctx context.Context, groupID, userID uuid.UUID, choreID *uuid.UUID,
	createdBy uuid.UUID, amount int64, entryType models.LedgerEntryType,
	direction models.LedgerDirection, status models.LedgerStatus, note *string,
	decidedBy *uuid.UUID, decidedAt *time.Time) (*models.LedgerEntry, error) {
	return r.createTx(ctx, r.pool, groupID, userID, choreID, createdBy, amount, entryType,
		direction, status, note, decidedBy, decidedAt, nil)
}

// CreateAt is Create with an explicit occurred date (created_at override).
func (r *LedgerRepo) CreateAt(ctx context.Context, groupID, userID uuid.UUID, choreID *uuid.UUID,
	createdBy uuid.UUID, amount int64, entryType models.LedgerEntryType,
	direction models.LedgerDirection, status models.LedgerStatus, note *string,
	decidedBy *uuid.UUID, decidedAt *time.Time, occurredAt *time.Time) (*models.LedgerEntry, error) {
	return r.createTx(ctx, r.pool, groupID, userID, choreID, createdBy, amount, entryType,
		direction, status, note, decidedBy, decidedAt, occurredAt)
}

// GetByID retrieves a ledger entry by ID
func (r *LedgerRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.LedgerEntry, error) {
	query := `SELECT ` + selectLedgerColumns + ` FROM ledger_entries WHERE id = $1`

	entry, err := scanEntry(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get ledger entry by id: %w", err)
	}
	return entry, nil
}

// GetForUpdate re-reads a ledger entry FOR UPDATE on the caller's Querier (tx).
// It both locks the row for the edit/delete and is the snapshot source for the
// entry_audit old_row (V3-3.2 §1.4a). ErrNotFound when the row is gone (lost race).
func (r *LedgerRepo) GetForUpdate(ctx context.Context, q Querier, id uuid.UUID) (*models.LedgerEntry, error) {
	query := `SELECT ` + selectLedgerColumns + ` FROM ledger_entries WHERE id = $1 FOR UPDATE`

	entry, err := scanEntry(q.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get ledger entry for update: %w", err)
	}
	return entry, nil
}

// UpdateManualEntry mutates only the editable fields of a manual entry (V3-3.2
// §1.2): amount (always), direction (only when non-nil — adjustments), and note
// (full-replace: a nil note clears it to NULL). entry_type/user_id/chore_id/
// created_at/period/status stay immutable, which keeps the edited entry in the
// same effMonth (invariant-safe, §5). Runs on the caller's Querier so it joins
// the correction tx alongside the audit insert.
func (r *LedgerRepo) UpdateManualEntry(ctx context.Context, q Querier, id uuid.UUID,
	amount int64, direction *models.LedgerDirection, note *string) (*models.LedgerEntry, error) {

	query := `
		UPDATE ledger_entries
		SET amount = $2,
		    direction = COALESCE($3::ledger_direction, direction),
		    note = $4
		WHERE id = $1
		RETURNING ` + selectLedgerColumns

	entry, err := scanEntry(q.QueryRow(ctx, query, id, amount, direction, note))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to update manual ledger entry: %w", err)
	}
	return entry, nil
}

// DeleteEntry hard-deletes a ledger entry (V3-3.2 §0). The entry_audit row's
// entry_id FK is ON DELETE SET NULL, so the audit snapshot survives the delete.
// Runs on the caller's Querier so it joins the correction tx after the audit
// insert. ErrNotFound when the row is already gone.
func (r *LedgerRepo) DeleteEntry(ctx context.Context, q Querier, id uuid.UUID) error {
	tag, err := q.Exec(ctx, `DELETE FROM ledger_entries WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete ledger entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListForGroup retrieves all ledger entries for a group with optional status filter
func (r *LedgerRepo) ListForGroup(ctx context.Context, groupID uuid.UUID, status *models.LedgerStatus) ([]*models.LedgerEntry, error) {
	return r.ListForGroupWithUser(ctx, groupID, status, nil, nil, nil)
}

// ListForGroupWithUser retrieves ledger entries for a group with optional filters.
// Members see only their own entries (caller enforces by passing filterUserID = self).
func (r *LedgerRepo) ListForGroupWithUser(ctx context.Context, groupID uuid.UUID,
	status *models.LedgerStatus, userID *uuid.UUID,
	entryType *models.LedgerEntryType, period *string) ([]*models.LedgerEntry, error) {

	query := `SELECT ` + selectLedgerColumns + ` FROM ledger_entries WHERE group_id = $1`
	args := []interface{}{groupID}
	n := 2

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", n)
		args = append(args, *status)
		n++
	}
	if userID != nil {
		query += fmt.Sprintf(" AND user_id = $%d", n)
		args = append(args, *userID)
		n++
	}
	if entryType != nil {
		query += fmt.Sprintf(" AND entry_type = $%d", n)
		args = append(args, *entryType)
		n++
	}
	if period != nil {
		query += fmt.Sprintf(" AND period = $%d", n)
		args = append(args, *period)
		// n++ — not needed after last append
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list ledger entries: %w", err)
	}
	defer rows.Close()

	var entries []*models.LedgerEntry
	for rows.Next() {
		entry, err := scanEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ledger entry: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// SetDecision atomically updates a ledger entry's status and records the approver/rejecter.
func (r *LedgerRepo) SetDecision(ctx context.Context, id uuid.UUID, status models.LedgerStatus,
	decidedBy uuid.UUID, decidedAt time.Time) (*models.LedgerEntry, error) {

	query := `
		UPDATE ledger_entries
		SET status = $2, decided_by = $3, decided_at = $4
		WHERE id = $1
		RETURNING ` + selectLedgerColumns

	entry, err := scanEntry(r.pool.QueryRow(ctx, query, id, status, decidedBy, decidedAt))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to set decision on ledger entry: %w", err)
	}
	return entry, nil
}

// InsertAllowancePosting inserts one approved allowance credit for (group,user,period),
// idempotently. Returns (inserted bool) so the engine/tests can assert exactly-once.
// created_by is the group head. decided_by/decided_at stay NULL (machine post).
// loan_id stays NULL so (group,user,'allowance',period,NULL) collides via NULLS NOT DISTINCT.
// The bare ON CONFLICT DO NOTHING catches the partial unique index without restating it.
func (r *LedgerRepo) InsertAllowancePosting(ctx context.Context, q Querier,
	groupID, userID uuid.UUID, amount int64, period string, createdBy uuid.UUID) (bool, error) {

	tag, err := q.Exec(ctx, `
		INSERT INTO ledger_entries
			(id, group_id, user_id, chore_id, amount, status, entry_type, direction,
			 loan_id, period, note, created_by_user_id, decided_by, decided_at)
		VALUES (gen_random_uuid(), $1, $2, NULL, $3, 'approved', 'allowance', 'credit',
		        NULL, $4, NULL, $5, NULL, NULL)
		ON CONFLICT DO NOTHING`,
		groupID, userID, amount, period, createdBy)
	if err != nil {
		return false, fmt.Errorf("failed to insert allowance posting: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// PostedAllowancePeriods returns, per user, the set of periods already posted as
// allowance entries in this group. Used by the posting engine as a fast-path guard.
func (r *LedgerRepo) PostedAllowancePeriods(ctx context.Context, groupID uuid.UUID) (map[uuid.UUID]map[string]bool, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT user_id, period FROM ledger_entries
		WHERE group_id = $1 AND entry_type = 'allowance' AND period IS NOT NULL`,
		groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to query posted allowance periods: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]map[string]bool)
	for rows.Next() {
		var userID uuid.UUID
		var period string
		if err := rows.Scan(&userID, &period); err != nil {
			return nil, fmt.Errorf("failed to scan posted period: %w", err)
		}
		if result[userID] == nil {
			result[userID] = make(map[string]bool)
		}
		result[userID][period] = true
	}
	return result, nil
}

// InsertEMIPosting inserts one approved emi debit for (group,user,loan,period), idempotently.
// Returns (inserted bool) so the engine/close path can assert exactly-once.
// loan_id AND period are both set, so the row collides on
// (group,user,'emi',period,loan_id) — distinct per loan, distinct from allowance
// rows (different entry_type) and from other loans' EMIs (different loan_id).
// The bare ON CONFLICT DO NOTHING catches the partial unique index without restating it.
// Invariant: loan_id and period must both be non-NULL or the index won't guard them.
func (r *LedgerRepo) InsertEMIPosting(ctx context.Context, q Querier,
	groupID, userID, loanID uuid.UUID, amount int64, period string,
	note *string, createdBy uuid.UUID) (bool, error) {

	tag, err := q.Exec(ctx, `
		INSERT INTO ledger_entries
			(id, group_id, user_id, chore_id, amount, status, entry_type, direction,
			 loan_id, period, note, created_by_user_id, decided_by, decided_at)
		VALUES (gen_random_uuid(), $1, $2, NULL, $3, 'approved', 'emi', 'debit',
		        $4, $5, $6, $7, NULL, NULL)
		ON CONFLICT DO NOTHING`,
		groupID, userID, amount, loanID, period, note, createdBy)
	if err != nil {
		return false, fmt.Errorf("failed to insert emi posting: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// GetBalanceForGroup calculates the balance for each member in a group.
// Balance = Σ approved credits − Σ approved debits, computed from direction.
// No chores join: settlement rows (chore_id NULL) are counted correctly.
func (r *LedgerRepo) GetBalanceForGroup(ctx context.Context, groupID uuid.UUID) ([]*models.Balance, error) {
	query := `
		WITH ledger_totals AS (
			SELECT le.user_id,
				COALESCE(SUM(CASE WHEN le.direction = 'credit' THEN le.amount ELSE 0 END), 0)::bigint AS credits,
				COALESCE(SUM(CASE WHEN le.direction = 'debit'  THEN le.amount ELSE 0 END), 0)::bigint AS debits
			FROM ledger_entries le
			WHERE le.group_id = $1 AND le.status = 'approved'
			GROUP BY le.user_id
		),
		all_members AS (
			SELECT gm.user_id, u.name, gm.role
			FROM group_members gm JOIN users u ON gm.user_id = u.id
			WHERE gm.group_id = $1
		)
		SELECT am.user_id, am.name, am.role,
			(COALESCE(lt.credits, 0) - COALESCE(lt.debits, 0))::bigint AS balance
		FROM all_members am LEFT JOIN ledger_totals lt ON am.user_id = lt.user_id
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
		if role != "admin" {
			balances = append(balances, balance)
		}
	}
	return balances, nil
}

// MemberBalanceTx computes one member's balance (Σ approved credits − Σ approved
// debits, ::bigint) on the tx querier — identical math to GetBalanceForGroup.
func (r *LedgerRepo) MemberBalanceTx(ctx context.Context, q Querier, groupID, userID uuid.UUID) (int64, error) {
	var balance int64
	err := q.QueryRow(ctx, `
		SELECT (COALESCE(SUM(CASE WHEN direction = 'credit' THEN amount ELSE 0 END), 0)
		      - COALESCE(SUM(CASE WHEN direction = 'debit'  THEN amount ELSE 0 END), 0))::bigint
		FROM ledger_entries
		WHERE group_id = $1 AND user_id = $2 AND status = 'approved'`,
		groupID, userID).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("failed to compute member balance: %w", err)
	}
	return balance, nil
}

// RejectPendingForMember flips all of a member's pending_approval entries to
// rejected in this group, recording the actor. Pending entries never counted toward
// balance, so this does not change any total; it prevents a post-removal "ghost
// approval" (D7). Returns rows affected (informational).
func (r *LedgerRepo) RejectPendingForMember(ctx context.Context, q Querier, groupID, userID, decidedBy uuid.UUID, decidedAt time.Time) (int64, error) {
	tag, err := q.Exec(ctx, `
		UPDATE ledger_entries
		SET status = 'rejected', decided_by = $3, decided_at = $4
		WHERE group_id = $1 AND user_id = $2 AND status = 'pending_approval'`,
		groupID, userID, decidedBy, decidedAt)
	if err != nil {
		return 0, fmt.Errorf("failed to reject pending entries: %w", err)
	}
	return tag.RowsAffected(), nil
}
