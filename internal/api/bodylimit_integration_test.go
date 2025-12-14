package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBodyLimitIntegration tests body limits with realistic payloads
func TestBodyLimitIntegration(t *testing.T) {
	t.Run("large survey YAML is rejected", func(t *testing.T) {
		e := echo.New()
		mq := NewMockQueries()
		h := &Handlers{queries: mq}

		setupRoutesWithBodyLimits(e, h)

		// Create a large YAML payload (over 100KB)
		largeYAML := `title: Test Survey
description: ` + string(bytes.Repeat([]byte("This is a very long description. "), 4000)) + `
questions:
  - text: Question 1
    type: single_choice
    options:
      - Option A
      - Option B
`
		require.Greater(t, len(largeYAML), 100*1024, "Test payload should be > 100KB")

		req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewBufferString(largeYAML))
		req.Header.Set(echo.HeaderContentType, "application/x-yaml")
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	})

	t.Run("normal survey YAML is accepted", func(t *testing.T) {
		e := echo.New()
		mq := NewMockQueries()
		h := &Handlers{queries: mq}

		setupRoutesWithBodyLimits(e, h)

		// Normal sized YAML
		normalYAML := `title: Test Survey
description: A normal survey
questions:
  - text: What is your favorite color?
    type: single_choice
    options:
      - Red
      - Blue
      - Green
`
		require.Less(t, len(normalYAML), 100*1024, "Test payload should be < 100KB")

		req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewBufferString(normalYAML))
		req.Header.Set(echo.HeaderContentType, "application/x-yaml")
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Should not be rejected due to body size (may fail for other reasons like validation)
		assert.NotEqual(t, http.StatusRequestEntityTooLarge, rec.Code)
	})

	t.Run("large response submission is rejected", func(t *testing.T) {
		e := echo.New()
		mq := NewMockQueries()
		h := &Handlers{queries: mq}

		setupRoutesWithBodyLimits(e, h)

		// Create a response payload over 10KB
		largeResponse := `{
			"answers": {
				"q1": "` + string(bytes.Repeat([]byte("very long answer "), 1000)) + `"
			}
		}`
		require.Greater(t, len(largeResponse), 10*1024, "Test payload should be > 10KB")

		req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys/test-slug/responses", bytes.NewBufferString(largeResponse))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	})

	t.Run("normal response submission is accepted", func(t *testing.T) {
		e := echo.New()
		mq := NewMockQueries()
		h := &Handlers{queries: mq}

		setupRoutesWithBodyLimits(e, h)

		// Normal sized response
		normalResponse := `{
			"answers": {
				"q1": "Blue",
				"q2": "Developer"
			}
		}`
		require.Less(t, len(normalResponse), 10*1024, "Test payload should be < 10KB")

		req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys/test-slug/responses", bytes.NewBufferString(normalResponse))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Should not be rejected due to body size
		assert.NotEqual(t, http.StatusRequestEntityTooLarge, rec.Code)
	})
}

// TestBodyLimitErrorMessage verifies that 413 responses are clear
func TestBodyLimitErrorMessage(t *testing.T) {
	e := echo.New()
	mq := NewMockQueries()
	h := &Handlers{queries: mq}

	setupRoutesWithBodyLimits(e, h)

	// Send oversized request
	largeBody := bytes.Repeat([]byte("a"), 200*1024)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewReader(largeBody))
	req.Header.Set(echo.HeaderContentType, "application/x-yaml")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	// Echo's default error handler should provide a clear message
	assert.Contains(t, rec.Body.String(), "Request Entity Too Large")
}
