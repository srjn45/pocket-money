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

// LoanRepo handles database operations for loans.
type LoanRepo struct {
	pool *pgxpool.Pool
}

// NewLoanRepo creates a new LoanRepo.
func NewLoanRepo(pool *pgxpool.Pool) *LoanRepo {
	return &LoanRepo{pool: pool}
}

const selectLoanColumns = `
	id, group_id, user_id, principal, installments, emi_amount,
	start_period, status, note, requested_at, decided_by, decided_at`

func scanLoan(row rowScanner) (*models.Loan, error) {
	loan := &models.Loan{}
	var status string
	err := row.Scan(
		&loan.ID,
		&loan.GroupID,
		&loan.UserID,
		&loan.Principal,
		&loan.Installments,
		&loan.EMIAmount,
		&loan.StartPeriod,
		&status,
		&loan.Note,
		&loan.RequestedAt,
		&loan.DecidedBy,
		&loan.DecidedAt,
	)
	if err != nil {
		return nil, err
	}
	loan.Status = models.LoanStatus(status)
	return loan, nil
}

// Create inserts a new loan row. Used for both member requests (status=requested,
// startPeriod=nil) and head pre-approved loans (status=active, startPeriod set).
func (r *LoanRepo) Create(ctx context.Context,
	groupID, userID uuid.UUID,
	principal int64, installments int, emiAmount int64,
	status models.LoanStatus, startPeriod *string, note *string,
	decidedBy *uuid.UUID, decidedAt *time.Time) (*models.Loan, error) {

	query := `
		INSERT INTO loans
			(group_id, user_id, principal, installments, emi_amount,
			 status, start_period, note, decided_by, decided_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING ` + selectLoanColumns

	loan, err := scanLoan(r.pool.QueryRow(ctx, query,
		groupID, userID, principal, installments, emiAmount,
		status, startPeriod, note, decidedBy, decidedAt,
	))
	if err != nil {
		return nil, fmt.Errorf("failed to create loan: %w", err)
	}
	return loan, nil
}

// GetByID retrieves a loan by ID. Returns ErrNotFound on miss.
func (r *LoanRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Loan, error) {
	query := `SELECT ` + selectLoanColumns + ` FROM loans WHERE id = $1`
	loan, err := scanLoan(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get loan by id: %w", err)
	}
	return loan, nil
}

// LoanWithProgress embeds Loan with computed installments_posted and outstanding.
type LoanWithProgress struct {
	models.Loan
	InstallmentsPosted int   `json:"installments_posted"`
	Outstanding        int64 `json:"outstanding"`
}

// ListForGroup returns loans for the group with computed progress fields.
// Head sees all; member sees own (enforced by caller passing userID filter).
// Optional filters: userID and/or status are appended if non-nil.
func (r *LoanRepo) ListForGroup(ctx context.Context, groupID uuid.UUID,
	userID *uuid.UUID, status *models.LoanStatus) ([]LoanWithProgress, error) {

	query := `
		SELECT l.id, l.group_id, l.user_id, l.principal, l.installments, l.emi_amount,
		       l.start_period, l.status, l.note, l.requested_at, l.decided_by, l.decided_at,
		       COALESCE(e.posted_count, 0)                 AS installments_posted,
		       (l.principal - COALESCE(e.paid, 0))::bigint AS outstanding
		FROM loans l
		LEFT JOIN (
		    SELECT loan_id, COUNT(*) AS posted_count, SUM(amount)::bigint AS paid
		    FROM ledger_entries
		    WHERE entry_type = 'emi' AND loan_id IS NOT NULL
		    GROUP BY loan_id
		) e ON e.loan_id = l.id
		WHERE l.group_id = $1`
	args := []interface{}{groupID}
	n := 2

	if userID != nil {
		query += fmt.Sprintf(" AND l.user_id = $%d", n)
		args = append(args, *userID)
		n++
	}
	if status != nil {
		query += fmt.Sprintf(" AND l.status = $%d", n)
		args = append(args, *status)
		// n++ -- not needed after last append
	}
	query += " ORDER BY l.requested_at DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list loans: %w", err)
	}
	defer rows.Close()

	var result []LoanWithProgress
	for rows.Next() {
		var lwp LoanWithProgress
		var statusStr string
		err := rows.Scan(
			&lwp.ID, &lwp.GroupID, &lwp.UserID,
			&lwp.Principal, &lwp.Installments, &lwp.EMIAmount,
			&lwp.StartPeriod, &statusStr, &lwp.Note,
			&lwp.RequestedAt, &lwp.DecidedBy, &lwp.DecidedAt,
			&lwp.InstallmentsPosted, &lwp.Outstanding,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan loan with progress: %w", err)
		}
		lwp.Status = models.LoanStatus(statusStr)
		result = append(result, lwp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate loan rows: %w", err)
	}
	if result == nil {
		result = []LoanWithProgress{}
	}
	return result, nil
}

