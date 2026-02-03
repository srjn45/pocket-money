package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupCORSTestRouter(origins string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORSMiddleware(origins))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return router
}

func TestCORSMiddleware_AllowsAllOrigins(t *testing.T) {
	router := setupCORSTestRouter("*")

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// When * is allowed and origin is provided, we echo back the origin
	assert.Equal(t, "http://example.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_NoOriginHeader(t *testing.T) {
	router := setupCORSTestRouter("*")

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	// No Origin header - CORS headers only apply when Origin is present

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// CORS headers are only set when Origin header is present
	// When no Origin header, we don't set Access-Control-Allow-Origin
}

func TestCORSMiddleware_AllowsSpecificOrigin(t *testing.T) {
	router := setupCORSTestRouter("http://localhost:3000,http://example.com")

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_OptionsPreflightReturns204(t *testing.T) {
	router := setupCORSTestRouter("*")

	req, _ := http.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://example.com")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestCORSMiddleware_AllowedHeaders(t *testing.T) {
	router := setupCORSTestRouter("*")

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "PATCH")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "DELETE")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
}

func TestParseOrigins(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{"*"}},
		{"*", []string{"*"}},
		{"http://localhost:3000", []string{"http://localhost:3000"}},
		{"http://localhost:3000,http://example.com", []string{"http://localhost:3000", "http://example.com"}},
		{"  http://localhost:3000  ,  http://example.com  ", []string{"http://localhost:3000", "http://example.com"}},
	}

	for _, tt := range tests {
		result := parseOrigins(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}
