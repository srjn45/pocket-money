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

	"github.com/srjn45/pocket-money/backend/internal/auth"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/handlers"
	"github.com/srjn45/pocket-money/backend/testutil"
)

const authTestJWTSecret = "test-jwt-secret-for-integration-tests"

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
	authHandler := handlers.NewAuthHandler(userRepo, authTestJWTSecret)

	router := gin.New()
	router.POST("/api/v1/auth/register", authHandler.Register)

	// Protected routes (auth middleware applied per-group)
	protected := router.Group("/api/v1/auth")
	protected.Use(auth.AuthMiddleware(authTestJWTSecret))
	protected.GET("/me", authHandler.Me)

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

	var response handlers.LoginResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotEmpty(t, response.Token)
	assert.Equal(t, "test@example.com", response.User.Email)
	assert.Equal(t, "Test User", response.User.Name)
	assert.NotEmpty(t, response.User.ID)
	assert.NotZero(t, response.User.CreatedAt)
}

func TestRegister_TokenLogsIn(t *testing.T) {
	router, cleanup := setupAuthTestRouter(t)
	defer cleanup()

	// Register a new user
	regBody := map[string]interface{}{
		"email":    "tokentest@example.com",
		"password": "password123",
		"name":     "Token Test",
	}
	jsonBody, _ := json.Marshal(regBody)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var regResp handlers.LoginResponse
	err := json.Unmarshal(w.Body.Bytes(), &regResp)
	require.NoError(t, err)
	require.NotEmpty(t, regResp.Token)

	// Use the returned token to hit a protected route
	meReq, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+regResp.Token)

	meW := httptest.NewRecorder()
	router.ServeHTTP(meW, meReq)

	assert.Equal(t, http.StatusOK, meW.Code)

	var meResp handlers.UserResponse
	err = json.Unmarshal(meW.Body.Bytes(), &meResp)
	require.NoError(t, err)
	assert.Equal(t, "tokentest@example.com", meResp.Email)
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