// ListActiveLoans returns all active loans for a group in deterministic order.
// Order BY user_id, start_period, id is load-bearing: concurrent PostDue calls
// must lock loans in identical order to avoid deadlocks (§4.2).
func (r *LoanRepo) ListActiveLoans(ctx context.Context, groupID uuid.UUID) ([]models.LoanPostingInput, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, principal, installments, emi_amount, start_period
		FROM loans
		WHERE group_id = $1 AND status = 'active'
		ORDER BY user_id, start_period, id`,
		groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to list active loans: %w", err)
	}
	defer rows.Close()

	var result []models.LoanPostingInput
	for rows.Next() {
		var inp models.LoanPostingInput
		if err := rows.Scan(&inp.LoanID, &inp.UserID, &inp.Principal, &inp.Installments, &inp.EMIAmount, &inp.StartPeriod); err != nil {
			return nil, fmt.Errorf("failed to scan active loan: %w", err)
		}
		result = append(result, inp)
	}
	return result, nil
}

// PostedEMIPeriods returns the set of already-posted EMI (loan_id, period) pairs
// for the group, keyed by loan_id then period. Used as a fast-path guard in
// PostDue to skip loans that are already fully caught up.
func (r *LoanRepo) PostedEMIPeriods(ctx context.Context, groupID uuid.UUID) (map[uuid.UUID]map[string]bool, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT loan_id, period FROM ledger_entries
		WHERE group_id = $1 AND entry_type = 'emi' AND loan_id IS NOT NULL AND period IS NOT NULL`,
		groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to query posted emi periods: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]map[string]bool)
	for rows.Next() {
		var loanID uuid.UUID
		var period string
		if err := rows.Scan(&loanID, &period); err != nil {
			return nil, fmt.Errorf("failed to scan posted emi period: %w", err)
		}
		if result[loanID] == nil {
			result[loanID] = make(map[string]bool)
		}
		result[loanID][period] = true
	}
	return result, nil
}

// LockActiveLoan acquires a FOR UPDATE row lock on the loan and returns whether
// it is currently active. Used by PostDue and the close endpoint to serialize
// concurrent writers (§4.5). pgx.ErrNoRows returns (false, nil) — loan vanished.
func (r *LoanRepo) LockActiveLoan(ctx context.Context, q Querier, id uuid.UUID) (bool, error) {
	var status string
	err := q.QueryRow(ctx, `SELECT status FROM loans WHERE id = $1 FOR UPDATE`, id).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to lock loan %s: %w", id, err)
	}
	return status == string(models.LoanStatusActive), nil
}

// CountPostedEMIs counts committed EMI entries for a loan, run on the tx querier
// so the count is authoritative under the FOR UPDATE lock (§4.5).
func (r *LoanRepo) CountPostedEMIs(ctx context.Context, q Querier, loanID uuid.UUID) (int, error) {
	var count int
	err := q.QueryRow(ctx,
		`SELECT COUNT(*) FROM ledger_entries WHERE loan_id = $1 AND entry_type = 'emi'`,
		loanID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count posted emis for loan %s: %w", loanID, err)
	}
	return count, nil
}

