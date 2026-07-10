package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/srjn45/pocket-money/backend/internal/auth"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
)

// errAlreadyRegistered is an internal sentinel for the claim path: a shadow row
// flipped to 'registered' under a concurrent claim, so registration must be
// rejected as a plain duplicate email.
var errAlreadyRegistered = errors.New("email already registered")

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	userRepo         *db.UserRepo
	groupRepo        *db.GroupRepo
	notificationRepo *db.NotificationRepo
	pool             *pgxpool.Pool
	jwtSecret        string
}

// NewAuthHandler creates a new AuthHandler. groupRepo/notificationRepo/pool are
// used by the register-claim flow (§5) to fan out N-2 notifications atomically.
func NewAuthHandler(userRepo *db.UserRepo, groupRepo *db.GroupRepo,
	notificationRepo *db.NotificationRepo, pool *pgxpool.Pool, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		userRepo:         userRepo,
		groupRepo:        groupRepo,
		notificationRepo: notificationRepo,
		pool:             pool,
		jwtSecret:        jwtSecret,
	}
}

// RegisterRequest represents the request body for user registration
type RegisterRequest struct {
	Email    string  `json:"email" binding:"required,email"`
	Password string  `json:"password" binding:"required,min=6"`
	Name     string  `json:"name" binding:"required"`
	DOB      *string `json:"dob"`
	Sex      *string `json:"sex"`
}

// LoginRequest represents the request body for user login
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents the response for successful login
type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

// UserResponse represents a user in API responses (without password)
type UserResponse struct {
	ID        uuid.UUID  `json:"id"`
	Email     string     `json:"email"`
	Name      string     `json:"name"`
	Status    string     `json:"status"` // 'shadow' | 'registered' (a logged-in user is always registered)
	DOB       *time.Time `json:"dob,omitempty"`
	Sex       *string    `json:"sex,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// toUserResponse maps a models.User to its API representation.
func toUserResponse(user *models.User) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Status:    user.Status,
		DOB:       user.DOB,
		Sex:       user.Sex,
		CreatedAt: user.CreatedAt,
	}
}

// Register handles user registration
// POST /api/v1/auth/register
//
// A matching shadow user (added by email but never registered) is CLAIMED in
// place — same user id, so all memberships/ledger/loans stay attached — and the
// group admins are notified (N-2). A matching registered user is a duplicate.
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	// Resolve any existing account for this email to decide the path (§5).
	existing, err := h.userRepo.GetByEmail(ctx, req.Email)
	switch {
	case errors.Is(err, db.ErrNotFound):
		// Brand-new registered user (no groups yet, so no notification).
		user, err := h.userRepo.Create(ctx, req.Email, string(hashedPassword), req.Name, req.DOB, req.Sex)
		if err != nil {
			if errors.Is(err, db.ErrDuplicateEmail) {
				// Raced with a concurrent insert — treat as a duplicate email.
				c.JSON(http.StatusBadRequest, gin.H{"error": "email already exists"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
			return
		}
		h.respondWithToken(c, http.StatusCreated, user)
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to find user"})
		return
	}

	// A real (registered) account already owns this email.
	if existing.Status == models.UserStatusRegistered {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email already exists"})
		return
	}

	// A shadow owns this email → claim it in place and notify group admins (N-2).
	claimed, err := h.claimShadow(ctx, existing, string(hashedPassword))
	if err != nil {
		if errors.Is(err, errAlreadyRegistered) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register"})
		return
	}
	h.respondWithToken(c, http.StatusCreated, claimed)
}

// claimShadow upgrades a shadow user to registered in one transaction (§5):
// re-locks the row and re-checks it is still a shadow, sets the password/status/
// claimed_at, then fans out N-2 (shadow_claimed) to every group admin that is not
// the claimant. Returns the claimed user on success; errAlreadyRegistered if the
// row flipped to registered under a concurrent claim.
func (h *AuthHandler) claimShadow(ctx context.Context, existing *models.User, passwordHash string) (*models.User, error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Lock the row and re-check the status (guards a concurrent claim).
	locked, err := h.userRepo.GetByEmailForUpdate(ctx, tx, existing.Email)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, errAlreadyRegistered
		}
		return nil, err
	}
	if locked.Status != models.UserStatusShadow {
		return nil, errAlreadyRegistered
	}

	if err := h.userRepo.ClaimShadow(ctx, tx, locked.ID, passwordHash); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, errAlreadyRegistered
		}
		return nil, err
	}

	// N-2 fan-out: notify each of the claimant's groups' admins (except self).
	groupIDs, err := h.groupRepo.ListGroupIDsForUser(ctx, tx, locked.ID)
	if err != nil {
		return nil, err
	}
	for _, gid := range groupIDs {
		group, err := h.groupRepo.GetByID(ctx, gid)
		if err != nil {
			return nil, err
		}
		adminIDs, err := h.groupRepo.ListAdminUserIDs(ctx, tx, gid)
		if err != nil {
			return nil, err
		}
		payload, err := json.Marshal(map[string]string{
			"group_id":           gid.String(),
			"group_name":         group.Name,
			"claimed_user_id":    locked.ID.String(),
			"claimed_user_name":  locked.Name,
			"claimed_user_email": locked.Email,
		})
		if err != nil {
			return nil, err
		}
		for _, adminID := range adminIDs {
			if adminID == locked.ID {
				continue // never notify the claimant about their own claim
			}
			if err := h.notificationRepo.Insert(ctx, tx, adminID, models.NotificationShadowClaimed, payload); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Reflect the claimed state for the response body.
	locked.Status = models.UserStatusRegistered
	locked.PasswordHash = &passwordHash
	return locked, nil
}

// respondWithToken issues a JWT for the user and returns it with the user body.
func (h *AuthHandler) respondWithToken(c *gin.Context, status int, user *models.User) {
	// Issue a JWT so the client is logged in immediately (auto-login, WP-4.6 D1).
	token, err := auth.IssueToken(user.ID.String(), h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	c.JSON(status, LoginResponse{
		Token: token,
		User:  toUserResponse(user),
	})
}

// Login handles user login
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user by email
	user, err := h.userRepo.GetByEmail(c.Request.Context(), req.Email)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to find user"})
		return
	}

	// Shadow users have no password (PasswordHash == nil) and can NEVER
	// authenticate. Reject with the SAME generic message as a wrong password so
	// the response does not reveal that the email exists as a shadow (§6). This
	// is the single structural rejection point — no token is ever minted here.
	if user.PasswordHash == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	// Compare password
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	// Issue JWT token
	token, err := auth.IssueToken(user.ID.String(), h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	// Return token and user
	c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User:  toUserResponse(user),
	})
}

// ChangePasswordRequest is the body for PUT /auth/password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

// ChangePassword handles PUT /api/v1/auth/password.
func (h *AuthHandler) ChangePassword(c *gin.Context) {
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

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	// A shadow can never hold a token, so this is defensive: a nil hash means no
	// credential to verify against — reject as unauthenticated (§3.4).
	if user.PasswordHash == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	// Verify the CURRENT password. Mismatch → 403, NOT 401 (a 401 would trip the FE
	// client's global logout interceptor and end the session mid-change).
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "current password is incorrect"})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}
	if err := h.userRepo.UpdatePassword(c.Request.Context(), userID, string(newHash)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}

	c.Status(http.StatusNoContent) // 204 — nothing to return; JWT unchanged (D1)
}

// Me returns the current authenticated user
// GET /api/v1/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
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

	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}
