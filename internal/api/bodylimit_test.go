package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestBodyLimitMiddleware(t *testing.T) {
	tests := []struct {
		name                string
		route               string
		bodySize            int
		expectBodyLimitError bool
		description         string
	}{
		{
			name:                "survey creation within limit",
			route:               "/api/v1/surveys",
			bodySize:            50 * 1024, // 50KB (under 100KB limit)
			expectBodyLimitError: false,
			description:         "Should accept survey creation under 100KB",
		},
		{
			name:                "survey creation exceeds limit",
			route:               "/api/v1/surveys",
			bodySize:            150 * 1024, // 150KB (over 100KB limit)
			expectBodyLimitError: true,
			description:         "Should reject survey creation over 100KB",
		},
		{
			name:                "response submission within limit",
			route:               "/api/v1/surveys/test-slug/responses",
			bodySize:            5 * 1024, // 5KB (under 10KB limit)
			expectBodyLimitError: false,
			description:         "Should accept response submission under 10KB",
		},
		{
			name:                "response submission exceeds limit",
			route:               "/api/v1/surveys/test-slug/responses",
			bodySize:            15 * 1024, // 15KB (over 10KB limit)
			expectBodyLimitError: true,
			description:         "Should reject response submission over 10KB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create Echo instance with routes configured
			e := echo.New()
			mq := NewMockQueries()
			h := &Handlers{queries: mq}

			// Setup routes with body limits
			setupRoutesWithBodyLimits(e, h)

			// Create a large body of the specified size
			body := bytes.Repeat([]byte("a"), tt.bodySize)

			// Create request
			req := httptest.NewRequest(http.MethodPost, tt.route, bytes.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			// Serve
			e.ServeHTTP(rec, req)

			// Assert
			if tt.expectBodyLimitError {
				assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, tt.description)
			} else {
				// Body was accepted (may get other errors from handler, but not 413)
				assert.NotEqual(t, http.StatusRequestEntityTooLarge, rec.Code, tt.description)
			}
		})
	}
}

func TestBodyLimitHTML(t *testing.T) {
	tests := []struct {
		name                string
		route               string
		bodySize            int
		expectBodyLimitError bool
		description         string
	}{
		{
			name:                "HTML survey creation within limit",
			route:               "/surveys",
			bodySize:            50 * 1024, // 50KB
			expectBodyLimitError: false,
			description:         "Should accept HTML survey creation under 100KB",
		},
		{
			name:                "HTML survey creation exceeds limit",
			route:               "/surveys",
			bodySize:            150 * 1024, // 150KB
			expectBodyLimitError: true,
			description:         "Should reject HTML survey creation over 100KB",
		},
		{
			name:                "HTML response submission within limit",
			route:               "/surveys/test-slug/responses",
			bodySize:            5 * 1024, // 5KB
			expectBodyLimitError: false,
			description:         "Should accept HTML response submission under 10KB",
		},
		{
			name:                "HTML response submission exceeds limit",
			route:               "/surveys/test-slug/responses",
			bodySize:            15 * 1024, // 15KB
			expectBodyLimitError: true,
			description:         "Should reject HTML response submission over 10KB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create Echo instance with routes configured
			e := echo.New()
			mq := NewMockQueries()
			h := &Handlers{queries: mq}

			// Setup routes with body limits
			setupRoutesWithBodyLimits(e, h)

			// Create a large body of the specified size
			body := strings.Repeat("field=value&", tt.bodySize/12)

			// Create request
			req := httptest.NewRequest(http.MethodPost, tt.route, strings.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rec := httptest.NewRecorder()

			// Serve
			e.ServeHTTP(rec, req)

			// Assert
			if tt.expectBodyLimitError {
				assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, tt.description)
			} else {
				// Body was accepted (may get other errors from handler, but not 413)
				assert.NotEqual(t, http.StatusRequestEntityTooLarge, rec.Code, tt.description)
			}
		})
	}
}

// setupRoutesWithBodyLimits is a test helper that sets up routes with body limits
func setupRoutesWithBodyLimits(e *echo.Echo, h *Handlers) {
	// Create rate limiters
	rateLimiters := NewRateLimiterConfig()

	// Create body limit config
	bodyLimits := DefaultBodyLimitConfig()

	// JSON API routes
	api := e.Group("/api/v1")
	api.POST("/surveys", h.CreateSurvey, rateLimiters.SurveyCreation.Middleware(), NewBodyLimitMiddleware(bodyLimits.SurveyCreation))
	api.POST("/surveys/:slug/responses", h.SubmitResponse, rateLimiters.VoteSubmission.Middleware(), NewBodyLimitMiddleware(bodyLimits.ResponseSubmission))

	// HTML routes
	web := e.Group("")
	web.POST("/surveys", h.CreateSurveyHTML, rateLimiters.SurveyCreation.Middleware(), NewBodyLimitMiddleware(bodyLimits.SurveyCreation))
	web.POST("/surveys/:slug/responses", h.SubmitResponseHTML, rateLimiters.VoteSubmission.Middleware(), NewBodyLimitMiddleware(bodyLimits.ResponseSubmission))
}
