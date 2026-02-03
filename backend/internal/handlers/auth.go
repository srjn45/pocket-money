package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/srjn45/pocket-money/backend/internal/auth"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	userRepo  *db.UserRepo
	jwtSecret string
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(userRepo *db.UserRepo, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
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
	DOB       *time.Time `json:"dob,omitempty"`
	Sex       *string    `json:"sex,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Register handles user registration
// POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	// Create user in database
	user, err := h.userRepo.Create(c.Request.Context(), req.Email, string(hashedPassword), req.Name, req.DOB, req.Sex)
	if err != nil {
		if errors.Is(err, db.ErrDuplicateEmail) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	// Return user response
	c.JSON(http.StatusCreated, UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		DOB:       user.DOB,
		Sex:       user.Sex,
		CreatedAt: user.CreatedAt,
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

	// Compare password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
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
		User: UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			DOB:       user.DOB,
			Sex:       user.Sex,
			CreatedAt: user.CreatedAt,
		},
	})
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

	c.JSON(http.StatusOK, UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		DOB:       user.DOB,
		Sex:       user.Sex,
		CreatedAt: user.CreatedAt,
	})
}
