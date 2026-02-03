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

// ChoreHandler handles chore-related requests
type ChoreHandler struct {
	choreRepo *db.ChoreRepo
	groupRepo *db.GroupRepo
}

// NewChoreHandler creates a new ChoreHandler
func NewChoreHandler(choreRepo *db.ChoreRepo, groupRepo *db.GroupRepo) *ChoreHandler {
	return &ChoreHandler{
		choreRepo: choreRepo,
		groupRepo: groupRepo,
	}
}

// CreateChoreRequest represents the request body for creating a chore
type CreateChoreRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	Amount      float64 `json:"amount" binding:"required,gt=0"`
}

// UpdateChoreRequest represents the request body for updating a chore
type UpdateChoreRequest struct {
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	Amount      *float64 `json:"amount"`
}

// ChoreResponse represents a chore in API responses
type ChoreResponse struct {
	ID          uuid.UUID `json:"id"`
	GroupID     uuid.UUID `json:"group_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Amount      float64   `json:"amount"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListChores returns all chores for a group
// GET /api/v1/groups/:id/chores
func (h *ChoreHandler) ListChores(c *gin.Context) {
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

	chores, err := h.choreRepo.ListForGroup(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list chores"})
		return
	}

	response := make([]ChoreResponse, 0, len(chores))
	for _, ch := range chores {
		response = append(response, ChoreResponse{
			ID:          ch.ID,
			GroupID:     ch.GroupID,
			Name:        ch.Name,
			Description: ch.Description,
			Amount:      ch.Amount,
			CreatedAt:   ch.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, response)
}

// CreateChore creates a new chore for a group
// POST /api/v1/groups/:id/chores
func (h *ChoreHandler) CreateChore(c *gin.Context) {
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
		c.JSON(http.StatusForbidden, gin.H{"error": "only group head can create chores"})
		return
	}

	var req CreateChoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	chore, err := h.choreRepo.Create(c.Request.Context(), groupID, req.Name, req.Description, req.Amount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create chore"})
		return
	}

	c.JSON(http.StatusCreated, ChoreResponse{
		ID:          chore.ID,
		GroupID:     chore.GroupID,
		Name:        chore.Name,
		Description: chore.Description,
		Amount:      chore.Amount,
		CreatedAt:   chore.CreatedAt,
	})
}

// UpdateChore updates a chore
// PATCH /api/v1/chores/:id
func (h *ChoreHandler) UpdateChore(c *gin.Context) {
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

	choreIDStr := c.Param("id")
	choreID, err := uuid.Parse(choreIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chore ID"})
		return
	}

	// Get chore
	chore, err := h.choreRepo.GetByID(c.Request.Context(), choreID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "chore not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get chore"})
		return
	}

	// Check if user is head of the group
	member, err := h.groupRepo.GetMember(c.Request.Context(), chore.GroupID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}

	if member.Role != models.RoleHead {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group head can update chores"})
		return
	}

	var req UpdateChoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updatedChore, err := h.choreRepo.Update(c.Request.Context(), choreID, req.Name, req.Description, req.Amount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update chore"})
		return
	}

	c.JSON(http.StatusOK, ChoreResponse{
		ID:          updatedChore.ID,
		GroupID:     updatedChore.GroupID,
		Name:        updatedChore.Name,
		Description: updatedChore.Description,
		Amount:      updatedChore.Amount,
		CreatedAt:   updatedChore.CreatedAt,
	})
}

// DeleteChore deletes a chore
// DELETE /api/v1/chores/:id
func (h *ChoreHandler) DeleteChore(c *gin.Context) {
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

	choreIDStr := c.Param("id")
	choreID, err := uuid.Parse(choreIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chore ID"})
		return
	}

	// Get chore
	chore, err := h.choreRepo.GetByID(c.Request.Context(), choreID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "chore not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get chore"})
		return
	}

	// Check if user is head of the group
	member, err := h.groupRepo.GetMember(c.Request.Context(), chore.GroupID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}

	if member.Role != models.RoleHead {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group head can delete chores"})
		return
	}

	if err := h.choreRepo.Delete(c.Request.Context(), choreID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete chore"})
		return
	}

	c.Status(http.StatusNoContent)
}
