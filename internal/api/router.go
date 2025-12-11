package api

import (
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SetupRoutes configures all API routes
func SetupRoutes(e *echo.Echo, h *Handlers, hh *HealthHandlers) {
	// Health check and metrics endpoints (no middleware)
	e.GET("/health", hh.Health)
	e.GET("/health/ready", hh.Readiness)
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	// Apply middleware to all other routes
	e.Use(RequestIDMiddleware())
	e.Use(MetricsMiddleware())

	// JSON API routes - v1
	api := e.Group("/api/v1")

	// Survey management
	api.POST("/surveys", h.CreateSurvey)
	api.GET("/surveys", h.ListSurveys)
	api.GET("/surveys/:slug", h.GetSurvey)

	// Response submission and results
	api.POST("/surveys/:slug/responses", h.SubmitResponse)
	api.GET("/surveys/:slug/results", h.GetResults)

	// HTML routes (Templ handlers)
	web := e.Group("")

	// Survey list and creation
	web.GET("/surveys", h.ListSurveysHTML)
	web.GET("/surveys/new", h.CreateSurveyPageHTML)
	web.POST("/surveys", h.CreateSurveyHTML)

	// Survey viewing and voting
	web.GET("/surveys/:slug", h.GetSurveyHTML)
	web.POST("/surveys/:slug/responses", h.SubmitResponseHTML)

	// Results
	web.GET("/surveys/:slug/results", h.GetResultsHTML)
	web.GET("/surveys/:slug/results-partial", h.GetResultsPartialHTML)

	// Redirect root to surveys list
	e.GET("/", func(c echo.Context) error {
		return c.Redirect(302, "/surveys")
	})
}
