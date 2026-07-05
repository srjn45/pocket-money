package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/srjn45/pocket-money/backend/internal/auth"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
)

// LedgerHandler handles ledger-related requests
type LedgerHandler struct {
	ledgerRepo *db.LedgerRepo
	groupRepo  *db.GroupRepo
	choreRepo  *db.ChoreRepo
}

// NewLedgerHandler creates a new LedgerHandler
func NewLedgerHandler(ledgerRepo *db.LedgerRepo, groupRepo *db.GroupRepo, choreRepo *db.ChoreRepo) *LedgerHandler {
	return &LedgerHandler{
		ledgerRepo: ledgerRepo,
		groupRepo:  groupRepo,
		choreRepo:  choreRepo,
	}
}

// CreateLedgerRequest represents the request body for creating a ledger entry.
// entry_type is required. Other fields are conditionally required per type (enforced server-side).
type CreateLedgerRequest struct {
	EntryType models.LedgerEntryType  `json:"entry_type" binding:"required"`
	UserID    *uuid.UUID              `json:"user_id"`   // head may target a member
	ChoreID   *uuid.UUID              `json:"chore_id"`  // required iff entry_type=chore
	Amount    *int64                  `json:"amount"`    // required (>0) iff settlement/adjustment
	Direction *models.LedgerDirection `json:"direction"` // required iff entry_type=adjustment
	Note      *string                 `json:"note"`
}

