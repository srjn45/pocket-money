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

// AllowanceHandler handles allowance-related requests.
type AllowanceHandler struct {
	allowanceRepo *db.AllowanceRepo
	groupRepo     *db.GroupRepo
}

// NewAllowanceHandler creates a new AllowanceHandler.
func NewAllowanceHandler(allowanceRepo *db.AllowanceRepo, groupRepo *db.GroupRepo) *AllowanceHandler {
	return &AllowanceHandler{
		allowanceRepo: allowanceRepo,
		groupRepo:     groupRepo,
	}
}

// AllowanceResponse is the API representation of an allowance row.
type AllowanceResponse struct {
	ID            uuid.UUID `json:"id"`
	GroupID       uuid.UUID `json:"group_id"`
	UserID        uuid.UUID `json:"user_id"`
	Amount        int64     `json:"amount"`
	EffectiveFrom string    `json:"effective_from"`
	CreatedBy     uuid.UUID `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
}

// SetAllowanceRequest is the body for PUT /groups/:id/allowances/:userId.
type SetAllowanceRequest struct {
	Amount        int64   `json:"amount"`
	EffectiveFrom *string `json:"effective_from"`
}

func allowanceToResponse(a *models.Allowance) AllowanceResponse {
	return AllowanceResponse{
		ID:            a.ID,
		GroupID:       a.GroupID,
		UserID:        a.UserID,
		Amount:        a.Amount,
		EffectiveFrom: a.EffectiveFrom,
		CreatedBy:     a.CreatedBy,
		CreatedAt:     a.CreatedAt,
	}
}

// ListAllowances returns allowance rows for the group.
// GET /api/v1/groups/:id/allowances
// Head → all members' rows. Member → own rows only.
func (h *AllowanceHandler) ListAllowances(c *gin.Context) {
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

	var allowances []*models.Allowance
	if member.Role == models.RoleHead {
		allowances, err = h.allowanceRepo.ListForGroup(c.Request.Context(), groupID)
	} else {
		allowances, err = h.allowanceRepo.ListForUser(c.Request.Context(), groupID, userID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list allowances"})
		return
	}

	response := make([]AllowanceResponse, 0, len(allowances))
	for _, a := range allowances {
		response = append(response, allowanceToResponse(a))
	}
	c.JSON(http.StatusOK, response)
}

// SetAllowance sets or changes a member's monthly pocket money.
// PUT /api/v1/groups/:id/allowances/:userId
// Head only. Target must be a member (not the head).
func (h *AllowanceHandler) SetAllowance(c *gin.Context) {
	userIDStr, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	callerID, err := uuid.Parse(userIDStr)
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

	caller, err := h.groupRepo.GetMember(c.Request.Context(), groupID, callerID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}

	if caller.Role != models.RoleHead {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group head can manage allowances"})
		return
	}

	targetIDStr := c.Param("userId")
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	target, err := h.groupRepo.GetMember(c.Request.Context(), groupID, targetID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "target user is not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check target membership"})
		return
	}

	if target.Role == models.RoleHead {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot set an allowance for the group head"})
		return
	}

	var req SetAllowanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Amount < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be >= 0"})
		return
	}

	effectiveFrom := time.Now().Format("2006-01")
	if req.EffectiveFrom != nil {
		if !isValidPeriod(*req.EffectiveFrom) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "effective_from must match YYYY-MM format"})
			return
		}
		effectiveFrom = *req.EffectiveFrom
	}

	allowance, err := h.allowanceRepo.SetAllowance(c.Request.Context(), groupID, targetID, req.Amount, effectiveFrom, callerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set allowance"})
		return
	}

	c.JSON(http.StatusOK, allowanceToResponse(allowance))
}