// SumPostedEMIs sums committed EMI amounts for a loan, run on the tx querier.
// The ::bigint cast is required: SUM(bigint) returns numeric in pgx (WP-1.1 gotcha).
func (r *LoanRepo) SumPostedEMIs(ctx context.Context, q Querier, loanID uuid.UUID) (int64, error) {
	var total int64
	err := q.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0)::bigint FROM ledger_entries WHERE loan_id = $1 AND entry_type = 'emi'`,
		loanID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum posted emis for loan %s: %w", loanID, err)
	}
	return total, nil
}

// CloseLoan sets the loan status to closed. Idempotent: 0 rows affected is fine.
// Must run on the tx querier so it is serialized with the FOR UPDATE lock (§4.5).
func (r *LoanRepo) CloseLoan(ctx context.Context, q Querier, loanID uuid.UUID) error {
	_, err := q.Exec(ctx,
		`UPDATE loans SET status = 'closed' WHERE id = $1 AND status = 'active'`,
		loanID)
	if err != nil {
		return fmt.Errorf("failed to close loan %s: %w", loanID, err)
	}
	return nil
}

// Approve transitions a loan from requested to active. Returns ErrNotFound when
// the loan does not exist or its status is not 'requested' (caller maps to 409).
func (r *LoanRepo) Approve(ctx context.Context, q Querier,
	id uuid.UUID, principal int64, installments int, emiAmount int64,
	startPeriod string, decidedBy uuid.UUID, decidedAt time.Time) (*models.Loan, error) {

	query := `
		UPDATE loans
		SET principal = $2, installments = $3, emi_amount = $4,
		    start_period = $5, status = 'active', decided_by = $6, decided_at = $7
		WHERE id = $1 AND status = 'requested'
		RETURNING ` + selectLoanColumns

	loan, err := scanLoan(q.QueryRow(ctx, query,
		id, principal, installments, emiAmount, startPeriod, decidedBy, decidedAt))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to approve loan %s: %w", id, err)
	}
	return loan, nil
}

// Reject transitions a loan from requested to rejected. Returns ErrNotFound when
// the loan does not exist or its status is not 'requested' (caller maps to 409).
func (r *LoanRepo) Reject(ctx context.Context, q Querier,
	id, decidedBy uuid.UUID, decidedAt time.Time) (*models.Loan, error) {

	query := `
		UPDATE loans
		SET status = 'rejected', decided_by = $2, decided_at = $3
		WHERE id = $1 AND status = 'requested'
		RETURNING ` + selectLoanColumns

	loan, err := scanLoan(q.QueryRow(ctx, query, id, decidedBy, decidedAt))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to reject loan %s: %w", id, err)
	}
	return loan, nil
}

// LockBlockingLoans locks the member's non-terminal loans (requested|active) FOR
// UPDATE and returns how many there are. The lock serializes against a concurrent
// PostDue that would FOR UPDATE the same active loans before posting EMIs (WP-3.1).
// Ordered by (start_period, id) to match PostDue's per-user acquisition order
// (ListActiveLoans + LockActiveLoan iterate user_id, start_period, id): a member
// may hold >1 active loan, so locking those shared rows in the same relative order
// as PostDue is what prevents a within-user lock cycle. Requested loans carry a
// NULL start_period (sorted last) and are never locked by PostDue, so they cannot
// participate in a cycle.
func (r *LoanRepo) LockBlockingLoans(ctx context.Context, q Querier, groupID, userID uuid.UUID) (int, error) {
	rows, err := q.Query(ctx,
		`SELECT id FROM loans
		 WHERE group_id = $1 AND user_id = $2 AND status IN ('requested','active')
		 ORDER BY start_period, id
		 FOR UPDATE`, groupID, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to lock blocking loans: %w", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
	}
	return n, rows.Err()
}
