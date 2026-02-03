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

// CreateLedgerRequest represents the request body for creating a ledger entry
type CreateLedgerRequest struct {
	UserID  *uuid.UUID `json:"user_id"` // Optional, only head can specify
	ChoreID uuid.UUID  `json:"chore_id" binding:"required"`
	Amount  float64    `json:"amount" binding:"required,gt=0"`
}

// LedgerResponse represents a ledger entry in API responses
type LedgerResponse struct {
	ID               uuid.UUID           `json:"id"`
	GroupID          uuid.UUID           `json:"group_id"`
	UserID           uuid.UUID           `json:"user_id"`
	ChoreID          uuid.UUID           `json:"chore_id"`
	Amount           float64             `json:"amount"`
	Status           models.LedgerStatus `json:"status"`
	CreatedByUserID  uuid.UUID           `json:"created_by_user_id"`
	ApprovedByUserID *uuid.UUID          `json:"approved_by_user_id,omitempty"`
	RejectedByUserID *uuid.UUID          `json:"rejected_by_user_id,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
}

// BalanceResponse represents a user's balance
type BalanceResponse struct {
	UserID  uuid.UUID `json:"user_id"`
	Name    string    `json:"name"`
	Balance float64   `json:"balance"`
}

// ListLedger returns ledger entries for a group
// GET /api/v1/groups/:id/ledger
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

	// Check if user is a member
	_, err = h.groupRepo.GetMember(c.Request.Context(), groupID, userID)
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

	entries, err := h.ledgerRepo.ListForGroup(c.Request.Context(), groupID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list ledger entries"})
		return
	}

	response := make([]LedgerResponse, 0, len(entries))
	for _, e := range entries {
		response = append(response, LedgerResponse{
			ID:               e.ID,
			GroupID:          e.GroupID,
			UserID:           e.UserID,
			ChoreID:          e.ChoreID,
			Amount:           e.Amount,
			Status:           e.Status,
			CreatedByUserID:  e.CreatedByUserID,
			ApprovedByUserID: e.ApprovedByUserID,
			RejectedByUserID: e.RejectedByUserID,
			CreatedAt:        e.CreatedAt,
		})
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

	// Check membership and get role
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

	// Validate chore belongs to this group
	chore, err := h.choreRepo.GetByID(c.Request.Context(), req.ChoreID)
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

	var targetUserID uuid.UUID
	var status models.LedgerStatus
	var approvedByUserID *uuid.UUID

	if member.Role == models.RoleHead {
		// Head can specify user_id and entry is auto-approved
		if req.UserID != nil {
			targetUserID = *req.UserID
			// Verify target user is a member
			_, err := h.groupRepo.GetMember(c.Request.Context(), groupID, targetUserID)
			if err != nil {
				if errors.Is(err, db.ErrNotFound) {
					c.JSON(http.StatusBadRequest, gin.H{"error": "target user is not a member of this group"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check target membership"})
				return
			}
		} else {
			targetUserID = userID
		}
		status = models.StatusApproved
		approvedByUserID = &userID
	} else {
		// Member can only create for self, pending approval
		targetUserID = userID
		status = models.StatusPendingApproval
		approvedByUserID = nil
	}

	entry, err := h.ledgerRepo.Create(c.Request.Context(), groupID, targetUserID, req.ChoreID, userID, req.Amount, status, approvedByUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create ledger entry"})
		return
	}

	c.JSON(http.StatusCreated, LedgerResponse{
		ID:               entry.ID,
		GroupID:          entry.GroupID,
		UserID:           entry.UserID,
		ChoreID:          entry.ChoreID,
		Amount:           entry.Amount,
		Status:           entry.Status,
		CreatedByUserID:  entry.CreatedByUserID,
		ApprovedByUserID: entry.ApprovedByUserID,
		RejectedByUserID: entry.RejectedByUserID,
		CreatedAt:        entry.CreatedAt,
	})
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

	// Get entry
	entry, err := h.ledgerRepo.GetByID(c.Request.Context(), entryID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get entry"})
		return
	}

	// Check if user is head of the group
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

	// Check status is pending
	if entry.Status != models.StatusPendingApproval {
		c.JSON(http.StatusBadRequest, gin.H{"error": "entry is not pending approval"})
		return
	}

	// Update status
	updatedEntry, err := h.ledgerRepo.UpdateStatus(c.Request.Context(), entryID, models.StatusApproved, &userID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to approve entry"})
		return
	}

	c.JSON(http.StatusOK, LedgerResponse{
		ID:               updatedEntry.ID,
		GroupID:          updatedEntry.GroupID,
		UserID:           updatedEntry.UserID,
		ChoreID:          updatedEntry.ChoreID,
		Amount:           updatedEntry.Amount,
		Status:           updatedEntry.Status,
		CreatedByUserID:  updatedEntry.CreatedByUserID,
		ApprovedByUserID: updatedEntry.ApprovedByUserID,
		RejectedByUserID: updatedEntry.RejectedByUserID,
		CreatedAt:        updatedEntry.CreatedAt,
	})
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

	// Get entry
	entry, err := h.ledgerRepo.GetByID(c.Request.Context(), entryID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get entry"})
		return
	}

	// Check if user is head of the group
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

	// Check status is pending
	if entry.Status != models.StatusPendingApproval {
		c.JSON(http.StatusBadRequest, gin.H{"error": "entry is not pending approval"})
		return
	}

	// Update status
	updatedEntry, err := h.ledgerRepo.UpdateStatus(c.Request.Context(), entryID, models.StatusRejected, nil, &userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reject entry"})
		return
	}

	c.JSON(http.StatusOK, LedgerResponse{
		ID:               updatedEntry.ID,
		GroupID:          updatedEntry.GroupID,
		UserID:           updatedEntry.UserID,
		ChoreID:          updatedEntry.ChoreID,
		Amount:           updatedEntry.Amount,
		Status:           updatedEntry.Status,
		CreatedByUserID:  updatedEntry.CreatedByUserID,
		ApprovedByUserID: updatedEntry.ApprovedByUserID,
		RejectedByUserID: updatedEntry.RejectedByUserID,
		CreatedAt:        updatedEntry.CreatedAt,
	})
}

// ListPending returns pending ledger entries for a group (head only)
// GET /api/v1/groups/:id/pending
func (h *LedgerHandler) ListPending(c *gin.Context) {
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

	// Check if user is head of the group
	member, err := h.groupRepo.GetMember(c.Request.Context(), groupID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}

	if member.Role != models.RoleHead {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group head can view pending entries"})
		return
	}

	status := models.StatusPendingApproval
	entries, err := h.ledgerRepo.ListForGroup(c.Request.Context(), groupID, &status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list pending entries"})
		return
	}

	response := make([]LedgerResponse, 0, len(entries))
	for _, e := range entries {
		response = append(response, LedgerResponse{
			ID:               e.ID,
			GroupID:          e.GroupID,
			UserID:           e.UserID,
			ChoreID:          e.ChoreID,
			Amount:           e.Amount,
			Status:           e.Status,
			CreatedByUserID:  e.CreatedByUserID,
			ApprovedByUserID: e.ApprovedByUserID,
			RejectedByUserID: e.RejectedByUserID,
			CreatedAt:        e.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, response)
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

	// Check if user is a member
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
