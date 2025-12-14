package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/trace"
)

// getTraceID extracts the trace ID from the OpenTelemetry span context
// Returns empty string if no active span exists
func getTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return ""
	}
	return span.SpanContext().TraceID().String()
}

// InternalServerError returns a sanitized 500 error response to the client
// and logs the full error details server-side with the trace ID for debugging
//
// Parameters:
//   - c: Echo context
//   - userMessage: Safe message to show to user (e.g., "Failed to create survey")
//   - err: The actual error (will be logged but NOT sent to client)
//
// Example:
//
//	if err := db.Query(...); err != nil {
//	    return InternalServerError(c, "Failed to retrieve surveys", err)
//	}
//
// Client sees: {"error": "Failed to retrieve surveys", "details": "Reference: abc123..."}
// Server logs: [abc123...] Failed to retrieve surveys: pq: connection refused
func InternalServerError(c echo.Context, userMessage string, err error) error {
	traceID := getTraceID(c.Request().Context())

	// Log the FULL error server-side with trace ID
	if traceID != "" {
		c.Logger().Errorf("[%s] %s: %v", traceID, userMessage, err)
	} else {
		c.Logger().Errorf("%s: %v", userMessage, err)
	}

	// Build sanitized response for client
	response := ErrorResponse{
		Error: userMessage,
	}

	// Include trace ID if available (safe to show - it's just a reference)
	if traceID != "" {
		response.Details = fmt.Sprintf("Reference: %s", traceID)
	}

	return c.JSON(http.StatusInternalServerError, response)
}

// ValidationError returns a 400 error response with full details
// Validation errors are safe to show because they're controlled messages
//
// Parameters:
//   - c: Echo context
//   - message: Error message (e.g., "Invalid survey definition")
//   - details: Validation details (e.g., "slug must be 3-50 characters")
//
// Example:
//
//	if err := validateSlug(slug); err != nil {
//	    return ValidationError(c, "Invalid slug", err.Error())
//	}
func ValidationError(c echo.Context, message string, details string) error {
	return c.JSON(http.StatusBadRequest, ErrorResponse{
		Error:   message,
		Details: details,
	})
}
