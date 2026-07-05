package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srjn45/pocket-money/backend/internal/auth"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
	"github.com/srjn45/pocket-money/backend/internal/posting"
)

// GroupHandler handles group-related requests
type GroupHandler struct {
	groupRepo     *db.GroupRepo
	inviteRepo    *db.InviteRepo
	choreRepo     *db.ChoreRepo
	ledgerRepo    *db.LedgerRepo
	loanRepo      *db.LoanRepo
	allowanceRepo *db.AllowanceRepo
	postingSvc    *posting.Service
	pool          *pgxpool.Pool
	appBaseURL    string
}

// NewGroupHandler creates a new GroupHandler
func NewGroupHandler(
	groupRepo *db.GroupRepo,
	inviteRepo *db.InviteRepo,
	choreRepo *db.ChoreRepo,
	ledgerRepo *db.LedgerRepo,
	loanRepo *db.LoanRepo,
	allowanceRepo *db.AllowanceRepo,
	postingSvc *posting.Service,
	pool *pgxpool.Pool,
	appBaseURL string,
) *GroupHandler {
	return &GroupHandler{
		groupRepo:     groupRepo,
		inviteRepo:    inviteRepo,
		choreRepo:     choreRepo,
		ledgerRepo:    ledgerRepo,
		loanRepo:      loanRepo,
		allowanceRepo: allowanceRepo,
		postingSvc:    postingSvc,
		pool:          pool,
		appBaseURL:    appBaseURL,
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

// GroupSummaryResponse is one dashboard-listing row (see openapi GroupSummaryResponse).
type GroupSummaryResponse struct {
	ID             uuid.UUID         `json:"id"`
	Name           string            `json:"name"`
	HeadUserID     uuid.UUID         `json:"head_user_id"`
	CreatedAt      time.Time         `json:"created_at"`
	Role           models.MemberRole `json:"role"`
	MemberCount    int               `json:"member_count"`
	SummaryBalance int64             `json:"summary_balance"`
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

	// Create default Settlement chore (system chore for payouts)
	settlementDesc := "Cash payout to member"
	_, err = h.choreRepo.CreateWithSystem(c.Request.Context(), group.ID, "Settlement", &settlementDesc, 0, true)
	if err != nil {
		// Log error but don't fail group creation
		// The settlement chore can be created manually if needed
		fmt.Printf("Warning: failed to create settlement chore for group %s: %v\n", group.ID, err)
	}

	c.JSON(http.StatusCreated, GroupResponse{
		ID:         group.ID,
		Name:       group.Name,
		HeadUserID: group.HeadUserID,
		CreatedAt:  group.CreatedAt,
	})
}

// ListGroups returns the user's groups enriched for the dashboard. Does NOT trigger posting
// (WP-4.2 §0.2): summary_balance reflects currently-posted approved entries.
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

	summaries, err := h.groupRepo.ListForUserWithSummary(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list groups"})
		return
	}

	response := make([]GroupSummaryResponse, 0, len(summaries))
	for _, s := range summaries {
		response = append(response, GroupSummaryResponse{
			ID:             s.ID,
			Name:           s.Name,
			HeadUserID:     s.HeadUserID,
			CreatedAt:      s.CreatedAt,
			Role:           s.Role,
			MemberCount:    s.MemberCount,
			SummaryBalance: s.SummaryBalance,
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

	// Trigger due allowance posting before serving group detail (balance-sensitive).
	if !runPosting(c, h.postingSvc, groupID) {
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

	// Build invite URL.  Prefer APP_BASE_URL so the link points at the Expo web
	// server, not the API server (which has no /invite route).  Fall back to the
	// request host for same-origin or dev setups where both are co-located.
	var inviteURL string
	if h.appBaseURL != "" {
		inviteURL = fmt.Sprintf("%s/invite?token=%s", strings.TrimRight(h.appBaseURL, "/"), invite.Token)
	} else {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		inviteURL = fmt.Sprintf("%s://%s/invite?token=%s", scheme, c.Request.Host, invite.Token)
	}

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

// RemoveMember handles DELETE /api/v1/groups/:id/members/:userId.
// Head removes a member; member leaves self. See WP-4.7 §4.2.
func (h *GroupHandler) RemoveMember(c *gin.Context) {
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
	targetID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	// Caller must be a member; get their role for the authz decision.
	caller, err := h.groupRepo.GetMember(c.Request.Context(), groupID, callerID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}

	// Authz: head may remove others; anyone may remove self; a non-head may NOT remove another member.
	isSelf := targetID == callerID
	if !isSelf && caller.Role != models.RoleHead {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the group head can remove other members"})
		return
	}

	// D3: post any due allowance/EMI FIRST (own tx), so the balance below is current
	// and a stale un-posted entry cannot let a non-zero member out.
	if !runPosting(c, h.postingSvc, groupID) {
		return
	}

	ctx := c.Request.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to begin transaction"})
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Lock the target's membership (anchor); confirms they are a member.
	role, err := h.groupRepo.LockMembershipForUpdate(ctx, tx, groupID, targetID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user is not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to lock membership"})
		return
	}
	// D6: the head can neither leave nor be removed.
	if role == models.RoleHead {
		c.JSON(http.StatusConflict, gin.H{"error": "the group head cannot leave or be removed"})
		return
	}

	// D5: block on any requested/active loan (FOR UPDATE serializes vs PostDue EMI posting).
	blocking, err := h.loanRepo.LockBlockingLoans(ctx, tx, groupID, targetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check loans"})
		return
	}
	if blocking > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "cannot remove a member who has an active or pending loan; close or reject it first"})
		return
	}

	// D4: balance must be exactly zero (same math as GetBalanceForGroup).
	balance, err := h.ledgerRepo.MemberBalanceTx(ctx, tx, groupID, targetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check balance"})
		return
	}
	if balance != 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "cannot remove a member whose balance is not settled to zero; settle their balance first"})
		return
	}

	// D7: reject the member's pending entries, delete their allowance config, delete
	// the membership. Ledger history (approved/rejected) and closed/rejected loans stay.
	now := time.Now()
	if _, err := h.ledgerRepo.RejectPendingForMember(ctx, tx, groupID, targetID, callerID, now); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reject pending entries"})
		return
	}
	if err := h.allowanceRepo.DeleteForMember(ctx, tx, groupID, targetID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete allowances"})
		return
	}
	if err := h.groupRepo.DeleteMembership(ctx, tx, groupID, targetID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove member"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit"})
		return
	}
	c.Status(http.StatusNoContent) // 204
}
