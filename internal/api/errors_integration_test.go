package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/openmeet-team/survey/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// TestErrorSanitization_EndToEnd verifies that internal errors are sanitized
// and validation errors are not
func TestErrorSanitization_EndToEnd(t *testing.T) {
	// Setup tracer for realistic trace IDs
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(t.Context())

	tests := []struct {
		name           string
		endpoint       string
		method         string
		body           string
		mockError      error
		wantStatus     int
		wantInBody     []string
		wantNotInBody  []string
	}{
		{
			name:       "500 error sanitizes database error",
			endpoint:   "/api/v1/surveys/test-survey",
			method:     http.MethodGet,
			mockError:  errors.New("pq: relation \"surveys\" does not exist"),
			wantStatus: http.StatusInternalServerError,
			wantInBody: []string{
				`"error":"Failed to retrieve survey"`,
				`"details":"Reference:`,
			},
			wantNotInBody: []string{
				"pq:",
				"relation",
				"does not exist",
			},
		},
		{
			name:       "400 error shows validation details",
			endpoint:   "/api/v1/surveys",
			method:     http.MethodPost,
			body:       `{"slug":"a","definition":"invalid"}`,
			wantStatus: http.StatusBadRequest,
			wantInBody: []string{
				"Invalid survey definition",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Echo with otelecho middleware
			e := echo.New()
			e.Use(otelecho.Middleware("test-api"))

			// Create mock that returns error
			mockQueries := &MockQueriesWithError{
				err: tt.mockError,
			}
			handlers := NewHandlers(mockQueries)

			// Setup routes
			e.GET("/api/v1/surveys/:slug", handlers.GetSurvey)
			e.POST("/api/v1/surveys", handlers.CreateSurvey)

			// Make request
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.endpoint, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.endpoint, nil)
			}
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			if tt.wantStatus != 0 {
				assert.Equal(t, tt.wantStatus, rec.Code)
			}

			body := rec.Body.String()
			for _, substr := range tt.wantInBody {
				assert.Contains(t, body, substr, "response should contain: %s", substr)
			}

			for _, substr := range tt.wantNotInBody {
				assert.NotContains(t, body, substr, "response should NOT contain: %s", substr)
			}
		})
	}
}

// MockQueriesWithError is a mock that always returns an error
type MockQueriesWithError struct {
	MockQueries
	err error
}

func (m *MockQueriesWithError) GetSurveyBySlug(ctx context.Context, slug string) (*models.Survey, error) {
	if m.err != nil {
		return nil, m.err
	}
	return nil, sql.ErrNoRows
}

func (m *MockQueriesWithError) CreateSurvey(ctx context.Context, s *models.Survey) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

// TestTraceIDInRealHandler verifies trace ID appears in actual handler errors
func TestTraceIDInRealHandler(t *testing.T) {
	// Setup tracer
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(t.Context())

	// Setup Echo with otelecho middleware
	e := echo.New()
	e.Use(otelecho.Middleware("test-api"))

	// Create mock that returns database error
	mockQueries := &MockQueriesWithError{
		err: errors.New("pq: connection refused"),
	}
	handlers := NewHandlers(mockQueries)

	// Setup route
	e.GET("/api/v1/surveys/:slug", handlers.GetSurvey)

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveys/test", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Verify response
	require.Equal(t, http.StatusInternalServerError, rec.Code)

	body := rec.Body.String()

	// Should have user-friendly message
	assert.Contains(t, body, "Failed to retrieve survey")

	// Should have trace ID reference
	assert.Contains(t, body, "Reference:")

	// Should NOT leak internal details
	assert.NotContains(t, body, "pq:")
	assert.NotContains(t, body, "connection refused")
}
