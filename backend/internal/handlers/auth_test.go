//go:build integration

package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/handlers"
	"github.com/srjn45/pocket-money/backend/testutil"
)

func setupAuthTestRouter(t *testing.T) (*gin.Engine, func()) {
	gin.SetMode(gin.TestMode)

	pool, err := testutil.NewTestPool()
	if err != nil {
		t.Skipf("Skipping test: could not connect to test database: %v", err)
	}

	// Full reset to ensure clean state (drops schema + data)
	_ = testutil.ResetTestDB(pool)

	// Run migrations
	dbURL := testutil.GetTestDatabaseURL()
	err = db.RunMigrations(dbURL)
	require.NoError(t, err)

	userRepo := db.NewUserRepo(pool)
	jwtSecret := "test-jwt-secret-for-integration-tests"
	authHandler := handlers.NewAuthHandler(userRepo, jwtSecret)

	router := gin.New()
	router.POST("/api/v1/auth/register", authHandler.Register)

	cleanup := func() {
		testutil.CleanupTestDB(pool)
		pool.Close()
	}

	return router, cleanup
}

func TestRegister_Success(t *testing.T) {
	router, cleanup := setupAuthTestRouter(t)
	defer cleanup()

	body := map[string]interface{}{
		"email":    "test@example.com",
		"password": "password123",
		"name":     "Test User",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response handlers.UserResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "test@example.com", response.Email)
	assert.Equal(t, "Test User", response.Name)
	assert.NotEmpty(t, response.ID)
	assert.NotZero(t, response.CreatedAt)
}

func TestRegister_MissingFields(t *testing.T) {
	router, cleanup := setupAuthTestRouter(t)
	defer cleanup()

	testCases := []struct {
		name string
		body map[string]interface{}
	}{
		{
			name: "missing email",
			body: map[string]interface{}{
				"password": "password123",
				"name":     "Test User",
			},
		},
		{
			name: "missing password",
			body: map[string]interface{}{
				"email": "test@example.com",
				"name":  "Test User",
			},
		},
		{
			name: "missing name",
			body: map[string]interface{}{
				"email":    "test@example.com",
				"password": "password123",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jsonBody, _ := json.Marshal(tc.body)
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	router, cleanup := setupAuthTestRouter(t)
	defer cleanup()

	body := map[string]interface{}{
		"email":    "duplicate@example.com",
		"password": "password123",
		"name":     "Test User",
	}
	jsonBody, _ := json.Marshal(body)

	// First registration should succeed
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Second registration with same email should fail
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "email already exists")
}

func TestRegister_InvalidEmail(t *testing.T) {
	router, cleanup := setupAuthTestRouter(t)
	defer cleanup()

	body := map[string]interface{}{
		"email":    "invalid-email",
		"password": "password123",
		"name":     "Test User",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegister_ShortPassword(t *testing.T) {
	router, cleanup := setupAuthTestRouter(t)
	defer cleanup()

	body := map[string]interface{}{
		"email":    "test@example.com",
		"password": "short",
		"name":     "Test User",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
