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

// SettlementHandler handles settlement-related requests
type SettlementHandler struct {
	settlementRepo *db.SettlementRepo
	groupRepo      *db.GroupRepo
}

// NewSettlementHandler creates a new SettlementHandler
func NewSettlementHandler(settlementRepo *db.SettlementRepo, groupRepo *db.GroupRepo) *SettlementHandler {
	return &SettlementHandler{
		settlementRepo: settlementRepo,
		groupRepo:      groupRepo,
	}
}

// CreateSettlementRequest represents the request body for creating a settlement
type CreateSettlementRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required"`
	Amount float64   `json:"amount" binding:"required,gt=0"`
	Date   string    `json:"date" binding:"required"` // YYYY-MM-DD format
	Note   *string   `json:"note"`
}

// SettlementResponse represents a settlement in API responses
type SettlementResponse struct {
	ID        uuid.UUID `json:"id"`
	GroupID   uuid.UUID `json:"group_id"`
	UserID    uuid.UUID `json:"user_id"`
	Amount    float64   `json:"amount"`
	Date      time.Time `json:"date"`
	Note      *string   `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ListSettlements returns all settlements for a group
// GET /api/v1/groups/:id/settlements
func (h *SettlementHandler) ListSettlements(c *gin.Context) {
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

	settlements, err := h.settlementRepo.ListForGroup(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list settlements"})
		return
	}

	response := make([]SettlementResponse, 0, len(settlements))
	for _, s := range settlements {
		response = append(response, SettlementResponse{
			ID:        s.ID,
			GroupID:   s.GroupID,
			UserID:    s.UserID,
			Amount:    s.Amount,
			Date:      s.Date,
			Note:      s.Note,
			CreatedAt: s.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, response)
}

// CreateSettlement creates a new settlement
// POST /api/v1/groups/:id/settlements
func (h *SettlementHandler) CreateSettlement(c *gin.Context) {
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
		c.JSON(http.StatusForbidden, gin.H{"error": "only group head can create settlements"})
		return
	}

	var req CreateSettlementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse date
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format, use YYYY-MM-DD"})
		return
	}

	// Verify target user is a member
	_, err = h.groupRepo.GetMember(c.Request.Context(), groupID, req.UserID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target user is not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check target membership"})
		return
	}

	settlement, err := h.settlementRepo.Create(c.Request.Context(), groupID, req.UserID, req.Amount, date, req.Note)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create settlement"})
		return
	}

	c.JSON(http.StatusCreated, SettlementResponse{
		ID:        settlement.ID,
		GroupID:   settlement.GroupID,
		UserID:    settlement.UserID,
		Amount:    settlement.Amount,
		Date:      settlement.Date,
		Note:      settlement.Note,
		CreatedAt: settlement.CreatedAt,
	})
}
