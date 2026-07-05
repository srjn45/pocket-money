package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srjn45/pocket-money/backend/internal/auth"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
	"github.com/srjn45/pocket-money/backend/internal/posting"
)

// LoanHandler handles loan lifecycle requests.
type LoanHandler struct {
	loanRepo   *db.LoanRepo
	ledgerRepo *db.LedgerRepo
	groupRepo  *db.GroupRepo
	pool       *pgxpool.Pool
}

// NewLoanHandler creates a new LoanHandler.
func NewLoanHandler(loanRepo *db.LoanRepo, ledgerRepo *db.LedgerRepo, groupRepo *db.GroupRepo, pool *pgxpool.Pool) *LoanHandler {
	return &LoanHandler{
		loanRepo:   loanRepo,
		ledgerRepo: ledgerRepo,
		groupRepo:  groupRepo,
		pool:       pool,
	}
}

// LoanResponse is the API representation of a loan with computed progress.
type LoanResponse struct {
	ID                 uuid.UUID         `json:"id"`
	GroupID            uuid.UUID         `json:"group_id"`
	UserID             uuid.UUID         `json:"user_id"`
	Principal          int64             `json:"principal"`
	Installments       int               `json:"installments"`
	EMIAmount          int64             `json:"emi_amount"`
	StartPeriod        *string           `json:"start_period,omitempty"`
	Status             models.LoanStatus `json:"status"`
	Note               *string           `json:"note,omitempty"`
	RequestedAt        time.Time         `json:"requested_at"`
	DecidedBy          *uuid.UUID        `json:"decided_by,omitempty"`
	DecidedAt          *time.Time        `json:"decided_at,omitempty"`
	InstallmentsPosted int               `json:"installments_posted"`
	Outstanding        int64             `json:"outstanding"`
}

func loanWithProgressToResponse(lwp db.LoanWithProgress) LoanResponse {
	return LoanResponse{
		ID:                 lwp.ID,
		GroupID:            lwp.GroupID,
		UserID:             lwp.UserID,
		Principal:          lwp.Principal,
		Installments:       lwp.Installments,
		EMIAmount:          lwp.EMIAmount,
		StartPeriod:        lwp.StartPeriod,
		Status:             lwp.Status,
		Note:               lwp.Note,
		RequestedAt:        lwp.RequestedAt,
		DecidedBy:          lwp.DecidedBy,
		DecidedAt:          lwp.DecidedAt,
		InstallmentsPosted: lwp.InstallmentsPosted,
		Outstanding:        lwp.Outstanding,
	}
}

func loanToResponse(loan *models.Loan, installmentsPosted int, outstanding int64) LoanResponse {
	return LoanResponse{
		ID:                 loan.ID,
		GroupID:            loan.GroupID,
		UserID:             loan.UserID,
		Principal:          loan.Principal,
		Installments:       loan.Installments,
		EMIAmount:          loan.EMIAmount,
		StartPeriod:        loan.StartPeriod,
		Status:             loan.Status,
		Note:               loan.Note,
		RequestedAt:        loan.RequestedAt,
		DecidedBy:          loan.DecidedBy,
		DecidedAt:          loan.DecidedAt,
		InstallmentsPosted: installmentsPosted,
		Outstanding:        outstanding,
	}
}

// CreateLoanRequest is the body for POST /groups/:id/loans.
type CreateLoanRequest struct {
	UserID       *uuid.UUID `json:"user_id"`
	Principal    int64      `json:"principal"`
	Installments int        `json:"installments"`
	Note         *string    `json:"note"`
}

// ApproveLoanRequest is the body for POST /loans/:id/approve.
type ApproveLoanRequest struct {
	Principal    *int64 `json:"principal"`
	Installments *int   `json:"installments"`
}

// loanCeilDiv returns ceil(a / b) for positive a, b.
func loanCeilDiv(a int64, b int) int64 {
	return (a + int64(b) - 1) / int64(b)
}

// loanNextPeriod returns the YYYY-MM for the calendar month after now.
// Uses integer month arithmetic (posting.AddMonths) to avoid DST surprises.
func loanNextPeriod(now time.Time) string {
	return posting.AddMonths(now.Format("2006-01"), 1)
}

