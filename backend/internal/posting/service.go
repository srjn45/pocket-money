package posting

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
)

// Service wraps PostDue with a concrete Store built from real repos.
type Service struct {
	store Store
}

// NewService builds a Service from real repos. The returned service is safe
// for concurrent use (each PostDue call opens its own transaction).
func NewService(allowanceRepo *db.AllowanceRepo, ledgerRepo *db.LedgerRepo, loanRepo *db.LoanRepo, groupRepo *db.GroupRepo, pool *pgxpool.Pool) *Service {
	return &Service{store: &storeAdapter{
		allowanceRepo: allowanceRepo,
		ledgerRepo:    ledgerRepo,
		loanRepo:      loanRepo,
		groupRepo:     groupRepo,
		pool:          pool,
	}}
}

// PostDue triggers due allowance and EMI posting for groupID as of now.
func (s *Service) PostDue(ctx context.Context, groupID uuid.UUID, now time.Time) error {
	return PostDue(ctx, s.store, groupID, now)
}

// storeAdapter is the concrete implementation of Store backed by real repos.
type storeAdapter struct {
	allowanceRepo *db.AllowanceRepo
	ledgerRepo    *db.LedgerRepo
	loanRepo      *db.LoanRepo
	groupRepo     *db.GroupRepo
	pool          *pgxpool.Pool
}

func (s *storeAdapter) ListPostingInputs(ctx context.Context, groupID uuid.UUID) ([]models.AllowancePostingInput, error) {
	return s.allowanceRepo.ListPostingInputs(ctx, groupID)
}

func (s *storeAdapter) PostedAllowancePeriods(ctx context.Context, groupID uuid.UUID) (map[uuid.UUID]map[string]bool, error) {
	return s.ledgerRepo.PostedAllowancePeriods(ctx, groupID)
}

func (s *storeAdapter) GroupHead(ctx context.Context, groupID uuid.UUID) (uuid.UUID, error) {
	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get group head: %w", err)
	}
	return group.HeadUserID, nil
}

func (s *storeAdapter) WithTx(ctx context.Context, fn func(q db.Querier) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (s *storeAdapter) InsertAllowancePosting(ctx context.Context, q db.Querier,
	groupID, userID uuid.UUID, amount int64, period string, createdBy uuid.UUID) (bool, error) {
	return s.ledgerRepo.InsertAllowancePosting(ctx, q, groupID, userID, amount, period, createdBy)
}

func (s *storeAdapter) ListActiveLoans(ctx context.Context, groupID uuid.UUID) ([]models.LoanPostingInput, error) {
	return s.loanRepo.ListActiveLoans(ctx, groupID)
}

func (s *storeAdapter) PostedEMIPeriods(ctx context.Context, groupID uuid.UUID) (map[uuid.UUID]map[string]bool, error) {
	return s.loanRepo.PostedEMIPeriods(ctx, groupID)
}

func (s *storeAdapter) LockActiveLoan(ctx context.Context, q db.Querier, loanID uuid.UUID) (bool, error) {
	return s.loanRepo.LockActiveLoan(ctx, q, loanID)
}

func (s *storeAdapter) InsertEMIPosting(ctx context.Context, q db.Querier,
	groupID, userID, loanID uuid.UUID,
	amount int64, period string, note *string, createdBy uuid.UUID) (bool, error) {
	return s.ledgerRepo.InsertEMIPosting(ctx, q, groupID, userID, loanID, amount, period, note, createdBy)
}

func (s *storeAdapter) CountPostedEMIs(ctx context.Context, q db.Querier, loanID uuid.UUID) (int, error) {
	return s.loanRepo.CountPostedEMIs(ctx, q, loanID)
}

func (s *storeAdapter) CloseLoan(ctx context.Context, q db.Querier, loanID uuid.UUID) error {
	return s.loanRepo.CloseLoan(ctx, q, loanID)
}
