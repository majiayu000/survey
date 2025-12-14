package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecurityHeadersMiddleware_AllHeadersSet verifies all security headers are present
func TestSecurityHeadersMiddleware_AllHeadersSet(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Dummy handler
	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	}

	// Apply middleware
	h := SecurityHeadersMiddleware()(handler)
	err := h(c)
	require.NoError(t, err)

	// Verify X-Frame-Options
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"),
		"X-Frame-Options should prevent clickjacking")

	// Verify X-Content-Type-Options
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"),
		"X-Content-Type-Options should prevent MIME sniffing")

	// Verify X-XSS-Protection
	assert.Equal(t, "1; mode=block", rec.Header().Get("X-XSS-Protection"),
		"X-XSS-Protection should enable legacy XSS protection")

	// Verify Referrer-Policy
	assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"),
		"Referrer-Policy should control referrer information")
}

// TestSecurityHeadersMiddleware_HSTS verifies HSTS header for HTTPS
func TestSecurityHeadersMiddleware_HSTS(t *testing.T) {
	tests := []struct {
		name           string
		scheme         string
		expectedHSTS   string
		shouldHaveHSTS bool
	}{
		{
			name:           "HTTPS request gets HSTS header",
			scheme:         "https",
			expectedHSTS:   "max-age=31536000; includeSubDomains",
			shouldHaveHSTS: true,
		},
		{
			name:           "HTTP request does not get HSTS header",
			scheme:         "http",
			expectedHSTS:   "",
			shouldHaveHSTS: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.URL.Scheme = tt.scheme
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handler := func(c echo.Context) error {
				return c.String(http.StatusOK, "test")
			}

			h := SecurityHeadersMiddleware()(handler)
			err := h(c)
			require.NoError(t, err)

			if tt.shouldHaveHSTS {
				assert.Equal(t, tt.expectedHSTS, rec.Header().Get("Strict-Transport-Security"),
					"HTTPS requests should include HSTS header")
			} else {
				assert.Empty(t, rec.Header().Get("Strict-Transport-Security"),
					"HTTP requests should not include HSTS header")
			}
		})
	}
}

// TestSecurityHeadersMiddleware_CSP verifies Content-Security-Policy
func TestSecurityHeadersMiddleware_CSP(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	}

	h := SecurityHeadersMiddleware()(handler)
	err := h(c)
	require.NoError(t, err)

	csp := rec.Header().Get("Content-Security-Policy")
	assert.NotEmpty(t, csp, "Content-Security-Policy should be set")

	// Verify CSP contains expected directives
	assert.Contains(t, csp, "default-src 'self'", "CSP should restrict default sources to same origin")
	assert.Contains(t, csp, "script-src", "CSP should define script sources")
	assert.Contains(t, csp, "style-src", "CSP should define style sources")
	assert.Contains(t, csp, "img-src", "CSP should define image sources")
	assert.Contains(t, csp, "font-src", "CSP should define font sources")
}

// TestSecurityHeadersMiddleware_HTMLResponse verifies headers on HTML responses
func TestSecurityHeadersMiddleware_HTMLResponse(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/page", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := func(c echo.Context) error {
		return c.HTML(http.StatusOK, "<html><body>Test Page</body></html>")
	}

	h := SecurityHeadersMiddleware()(handler)
	err := h(c)
	require.NoError(t, err)

	// All security headers should be present on HTML responses
	assert.NotEmpty(t, rec.Header().Get("X-Frame-Options"))
	assert.NotEmpty(t, rec.Header().Get("X-Content-Type-Options"))
	assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
	assert.NotEmpty(t, rec.Header().Get("Referrer-Policy"))
}

// TestSecurityHeadersMiddleware_JSONResponse verifies headers on JSON responses
func TestSecurityHeadersMiddleware_JSONResponse(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}

	h := SecurityHeadersMiddleware()(handler)
	err := h(c)
	require.NoError(t, err)

	// All security headers should be present on JSON responses too
	assert.NotEmpty(t, rec.Header().Get("X-Frame-Options"))
	assert.NotEmpty(t, rec.Header().Get("X-Content-Type-Options"))
	assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
	assert.NotEmpty(t, rec.Header().Get("Referrer-Policy"))
}

// TestSecurityHeadersMiddleware_HandlerError verifies headers are set even when handler errors
func TestSecurityHeadersMiddleware_HandlerError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusInternalServerError, "Internal error")
	}

	h := SecurityHeadersMiddleware()(handler)
	err := h(c)
	require.Error(t, err) // Handler should error

	// Security headers should still be set even on error
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
}

// TestSecurityHeadersMiddleware_NoOverwrite verifies middleware doesn't overwrite existing headers
func TestSecurityHeadersMiddleware_NoOverwrite(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Pre-set a custom X-Frame-Options
	customValue := "SAMEORIGIN"

	handler := func(c echo.Context) error {
		c.Response().Header().Set("X-Frame-Options", customValue)
		return c.String(http.StatusOK, "test")
	}

	h := SecurityHeadersMiddleware()(handler)
	err := h(c)
	require.NoError(t, err)

	// Should NOT overwrite the custom value set by handler
	assert.Equal(t, customValue, rec.Header().Get("X-Frame-Options"),
		"Middleware should not overwrite handler-set headers")
}

// TestSecurityHeadersMiddleware_AllMethods verifies headers are set for all HTTP methods
func TestSecurityHeadersMiddleware_AllMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(method, "/test", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handler := func(c echo.Context) error {
				return c.NoContent(http.StatusOK)
			}

			h := SecurityHeadersMiddleware()(handler)
			err := h(c)
			require.NoError(t, err)

			assert.NotEmpty(t, rec.Header().Get("X-Frame-Options"),
				"Security headers should be set for %s requests", method)
		})
	}
}