// LedgerResponse represents a ledger entry in API responses
type LedgerResponse struct {
	ID              uuid.UUID               `json:"id"`
	GroupID         uuid.UUID               `json:"group_id"`
	UserID          uuid.UUID               `json:"user_id"`
	ChoreID         *uuid.UUID              `json:"chore_id,omitempty"`
	Amount          int64                   `json:"amount"`
	Status          models.LedgerStatus     `json:"status"`
	EntryType       models.LedgerEntryType  `json:"entry_type"`
	Direction       models.LedgerDirection  `json:"direction"`
	LoanID          *uuid.UUID              `json:"loan_id,omitempty"`
	Period          *string                 `json:"period,omitempty"`
	Note            *string                 `json:"note,omitempty"`
	CreatedByUserID uuid.UUID               `json:"created_by_user_id"`
	DecidedBy       *uuid.UUID              `json:"decided_by,omitempty"`
	DecidedAt       *time.Time              `json:"decided_at,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
}

// BalanceResponse represents a user's balance
type BalanceResponse struct {
	UserID  uuid.UUID `json:"user_id"`
	Name    string    `json:"name"`
	Balance int64     `json:"balance"`
}

func entryToResponse(e *models.LedgerEntry) LedgerResponse {
	return LedgerResponse{
		ID:              e.ID,
		GroupID:         e.GroupID,
		UserID:          e.UserID,
		ChoreID:         e.ChoreID,
		Amount:          e.Amount,
		Status:          e.Status,
		EntryType:       e.EntryType,
		Direction:       e.Direction,
		LoanID:          e.LoanID,
		Period:          e.Period,
		Note:            e.Note,
		CreatedByUserID: e.CreatedByUserID,
		DecidedBy:       e.DecidedBy,
		DecidedAt:       e.DecidedAt,
		CreatedAt:       e.CreatedAt,
	}
}

// isValidPeriod returns true if s matches YYYY-MM format.
func isValidPeriod(s string) bool {
	if len(s) != 7 || s[4] != '-' {
		return false
	}
	for i, c := range s {
		if i == 4 {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ListLedger returns ledger entries for a group
// GET /api/v1/groups/:id/ledger
// Query params: status (optional), user_id (optional - head only), type (optional), period (optional)
func (h *LedgerHandler) ListLedger(c *gin.Context) {
	userIDStr, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	groupIDStr := c.Param("id")
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group ID"})
		return
	}

	member, err := h.groupRepo.GetMember(c.Request.Context(), groupID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}

	// Parse optional status filter
	var status *models.LedgerStatus
	if statusStr := c.Query("status"); statusStr != "" {
		s := models.LedgerStatus(statusStr)
		if s != models.StatusApproved && s != models.StatusPendingApproval && s != models.StatusRejected {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return
		}
		status = &s
	}

	// Parse optional type filter
	var entryType *models.LedgerEntryType
	if typeStr := c.Query("type"); typeStr != "" {
		t := models.LedgerEntryType(typeStr)
		switch t {
		case models.EntryTypeChore, models.EntryTypeAllowance, models.EntryTypeEMI,
			models.EntryTypeSettlement, models.EntryTypeAdjustment:
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid type"})
			return
		}
		entryType = &t
	}

	// Parse optional period filter
	var period *string
	if periodStr := c.Query("period"); periodStr != "" {
		if !isValidPeriod(periodStr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period format, use YYYY-MM"})
			return
		}
		period = &periodStr
	}

	// Members can only see their own entries regardless of user_id query param
	var filterUserID *uuid.UUID
	if member.Role == models.RoleHead {
		if uidStr := c.Query("user_id"); uidStr != "" {
			parsed, err := uuid.Parse(uidStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
				return
			}
			filterUserID = &parsed
		}
	} else {
		filterUserID = &userID
	}

	entries, err := h.ledgerRepo.ListForGroupWithUser(c.Request.Context(), groupID, status, filterUserID, entryType, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list ledger entries"})
		return
	}

	response := make([]LedgerResponse, 0, len(entries))
	for _, e := range entries {
		response = append(response, entryToResponse(e))
	}
	c.JSON(http.StatusOK, response)
}

// CreateLedger creates a new ledger entry
// POST /api/v1/groups/:id/ledger
func (h *LedgerHandler) CreateLedger(c *gin.Context) {
	userIDStr, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	groupIDStr := c.Param("id")
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group ID"})
		return
	}

	member, err := h.groupRepo.GetMember(c.Request.Context(), groupID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}

	var req CreateLedgerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Reject machine-posted entry types
	if req.EntryType == models.EntryTypeAllowance || req.EntryType == models.EntryTypeEMI {
		c.JSON(http.StatusBadRequest, gin.H{"error": "allowance/emi entries are machine-posted only"})
		return
	}

	isHead := member.Role == models.RoleHead
	now := time.Now()

	var (
		targetUserID uuid.UUID
		amount       int64
		direction    models.LedgerDirection
		status       models.LedgerStatus
		decidedBy    *uuid.UUID
		decidedAt    *time.Time
		choreID      *uuid.UUID
	)

	switch req.EntryType {
	case models.EntryTypeChore:
		if req.ChoreID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "chore_id is required for chore entries"})
			return
		}
		chore, err := h.choreRepo.GetByID(c.Request.Context(), *req.ChoreID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "chore not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get chore"})
			return
		}
		if chore.GroupID != groupID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "chore does not belong to this group"})
			return
		}
		if chore.IsSystem {
			c.JSON(http.StatusBadRequest, gin.H{"error": "chore_id must refer to a non-system chore"})
			return
		}
		choreID = req.ChoreID
		amount = chore.Amount // amount from chore config; custom amount is ignored
		direction = models.DirectionCredit
		if isHead {
			targetUserID = resolveTargetUser(c, h, req.UserID, groupID, userID)
			if c.IsAborted() {
				return
			}
			status = models.StatusApproved
			decidedBy = &userID
			decidedAt = &now
		} else {
			targetUserID = userID
			status = models.StatusPendingApproval
		}

	case models.EntryTypeSettlement:
		if !isHead {
			c.JSON(http.StatusForbidden, gin.H{"error": "only group head can create settlement entries"})
			return
		}
		if req.UserID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required for settlement entries"})
			return
		}
		if req.Amount == nil || *req.Amount <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "amount is required and must be > 0 for settlement entries"})
			return
		}
		targetUserID = resolveTargetUser(c, h, req.UserID, groupID, userID)
		if c.IsAborted() {
			return
		}
		amount = *req.Amount
		direction = models.DirectionDebit
		status = models.StatusApproved
		decidedBy = &userID
		decidedAt = &now

	case models.EntryTypeAdjustment:
		if !isHead {
			c.JSON(http.StatusForbidden, gin.H{"error": "only group head can create adjustment entries"})
			return
		}
		if req.UserID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required for adjustment entries"})
			return
		}
		if req.Amount == nil || *req.Amount <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "amount is required and must be > 0 for adjustment entries"})
			return
		}
		if req.Direction == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "direction is required for adjustment entries"})
			return
		}
		d := *req.Direction
		if d != models.DirectionCredit && d != models.DirectionDebit {
			c.JSON(http.StatusBadRequest, gin.H{"error": "direction must be credit or debit"})
			return
		}
		targetUserID = resolveTargetUser(c, h, req.UserID, groupID, userID)
		if c.IsAborted() {
			return
		}
		amount = *req.Amount
		direction = d
		status = models.StatusApproved
		decidedBy = &userID
		decidedAt = &now

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entry_type"})
		return
	}

	entry, err := h.ledgerRepo.Create(
		c.Request.Context(), groupID, targetUserID, choreID,
		userID, amount, req.EntryType, direction, status,
		req.Note, decidedBy, decidedAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create ledger entry"})
		return
	}

	c.JSON(http.StatusCreated, entryToResponse(entry))
}

// resolveTargetUser returns the target user ID, verifying they are a member of groupID.
// If userIDParam is nil, the caller (callerID) is used as the target.
// On validation error it writes the error response and aborts c.
func resolveTargetUser(c *gin.Context, h *LedgerHandler, userIDParam *uuid.UUID, groupID, callerID uuid.UUID) uuid.UUID {
	if userIDParam == nil {
		return callerID
	}
	targetUserID := *userIDParam
	_, err := h.groupRepo.GetMember(c.Request.Context(), groupID, targetUserID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target user is not a member of this group"})
			c.Abort()
			return uuid.Nil
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check target membership"})
		c.Abort()
		return uuid.Nil
	}
	return targetUserID
}

// ApproveLedger approves a pending ledger entry
// POST /api/v1/ledger/:id/approve
func (h *LedgerHandler) ApproveLedger(c *gin.Context) {
	userIDStr, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	entryIDStr := c.Param("id")
	entryID, err := uuid.Parse(entryIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entry ID"})
		return
	}

	entry, err := h.ledgerRepo.GetByID(c.Request.Context(), entryID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get entry"})
		return
	}

	member, err := h.groupRepo.GetMember(c.Request.Context(), entry.GroupID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}

	if member.Role != models.RoleHead {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group head can approve entries"})
		return
	}

	if entry.Status != models.StatusPendingApproval {
		c.JSON(http.StatusBadRequest, gin.H{"error": "entry is not pending approval"})
		return
	}

	updated, err := h.ledgerRepo.SetDecision(c.Request.Context(), entryID, models.StatusApproved, userID, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to approve entry"})
		return
	}

	c.JSON(http.StatusOK, entryToResponse(updated))
}

// RejectLedger rejects a pending ledger entry
// POST /api/v1/ledger/:id/reject
func (h *LedgerHandler) RejectLedger(c *gin.Context) {
	userIDStr, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	entryIDStr := c.Param("id")
	entryID, err := uuid.Parse(entryIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entry ID"})
		return
	}

	entry, err := h.ledgerRepo.GetByID(c.Request.Context(), entryID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get entry"})
		return
	}

	member, err := h.groupRepo.GetMember(c.Request.Context(), entry.GroupID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}

	if member.Role != models.RoleHead {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group head can reject entries"})
		return
	}

	if entry.Status != models.StatusPendingApproval {
		c.JSON(http.StatusBadRequest, gin.H{"error": "entry is not pending approval"})
		return
	}

	updated, err := h.ledgerRepo.SetDecision(c.Request.Context(), entryID, models.StatusRejected, userID, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reject entry"})
		return
	}

	c.JSON(http.StatusOK, entryToResponse(updated))
}

// GetBalance returns per-member balances for a group
// GET /api/v1/groups/:id/balance
func (h *LedgerHandler) GetBalance(c *gin.Context) {
	userIDStr, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	groupIDStr := c.Param("id")
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group ID"})
		return
	}

	_, err = h.groupRepo.GetMember(c.Request.Context(), groupID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}

	balances, err := h.ledgerRepo.GetBalanceForGroup(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get balances"})
		return
	}

	response := make([]BalanceResponse, 0, len(balances))
	for _, b := range balances {
		response = append(response, BalanceResponse{
			UserID:  b.UserID,
			Name:    b.Name,
			Balance: b.Balance,
		})
	}
	c.JSON(http.StatusOK, response)
}
