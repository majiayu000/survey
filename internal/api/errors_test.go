package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// setupTestTracer initializes a test tracer provider that generates valid trace IDs
func setupTestTracer(t *testing.T) func() {
	t.Helper()
	// Create a trace provider with AlwaysSample to ensure spans are valid
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	// Return cleanup function
	return func() {
		_ = tp.Shutdown(context.Background())
	}
}

// TestGetTraceID tests extracting trace ID from context
func TestGetTraceID(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	tests := []struct {
		name     string
		setupCtx func() context.Context
		validate func(t *testing.T, traceID string)
	}{
		{
			name: "returns trace ID when span exists",
			setupCtx: func() context.Context {
				// Create a real tracer and start a span
				tracer := otel.Tracer("test")
				ctx, _ := tracer.Start(context.Background(), "test-span")
				return ctx
			},
			validate: func(t *testing.T, traceID string) {
				// Verify it's a valid trace ID (32 hex chars)
				assert.NotEmpty(t, traceID, "trace ID should not be empty when span exists")
				assert.Regexp(t, "^[0-9a-f]{32}$", traceID, "trace ID should be 32 hex chars")
			},
		},
		{
			name: "returns empty string when no span exists",
			setupCtx: func() context.Context {
				return context.Background()
			},
			validate: func(t *testing.T, traceID string) {
				assert.Empty(t, traceID, "trace ID should be empty when no span exists")
			},
		},
		{
			name: "extracts correct trace ID from span context",
			setupCtx: func() context.Context {
				tracer := otel.Tracer("test")
				ctx, _ := tracer.Start(context.Background(), "test-operation")
				return ctx
			},
			validate: func(t *testing.T, traceID string) {
				assert.NotEmpty(t, traceID)
				// Should be valid hex
				assert.Regexp(t, "^[0-9a-f]{32}$", traceID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			got := getTraceID(ctx)
			tt.validate(t, got)
		})
	}
}

// TestInternalServerError tests the helper that returns JSON error with trace ID
func TestInternalServerError(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	tests := []struct {
		name           string
		userMessage    string
		err            error
		setupContext   func(*echo.Echo) echo.Context
		wantStatus     int
		wantInBody     []string
		wantNotInBody  []string
	}{
		{
			name:        "sanitizes database error and includes trace ID",
			userMessage: "Failed to create survey",
			err:         errors.New("pq: relation \"surveys\" does not exist"),
			setupContext: func(e *echo.Echo) echo.Context {
				req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", nil)
				rec := httptest.NewRecorder()
				c := e.NewContext(req, rec)

				// Add trace context
				tracer := otel.Tracer("test")
				ctx, _ := tracer.Start(req.Context(), "test-span")
				c.SetRequest(req.WithContext(ctx))

				return c
			},
			wantStatus:    http.StatusInternalServerError,
			wantInBody:    []string{`"error":"Failed to create survey"`, `"details":"Reference:`},
			wantNotInBody: []string{"pq:", "relation", "surveys", "does not exist"},
		},
		{
			name:        "sanitizes file path error",
			userMessage: "Failed to retrieve survey",
			err:         errors.New("failed to open /var/lib/app/config.json: permission denied"),
			setupContext: func(e *echo.Echo) echo.Context {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/surveys/test", nil)
				rec := httptest.NewRecorder()
				c := e.NewContext(req, rec)

				// Add trace context
				tracer := otel.Tracer("test")
				ctx, _ := tracer.Start(req.Context(), "test-span")
				c.SetRequest(req.WithContext(ctx))

				return c
			},
			wantStatus:    http.StatusInternalServerError,
			wantInBody:    []string{`"error":"Failed to retrieve survey"`, `"details":"Reference:`},
			wantNotInBody: []string{"/var/lib/app", "config.json", "permission denied"},
		},
		{
			name:        "works without trace ID",
			userMessage: "Failed to retrieve results",
			err:         errors.New("internal error"),
			setupContext: func(e *echo.Echo) echo.Context {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/surveys/test/results", nil)
				rec := httptest.NewRecorder()
				// No trace context - just regular context
				return e.NewContext(req, rec)
			},
			wantStatus:    http.StatusInternalServerError,
			wantInBody:    []string{`"error":"Failed to retrieve results"`},
			wantNotInBody: []string{"internal error"},
		},
		{
			name:        "preserves user message exactly as provided",
			userMessage: "Failed to check slug availability",
			err:         errors.New("sql: connection refused"),
			setupContext: func(e *echo.Echo) echo.Context {
				req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", nil)
				rec := httptest.NewRecorder()
				return e.NewContext(req, rec)
			},
			wantStatus:    http.StatusInternalServerError,
			wantInBody:    []string{`"error":"Failed to check slug availability"`},
			wantNotInBody: []string{"sql:", "connection refused"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			c := tt.setupContext(e)

			err := InternalServerError(c, tt.userMessage, tt.err)
			require.NoError(t, err) // The helper should not return an error

			rec := c.Response().Writer.(*httptest.ResponseRecorder)
			assert.Equal(t, tt.wantStatus, rec.Code)

			body := rec.Body.String()
			for _, substr := range tt.wantInBody {
				assert.Contains(t, body, substr, "response body should contain: %s", substr)
			}

			for _, substr := range tt.wantNotInBody {
				assert.NotContains(t, body, substr, "response body should NOT contain: %s", substr)
			}
		})
	}
}

// TestInternalServerErrorLogsFullError tests that full error is logged
func TestInternalServerErrorLogsFullError(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	// This test verifies that the full error is logged server-side
	// We can't easily capture Echo's logger output, but we can verify
	// the function doesn't panic and returns proper response

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Add trace context
	tracer := otel.Tracer("test")
	ctx, _ := tracer.Start(req.Context(), "test-span")
	c.SetRequest(req.WithContext(ctx))

	// Call with a detailed error
	detailedErr := errors.New("database error: connection pool exhausted at /app/db/pool.go:123")
	err := InternalServerError(c, "Failed to create survey", detailedErr)

	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	// Verify the detailed error is NOT in the response
	body := rec.Body.String()
	assert.NotContains(t, body, "connection pool exhausted")
	assert.NotContains(t, body, "/app/db/pool.go")
}

// TestTraceIDConsistency verifies trace ID matches the span's trace ID
func TestTraceIDConsistency(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-operation")
	defer span.End()

	// Extract trace ID using our function
	traceID := getTraceID(ctx)

	// Get the actual trace ID from the span
	spanContext := trace.SpanFromContext(ctx).SpanContext()
	expectedTraceID := spanContext.TraceID().String()

	// They should match
	assert.Equal(t, expectedTraceID, traceID, "extracted trace ID should match span's trace ID")
}

// TestValidationError tests that validation errors are NOT sanitized
func TestValidationError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := ValidationError(c, "Invalid survey definition", "slug must be 3-50 alphanumeric characters")
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Validation errors should show the actual details
	body := rec.Body.String()
	assert.Contains(t, body, "Invalid survey definition")
	assert.Contains(t, body, "slug must be 3-50 alphanumeric characters")
}
