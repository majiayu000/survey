package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// SurveyResponsesTotal tracks total number of survey responses
	SurveyResponsesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "survey_responses_total",
			Help: "Total number of survey responses",
		},
		[]string{"survey_slug", "source"}, // source: "web" or "atproto"
	)

	// HTTPRequestDuration tracks HTTP request duration
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
)

// RegisterMetrics registers all Prometheus metrics
// This is called during application startup
func RegisterMetrics() {
	// Metrics are auto-registered via promauto, but we keep this function
	// for consistency and future manual registration if needed
}
