package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// UserIDKey is the key used to store user ID in gin context
	UserIDKey = "user_id"
)

// AuthMiddleware validates JWT tokens and sets user_id in context
func AuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			return
		}

		// Check Bearer prefix
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			return
		}

		tokenString := parts[1]

		// Validate token
		userID, err := ValidateToken(tokenString, secret)
		if err != nil {
			if err == ErrExpiredToken {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has expired"})
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		// Set user ID in context
		c.Set(UserIDKey, userID)
		c.Next()
	}
}

// OptionalAuthMiddleware validates JWT tokens if present but doesn't require them
// Sets user_id in context if token is valid, otherwise continues without user_id
func OptionalAuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		// Check Bearer prefix
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.Next()
			return
		}

		tokenString := parts[1]

		// Validate token
		userID, err := ValidateToken(tokenString, secret)
		if err == nil {
			c.Set(UserIDKey, userID)
		}

		c.Next()
	}
}

// GetUserID retrieves the user ID from gin context
func GetUserID(c *gin.Context) (string, bool) {
	userID, exists := c.Get(UserIDKey)
	if !exists {
		return "", false
	}
	userIDStr, ok := userID.(string)
	if !ok {
		return "", false
	}
	return userIDStr, true
}
