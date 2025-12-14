package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/openmeet-team/survey/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests for short URL routes

func TestShortURLIntegration_SlugRedirect(t *testing.T) {
	e := echo.New()
	mq := NewMockQueries()
	h := NewHandlers(mq)

	// Create test survey
	survey := &models.Survey{
		ID:    uuid.New(),
		Slug:  "test-redirect",
		Title: "Test Redirect Survey",
		Definition: models.SurveyDefinition{
			Questions: []models.Question{
				{
					ID:       "q1",
					Text:     "Test?",
					Type:     models.QuestionTypeSingle,
					Required: true,
					Options:  []models.Option{{ID: "a", Text: "A"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mq.CreateSurvey(context.Background(), survey)

	// Setup route
	e.GET("/s/:slug", h.ShortSlugURL)

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/s/test-redirect", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/surveys/test-redirect", rec.Header().Get("Location"))
}

func TestShortURLIntegration_ATProtoRedirect(t *testing.T) {
	e := echo.New()
	mq := NewMockQueries()
	h := NewHandlers(mq)

	// Create test survey with AT URI
	did := "did:plc:integration123"
	rkey := "3labc123xyz"
	uri := fmt.Sprintf("at://%s/net.openmeet.survey/%s", did, rkey)

	survey := &models.Survey{
		ID:    uuid.New(),
		URI:   &uri,
		Slug:  "atproto-redirect",
		Title: "ATProto Redirect Survey",
		Definition: models.SurveyDefinition{
			Questions: []models.Question{
				{
					ID:       "q1",
					Text:     "Test?",
					Type:     models.QuestionTypeSingle,
					Required: true,
					Options:  []models.Option{{ID: "a", Text: "A"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mq.CreateSurvey(context.Background(), survey)

	// Setup route
	e.GET("/at/:did/:rkey", h.ATProtoURL)

	// Make request
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/at/%s/%s", did, rkey), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/surveys/atproto-redirect", rec.Header().Get("Location"))
}

func TestShortURLIntegration_404Handling(t *testing.T) {
	e := echo.New()
	mq := NewMockQueries()
	h := NewHandlers(mq)

	// Setup routes
	e.GET("/s/:slug", h.ShortSlugURL)
	e.GET("/at/:did/:rkey", h.ATProtoURL)

	// Test /s/:slug 404
	req := httptest.NewRequest(http.MethodGet, "/s/nonexistent-survey", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// Test /at/:did/:rkey 404
	req = httptest.NewRequest(http.MethodGet, "/at/did:plc:notfound/missing123", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestShortURLIntegration_WithRateLimiting(t *testing.T) {
	e := echo.New()
	mq := NewMockQueries()
	h := NewHandlers(mq)

	// Create test survey
	survey := &models.Survey{
		ID:    uuid.New(),
		Slug:  "rate-limited",
		Title: "Rate Limited Survey",
		Definition: models.SurveyDefinition{
			Questions: []models.Question{
				{
					ID:       "q1",
					Text:     "Test?",
					Type:     models.QuestionTypeSingle,
					Required: true,
					Options:  []models.Option{{ID: "a", Text: "A"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mq.CreateSurvey(context.Background(), survey)

	// Setup route with rate limiting
	rateLimiters := NewRateLimiterConfig()
	e.GET("/s/:slug", h.ShortSlugURL, rateLimiters.GeneralAPI.Middleware())

	// Make multiple requests rapidly
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/s/rate-limited", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.100")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// All should succeed with general API rate limit
		require.Equal(t, http.StatusSeeOther, rec.Code, "Request %d should succeed", i+1)
	}
}