// ListLoans handles GET /api/v1/groups/:id/loans?user_id=&status=
func (h *LoanHandler) ListLoans(c *gin.Context) {
	callerIDStr, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	callerID, err := uuid.Parse(callerIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group ID"})
		return
	}

	caller, err := h.groupRepo.GetMember(c.Request.Context(), groupID, callerID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}

	var filterUserID *uuid.UUID
	var filterStatus *models.LoanStatus

	if caller.Role == models.RoleHead {
		if uidStr := c.Query("user_id"); uidStr != "" {
			uid, err := uuid.Parse(uidStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
				return
			}
			filterUserID = &uid
		}
	} else {
		// Member sees only own loans regardless of any user_id query param.
		filterUserID = &callerID
	}

	if statusStr := c.Query("status"); statusStr != "" {
		s := models.LoanStatus(statusStr)
		switch s {
		case models.LoanStatusRequested, models.LoanStatusActive,
			models.LoanStatusRejected, models.LoanStatusClosed:
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return
		}
		filterStatus = &s
	}

	loans, err := h.loanRepo.ListForGroup(c.Request.Context(), groupID, filterUserID, filterStatus)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list loans"})
		return
	}

	resp := make([]LoanResponse, 0, len(loans))
	for _, lwp := range loans {
		resp = append(resp, loanWithProgressToResponse(lwp))
	}
	c.JSON(http.StatusOK, resp)
}

// CreateLoan handles POST /api/v1/groups/:id/loans
func (h *LoanHandler) CreateLoan(c *gin.Context) {
	callerIDStr, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	callerID, err := uuid.Parse(callerIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group ID"})
		return
	}

	caller, err := h.groupRepo.GetMember(c.Request.Context(), groupID, callerID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}

	var req CreateLoanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Principal <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "principal must be > 0"})
		return
	}
	if req.Installments <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "installments must be > 0"})
		return
	}

	emiAmount := loanCeilDiv(req.Principal, req.Installments)
	now := time.Now()

	var loan *models.Loan

	if caller.Role == models.RoleHead {
		// Head creates a pre-approved active loan for a member.
		if req.UserID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required for head-created loans"})
			return
		}
		target, err := h.groupRepo.GetMember(c.Request.Context(), groupID, *req.UserID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "target user is not a member of this group"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check target membership"})
			return
		}
		if target.Role == models.RoleHead {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot create a loan for the group head"})
			return
		}

		start := loanNextPeriod(now)
		decidedAt := now
		loan, err = h.loanRepo.Create(c.Request.Context(),
			groupID, *req.UserID,
			req.Principal, req.Installments, emiAmount,
			models.LoanStatusActive, &start, req.Note,
			&callerID, &decidedAt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create loan"})
			return
		}
	} else {
		// Member creates a loan request for themselves.
		if req.UserID != nil && *req.UserID != callerID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "members can only request loans for themselves"})
			return
		}

		loan, err = h.loanRepo.Create(c.Request.Context(),
			groupID, callerID,
			req.Principal, req.Installments, emiAmount,
			models.LoanStatusRequested, nil, req.Note,
			nil, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create loan request"})
			return
		}
	}

	c.JSON(http.StatusCreated, loanToResponse(loan, 0, loan.Principal))
}

// ApproveLoan handles POST /api/v1/loans/:id/approve
func (h *LoanHandler) ApproveLoan(c *gin.Context) {
	callerIDStr, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	callerID, err := uuid.Parse(callerIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	loanID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid loan ID"})
		return
	}

	loan, err := h.loanRepo.GetByID(c.Request.Context(), loanID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "loan not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get loan"})
		return
	}

	caller, err := h.groupRepo.GetMember(c.Request.Context(), loan.GroupID, callerID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}
	if caller.Role != models.RoleHead {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group head can approve loans"})
		return
	}

	var req ApproveLoanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Resolve effective principal and installments (overrides or existing values).
	effectivePrincipal := loan.Principal
	effectiveInstallments := loan.Installments

	if req.Principal != nil {
		if *req.Principal <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "principal must be > 0"})
			return
		}
		effectivePrincipal = *req.Principal
	}
	if req.Installments != nil {
		if *req.Installments <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "installments must be > 0"})
			return
		}
		effectiveInstallments = *req.Installments
	}

	effectiveEMI := loanCeilDiv(effectivePrincipal, effectiveInstallments)
	start := loanNextPeriod(time.Now())
	now := time.Now()

	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to begin transaction"})
		return
	}
	defer tx.Rollback(c.Request.Context()) //nolint:errcheck

	updated, err := h.loanRepo.Approve(c.Request.Context(), tx,
		loanID, effectivePrincipal, effectiveInstallments, effectiveEMI,
		start, callerID, now)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusConflict, gin.H{"error": "loan is not pending approval"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to approve loan"})
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit"})
		return
	}

	c.JSON(http.StatusOK, loanToResponse(updated, 0, updated.Principal))
}

