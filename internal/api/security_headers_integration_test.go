package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecurityHeadersIntegration_AppliedToAllRoutes verifies middleware is applied globally
func TestSecurityHeadersIntegration_AppliedToAllRoutes(t *testing.T) {
	// Setup full Echo instance with all middleware
	e := echo.New()

	// Apply middleware in same order as router.go
	e.Use(RequestIDMiddleware())
	e.Use(MetricsMiddleware())
	e.Use(SecurityHeadersMiddleware())

	// Add test routes
	e.GET("/test-html", func(c echo.Context) error {
		return c.HTML(http.StatusOK, "<html><body>Test</body></html>")
	})

	e.GET("/test-json", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	e.POST("/test-post", func(c echo.Context) error {
		return c.NoContent(http.StatusCreated)
	})

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "HTML route has security headers",
			method: http.MethodGet,
			path:   "/test-html",
		},
		{
			name:   "JSON route has security headers",
			method: http.MethodGet,
			path:   "/test-json",
		},
		{
			name:   "POST route has security headers",
			method: http.MethodPost,
			path:   "/test-post",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.URL.Scheme = "https" // For HSTS header
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			// Verify all security headers are present
			assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
			assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
			assert.Equal(t, "1; mode=block", rec.Header().Get("X-XSS-Protection"))
			assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))
			assert.Equal(t, "max-age=31536000; includeSubDomains", rec.Header().Get("Strict-Transport-Security"))

			csp := rec.Header().Get("Content-Security-Policy")
			assert.NotEmpty(t, csp)
			assert.Contains(t, csp, "default-src 'self'")
		})
	}
}

// TestSecurityHeadersIntegration_HealthCheckHasHeaders verifies even health endpoint gets security headers
// In Echo, e.Use() applies to all routes regardless of definition order
func TestSecurityHeadersIntegration_HealthCheckHasHeaders(t *testing.T) {
	e := echo.New()

	// Health check route BEFORE middleware (as in router.go)
	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Apply middleware AFTER health check
	e.Use(SecurityHeadersMiddleware())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	// Even health check gets security headers (e.Use applies globally)
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"),
		"Health check endpoint should have security headers")
}

// TestSecurityHeadersIntegration_404Response verifies headers are set on 404 responses
func TestSecurityHeadersIntegration_404Response(t *testing.T) {
	e := echo.New()
	e.Use(SecurityHeadersMiddleware())

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	req.URL.Scheme = "https"
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)

	// 404 responses should still have security headers
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
}

// TestSecurityHeadersIntegration_500Response verifies headers are set on error responses
func TestSecurityHeadersIntegration_500Response(t *testing.T) {
	e := echo.New()
	e.Use(SecurityHeadersMiddleware())

	e.GET("/error", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusInternalServerError, "Internal error")
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	req.URL.Scheme = "https"
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)

	// Error responses should still have security headers
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
}
