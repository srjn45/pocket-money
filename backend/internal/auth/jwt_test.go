package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueToken(t *testing.T) {
	userID := "test-user-id"
	secret := "test-secret"

	token, err := IssueToken(userID, secret)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestValidateToken_Valid(t *testing.T) {
	userID := "test-user-id"
	secret := "test-secret"

	token, err := IssueToken(userID, secret)
	require.NoError(t, err)

	validatedUserID, err := ValidateToken(token, secret)
	require.NoError(t, err)
	assert.Equal(t, userID, validatedUserID)
}

func TestValidateToken_InvalidSecret(t *testing.T) {
	userID := "test-user-id"
	secret := "test-secret"
	wrongSecret := "wrong-secret"

	token, err := IssueToken(userID, secret)
	require.NoError(t, err)

	_, err = ValidateToken(token, wrongSecret)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateToken_Expired(t *testing.T) {
	userID := "test-user-id"
	secret := "test-secret"

	// Create an expired token manually
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // Expired 1 hour ago
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	_, err = ValidateToken(tokenString, secret)
	assert.ErrorIs(t, err, ErrExpiredToken)
}

func TestValidateToken_Malformed(t *testing.T) {
	secret := "test-secret"

	_, err := ValidateToken("malformed-token", secret)
	assert.ErrorIs(t, err, ErrInvalidToken)
}
