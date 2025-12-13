package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/openmeet-team/survey/internal/models"
	"github.com/stretchr/testify/assert"
)

// createTestSurvey is a helper to create a test survey in MockQueries
func createTestSurvey(mq *MockQueries, slug string) *models.Survey {
	survey := &models.Survey{
		ID:    uuid.New(),
		Slug:  slug,
		Title: "Test Survey",
		Definition: models.SurveyDefinition{
			Questions: []models.Question{
				{
					ID:       "q1",
					Text:     "Test Question",
					Type:     models.QuestionTypeSingle,
					Required: false,
					Options: []models.Option{
						{
							ID:   "opt1",
							Text: "Option 1",
						},
						{
							ID:   "opt2",
							Text: "Option 2",
						},
					},
				},
			},
		},
	}
	mq.surveys[slug] = survey
	mq.slugs[slug] = true
	mq.responsesBySurvey[survey.ID] = make(map[string]*models.Response)
	return survey
}

// TestRateLimiting_SurveyCreationEndpoint tests rate limiting on survey creation
func TestRateLimiting_SurveyCreationEndpoint(t *testing.T) {
	e := echo.New()

	// Create rate limiter config
	config := &RateLimiterConfig{
		SurveyCreation: NewIPRateLimiter(3, time.Minute), // Lower limit for testing
	}

	// Counter to track successful requests
	successCount := 0

	// Setup route with rate limiter and simple handler
	api := e.Group("/api/v1")
	api.POST("/surveys", func(c echo.Context) error {
		successCount++
		return c.JSON(http.StatusCreated, map[string]string{"status": "ok"})
	}, config.SurveyCreation.Middleware())

	// Make 3 requests - all should succeed
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code, "Request %d should succeed", i+1)
	}

	// 4th request should be rate limited
	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))

	// Verify it's the rate limiter, not a validation error
	var response map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"], "Rate limit")

	// Verify only 3 requests succeeded
	assert.Equal(t, 3, successCount, "Should have processed 3 successful requests")
}

// TestRateLimiting_VoteSubmissionEndpoint tests rate limiting on vote submission
func TestRateLimiting_VoteSubmissionEndpoint(t *testing.T) {
	e := echo.New()

	// Create rate limiter config
	config := &RateLimiterConfig{
		VoteSubmission: NewIPRateLimiter(5, time.Minute), // 5 votes per minute
	}

	successCount := 0

	// Setup route with rate limiter
	api := e.Group("/api/v1")
	api.POST("/surveys/:slug/responses", func(c echo.Context) error {
		successCount++
		return c.JSON(http.StatusCreated, map[string]string{"status": "ok"})
	}, config.VoteSubmission.Middleware())

	// Make 5 requests - all should succeed
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys/test-survey/responses", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code, "Request %d should succeed", i+1)
	}

	// 6th request should be rate limited
	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys/test-survey/responses", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))

	// Verify responses were created
	assert.Equal(t, 5, successCount, "Should have processed 5 successful requests")
}

// TestRateLimiting_DifferentEndpointsDifferentLimits tests that endpoints have independent limits
func TestRateLimiting_DifferentEndpointsDifferentLimits(t *testing.T) {
	e := echo.New()

	// Create rate limiter config with different limits
	config := &RateLimiterConfig{
		SurveyCreation: NewIPRateLimiter(2, time.Minute),
		VoteSubmission: NewIPRateLimiter(3, time.Minute),
	}

	surveyCount := 0
	voteCount := 0

	// Setup routes with different rate limiters
	api := e.Group("/api/v1")
	api.POST("/surveys", func(c echo.Context) error {
		surveyCount++
		return c.JSON(http.StatusCreated, map[string]string{"status": "ok"})
	}, config.SurveyCreation.Middleware())
	api.POST("/surveys/:slug/responses", func(c echo.Context) error {
		voteCount++
		return c.JSON(http.StatusCreated, map[string]string{"status": "ok"})
	}, config.VoteSubmission.Middleware())

	clientIP := "192.168.1.100:12345"

	// Make 2 survey creation requests - both should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", nil)
		req.RemoteAddr = clientIP
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
	}

	// Make 3 vote submissions - all should succeed (different rate limit)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys/test-survey/responses", nil)
		req.RemoteAddr = clientIP
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
	}

	// 3rd survey creation should be rate limited
	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", nil)
	req.RemoteAddr = clientIP
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	// 4th vote submission should be rate limited
	req = httptest.NewRequest(http.MethodPost, "/api/v1/surveys/test-survey/responses", nil)
	req.RemoteAddr = clientIP
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	assert.Equal(t, 2, surveyCount)
	assert.Equal(t, 3, voteCount)
}

// TestRateLimiting_XForwardedForHeader tests that X-Forwarded-For is used when present
func TestRateLimiting_XForwardedForHeader(t *testing.T) {
	e := echo.New()

	// Create rate limiter
	config := &RateLimiterConfig{
		SurveyCreation: NewIPRateLimiter(2, time.Minute),
	}

	successCount := 0

	api := e.Group("/api/v1")
	api.POST("/surveys", func(c echo.Context) error {
		successCount++
		return c.JSON(http.StatusCreated, map[string]string{"status": "ok"})
	}, config.SurveyCreation.Middleware())

	// Make 2 requests with X-Forwarded-For header
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", nil)
		req.RemoteAddr = "10.0.0.1:12345"                    // Load balancer IP
		req.Header.Set("X-Forwarded-For", "203.0.113.100")    // Real client IP
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
	}

	// 3rd request from same client IP (via X-Forwarded-For) should be rate limited
	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.100")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	// But request from different client IP should succeed
	req = httptest.NewRequest(http.MethodPost, "/api/v1/surveys", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.200") // Different client
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)

	assert.Equal(t, 3, successCount)
}
