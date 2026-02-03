package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/srjn45/pocket-money/backend/internal/auth"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
)

// GroupHandler handles group-related requests
type GroupHandler struct {
	groupRepo  *db.GroupRepo
	inviteRepo *db.InviteRepo
}

// NewGroupHandler creates a new GroupHandler
func NewGroupHandler(groupRepo *db.GroupRepo, inviteRepo *db.InviteRepo) *GroupHandler {
	return &GroupHandler{
		groupRepo:  groupRepo,
		inviteRepo: inviteRepo,
	}
}

// CreateGroupRequest represents the request body for creating a group
type CreateGroupRequest struct {
	Name string `json:"name" binding:"required"`
}

// GroupResponse represents a group in API responses
type GroupResponse struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	HeadUserID uuid.UUID `json:"head_user_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// MemberResponse represents a member in API responses
type MemberResponse struct {
	UserID   uuid.UUID         `json:"user_id"`
	Name     string            `json:"name"`
	Email    string            `json:"email"`
	Role     models.MemberRole `json:"role"`
	JoinedAt time.Time         `json:"joined_at"`
}

// GroupDetailResponse represents detailed group information
type GroupDetailResponse struct {
	ID          uuid.UUID        `json:"id"`
	Name        string           `json:"name"`
	HeadUserID  uuid.UUID        `json:"head_user_id"`
	CreatedAt   time.Time        `json:"created_at"`
	Members     []MemberResponse `json:"members"`
	ChoresCount int              `json:"chores_count"`
}

// CreateGroup handles group creation
// POST /api/v1/groups
func (h *GroupHandler) CreateGroup(c *gin.Context) {
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

	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create group
	group, err := h.groupRepo.Create(c.Request.Context(), req.Name, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create group"})
		return
	}

	// Add creator as head member
	_, err = h.groupRepo.AddMember(c.Request.Context(), group.ID, userID, models.RoleHead)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add member"})
		return
	}

	c.JSON(http.StatusCreated, GroupResponse{
		ID:         group.ID,
		Name:       group.Name,
		HeadUserID: group.HeadUserID,
		CreatedAt:  group.CreatedAt,
	})
}

// ListGroups returns all groups for the authenticated user
// GET /api/v1/groups
func (h *GroupHandler) ListGroups(c *gin.Context) {
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

	groups, err := h.groupRepo.ListForUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list groups"})
		return
	}

	response := make([]GroupResponse, 0, len(groups))
	for _, g := range groups {
		response = append(response, GroupResponse{
			ID:         g.ID,
			Name:       g.Name,
			HeadUserID: g.HeadUserID,
			CreatedAt:  g.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, response)
}

// GetGroup returns a single group with details
// GET /api/v1/groups/:id
func (h *GroupHandler) GetGroup(c *gin.Context) {
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

	// Get group
	group, err := h.groupRepo.GetByID(c.Request.Context(), groupID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get group"})
		return
	}

	// Get members
	members, err := h.groupRepo.ListMembers(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get members"})
		return
	}

	// Get chores count
	choresCount, err := h.groupRepo.CountChores(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count chores"})
		return
	}

	memberResponses := make([]MemberResponse, 0, len(members))
	for _, m := range members {
		memberResponses = append(memberResponses, MemberResponse{
			UserID:   m.UserID,
			Name:     m.Name,
			Email:    m.Email,
			Role:     m.Role,
			JoinedAt: m.JoinedAt,
		})
	}

	c.JSON(http.StatusOK, GroupDetailResponse{
		ID:          group.ID,
		Name:        group.Name,
		HeadUserID:  group.HeadUserID,
		CreatedAt:   group.CreatedAt,
		Members:     memberResponses,
		ChoresCount: choresCount,
	})
}

// ListMembers returns all members of a group
// GET /api/v1/groups/:id/members
func (h *GroupHandler) ListMembers(c *gin.Context) {
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

	// Get members
	members, err := h.groupRepo.ListMembers(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get members"})
		return
	}

	response := make([]MemberResponse, 0, len(members))
	for _, m := range members {
		response = append(response, MemberResponse{
			UserID:   m.UserID,
			Name:     m.Name,
			Email:    m.Email,
			Role:     m.Role,
			JoinedAt: m.JoinedAt,
		})
	}

	c.JSON(http.StatusOK, response)
}

// InviteRequest represents the request body for creating an invite
type InviteRequest struct {
	ExpiresInDays int `json:"expires_in_days"`
}

// InviteResponse represents the response for creating an invite
type InviteResponse struct {
	InviteURL string    `json:"invite_url"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// JoinRequest represents the request body for joining a group
type JoinRequest struct {
	Token string `json:"token" binding:"required"`
}

// CreateInvite generates an invite token for a group
// POST /api/v1/groups/:id/invite
func (h *GroupHandler) CreateInvite(c *gin.Context) {
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
		c.JSON(http.StatusForbidden, gin.H{"error": "only group head can create invites"})
		return
	}

	// Parse request
	var req InviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Use default if not provided
		req.ExpiresInDays = 7
	}
	if req.ExpiresInDays <= 0 {
		req.ExpiresInDays = 7
	}

	// Generate random token
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	// Calculate expiry
	expiresAt := time.Now().Add(time.Duration(req.ExpiresInDays) * 24 * time.Hour)

	// Create invite in database
	invite, err := h.inviteRepo.Create(c.Request.Context(), groupID, token, expiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create invite"})
		return
	}

	// Build invite URL
	host := c.Request.Host
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	inviteURL := fmt.Sprintf("%s://%s/invite?token=%s", scheme, host, invite.Token)

	c.JSON(http.StatusCreated, InviteResponse{
		InviteURL: inviteURL,
		Token:     invite.Token,
		ExpiresAt: invite.ExpiresAt,
	})
}

// JoinGroup joins a group using an invite token
// POST /api/v1/groups/join
func (h *GroupHandler) JoinGroup(c *gin.Context) {
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

	var req JoinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get invite token
	invite, err := h.inviteRepo.GetByToken(c.Request.Context(), req.Token)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate token"})
		return
	}

	// Check if token is expired
	if time.Now().After(invite.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token expired"})
		return
	}

	// Check if user is already a member
	_, err = h.groupRepo.GetMember(c.Request.Context(), invite.GroupID, userID)
	if err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "already a member of this group"})
		return
	}
	if !errors.Is(err, db.ErrNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}

	// Add user as member
	_, err = h.groupRepo.AddMember(c.Request.Context(), invite.GroupID, userID, models.RoleMember)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to join group"})
		return
	}

	// Get group details
	group, err := h.groupRepo.GetByID(c.Request.Context(), invite.GroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get group"})
		return
	}

	c.JSON(http.StatusOK, GroupResponse{
		ID:         group.ID,
		Name:       group.Name,
		HeadUserID: group.HeadUserID,
		CreatedAt:  group.CreatedAt,
	})
}