// RejectLoan handles POST /api/v1/loans/:id/reject
func (h *LoanHandler) RejectLoan(c *gin.Context) {
	callerIDStr, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	callerID, err := uuid.Parse(callerIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	loanID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid loan ID"})
		return
	}

	loan, err := h.loanRepo.GetByID(c.Request.Context(), loanID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "loan not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get loan"})
		return
	}

	caller, err := h.groupRepo.GetMember(c.Request.Context(), loan.GroupID, callerID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}
	if caller.Role != models.RoleHead {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group head can reject loans"})
		return
	}

	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to begin transaction"})
		return
	}
	defer tx.Rollback(c.Request.Context()) //nolint:errcheck

	updated, err := h.loanRepo.Reject(c.Request.Context(), tx, loanID, callerID, time.Now())
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusConflict, gin.H{"error": "loan is not pending approval"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reject loan"})
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit"})
		return
	}

	c.JSON(http.StatusOK, loanToResponse(updated, 0, updated.Principal))
}

// CloseLoan handles POST /api/v1/loans/:id/close (early payoff, head only).
func (h *LoanHandler) CloseLoan(c *gin.Context) {
	callerIDStr, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	callerID, err := uuid.Parse(callerIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	loanID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid loan ID"})
		return
	}

	loan, err := h.loanRepo.GetByID(c.Request.Context(), loanID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "loan not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get loan"})
		return
	}

	caller, err := h.groupRepo.GetMember(c.Request.Context(), loan.GroupID, callerID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}
	if caller.Role != models.RoleHead {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group head can close loans"})
		return
	}

	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to begin transaction"})
		return
	}
	defer tx.Rollback(c.Request.Context()) //nolint:errcheck

	// Lock the loan row FOR UPDATE to serialize against concurrent PostDue EMI posts (§4.5).
	active, err := h.loanRepo.LockActiveLoan(c.Request.Context(), tx, loanID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to lock loan"})
		return
	}
	if !active {
		c.JSON(http.StatusConflict, gin.H{"error": "loan is not active"})
		return
	}

	postedCount, err := h.loanRepo.CountPostedEMIs(c.Request.Context(), tx, loanID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count posted EMIs"})
		return
	}

	paid, err := h.loanRepo.SumPostedEMIs(c.Request.Context(), tx, loanID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sum posted EMIs"})
		return
	}

	outstanding := loan.Principal - paid
	finalPostedCount := postedCount

	if outstanding > 0 {
		// Post one final EMI debit for the exact outstanding amount.
		// The period is the next un-posted installment slot (start_period + postedCount).
		// The FOR UPDATE lock guarantees no concurrent engine EMI can occupy this slot (§4.5).
		finalPeriod := posting.AddMonths(*loan.StartPeriod, postedCount)
		note := "Early payoff — loan closed"
		inserted, err := h.ledgerRepo.InsertEMIPosting(c.Request.Context(), tx,
			loan.GroupID, loan.UserID, loanID, outstanding, finalPeriod, &note, callerID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to post final EMI"})
			return
		}
		if !inserted {
			// Under the FOR UPDATE lock this is impossible — indicates a logic error.
			c.JSON(http.StatusInternalServerError, gin.H{"error": "final EMI conflict: logic error"})
			return
		}
		finalPostedCount++
	}

	if err := h.loanRepo.CloseLoan(c.Request.Context(), tx, loanID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to close loan"})
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit"})
		return
	}

	// Refresh the loan for the response.
	updated, err := h.loanRepo.GetByID(c.Request.Context(), loanID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reload loan"})
		return
	}

	c.JSON(http.StatusOK, loanToResponse(updated, finalPostedCount, 0))
}
