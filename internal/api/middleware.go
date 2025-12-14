package api

import (
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/openmeet-team/survey/internal/telemetry"
)

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()

			// Check if request ID already exists in header
			rid := req.Header.Get(echo.HeaderXRequestID)
			if rid == "" {
				rid = uuid.New().String()
			}

			// Set request ID in response header
			res.Header().Set(echo.HeaderXRequestID, rid)

			// Store in context for logging
			c.Set("request_id", rid)

			return next(c)
		}
	}
}

// MetricsMiddleware records HTTP request metrics for Prometheus
func MetricsMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			// Process request
			err := next(c)

			// Record duration
			duration := time.Since(start).Seconds()
			status := strconv.Itoa(c.Response().Status)
			method := c.Request().Method

			// Use route pattern (e.g., /surveys/:slug) not actual path to bound cardinality
			route := c.Path()
			if route == "" {
				route = "unknown" // Don't fall back to actual path - would explode cardinality
			}

			telemetry.HTTPRequestDuration.WithLabelValues(method, route, status).Observe(duration)

			return err
		}
	}
}

// BodyLimitConfig defines body size limits for different route types
type BodyLimitConfig struct {
	SurveyCreation   string
	ResponseSubmission string
	GeneralAPI       string
}

// DefaultBodyLimitConfig returns the default body size limits
func DefaultBodyLimitConfig() BodyLimitConfig {
	return BodyLimitConfig{
		SurveyCreation:     "100KB", // Survey YAML definitions
		ResponseSubmission: "10KB",  // Survey responses
		GeneralAPI:         "1MB",   // Default for other endpoints
	}
}

// NewBodyLimitMiddleware creates a body limit middleware with the given limit
func NewBodyLimitMiddleware(limit string) echo.MiddlewareFunc {
	return middleware.BodyLimit(limit)
}
