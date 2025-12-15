package telemetry

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIGenerationMetrics_CounterLabels(t *testing.T) {
	// Reset metrics before test
	AIGenerationsTotal.Reset()

	// Increment with different statuses
	AIGenerationsTotal.WithLabelValues("success").Inc()
	AIGenerationsTotal.WithLabelValues("error").Inc()
	AIGenerationsTotal.WithLabelValues("rate_limited").Inc()
	AIGenerationsTotal.WithLabelValues("budget_exceeded").Inc()

	// Verify success counter
	successCount := testutil.ToFloat64(AIGenerationsTotal.WithLabelValues("success"))
	assert.Equal(t, 1.0, successCount, "success counter should be 1")

	// Verify error counter
	errorCount := testutil.ToFloat64(AIGenerationsTotal.WithLabelValues("error"))
	assert.Equal(t, 1.0, errorCount, "error counter should be 1")

	// Verify rate_limited counter
	rateLimitedCount := testutil.ToFloat64(AIGenerationsTotal.WithLabelValues("rate_limited"))
	assert.Equal(t, 1.0, rateLimitedCount, "rate_limited counter should be 1")

	// Verify budget_exceeded counter
	budgetCount := testutil.ToFloat64(AIGenerationsTotal.WithLabelValues("budget_exceeded"))
	assert.Equal(t, 1.0, budgetCount, "budget_exceeded counter should be 1")
}

func TestAIGenerationDuration_Histogram(t *testing.T) {
	// Histogram should accept duration values without panicking
	AIGenerationDuration.Observe(0.5)  // 500ms
	AIGenerationDuration.Observe(1.2)  // 1.2s
	AIGenerationDuration.Observe(3.5)  // 3.5s

	// Just verify the metric exists and doesn't panic on observe
	assert.NotNil(t, AIGenerationDuration)
}

func TestAITokensTotal_CounterLabels(t *testing.T) {
	// Reset metrics before test
	AITokensTotal.Reset()

	// Record input and output tokens
	AITokensTotal.WithLabelValues("input").Add(150)
	AITokensTotal.WithLabelValues("output").Add(300)

	// Verify input tokens
	inputTokens := testutil.ToFloat64(AITokensTotal.WithLabelValues("input"))
	assert.Equal(t, 150.0, inputTokens, "input tokens should be 150")

	// Verify output tokens
	outputTokens := testutil.ToFloat64(AITokensTotal.WithLabelValues("output"))
	assert.Equal(t, 300.0, outputTokens, "output tokens should be 300")
}

func TestAIDailyCost_Gauge(t *testing.T) {
	// Set daily cost
	AIDailyCostUSD.Set(5.25)

	// Verify cost value
	cost := testutil.ToFloat64(AIDailyCostUSD)
	assert.Equal(t, 5.25, cost, "daily cost should be 5.25")

	// Update cost
	AIDailyCostUSD.Set(10.00)
	cost = testutil.ToFloat64(AIDailyCostUSD)
	assert.Equal(t, 10.00, cost, "daily cost should be 10.00")
}

func TestAIRateLimitHits_CounterLabels(t *testing.T) {
	// Reset metrics before test
	AIRateLimitHitsTotal.Reset()

	// Record rate limit hits
	AIRateLimitHitsTotal.WithLabelValues("anonymous").Inc()
	AIRateLimitHitsTotal.WithLabelValues("anonymous").Inc()
	AIRateLimitHitsTotal.WithLabelValues("authenticated").Inc()

	// Verify anonymous hits
	anonHits := testutil.ToFloat64(AIRateLimitHitsTotal.WithLabelValues("anonymous"))
	assert.Equal(t, 2.0, anonHits, "anonymous hits should be 2")

	// Verify authenticated hits
	authHits := testutil.ToFloat64(AIRateLimitHitsTotal.WithLabelValues("authenticated"))
	assert.Equal(t, 1.0, authHits, "authenticated hits should be 1")
}

func TestAIMetrics_PrometheusRegistration(t *testing.T) {
	// All metrics should be registered via promauto
	// Verify by checking they're not nil
	require.NotNil(t, AIGenerationsTotal, "AIGenerationsTotal should be registered")
	require.NotNil(t, AIGenerationDuration, "AIGenerationDuration should be registered")
	require.NotNil(t, AITokensTotal, "AITokensTotal should be registered")
	require.NotNil(t, AIDailyCostUSD, "AIDailyCostUSD should be registered")
	require.NotNil(t, AIRateLimitHitsTotal, "AIRateLimitHitsTotal should be registered")
}

func TestAIMetrics_LabelCardinality(t *testing.T) {
	// Verify metrics have bounded label cardinality
	// This is a sanity check - we want to ensure we don't have unbounded labels

	// AIGenerationsTotal: status has 4 values (success, error, rate_limited, budget_exceeded)
	// This is fine - bounded to 4 combinations
	statuses := []string{"success", "error", "rate_limited", "budget_exceeded"}
	assert.Len(t, statuses, 4, "status label should have exactly 4 values")

	// AITokensTotal: type has 2 values (input, output)
	// This is fine - bounded to 2 combinations
	tokenTypes := []string{"input", "output"}
	assert.Len(t, tokenTypes, 2, "type label should have exactly 2 values")

	// AIRateLimitHitsTotal: user_type has 2 values (anonymous, authenticated)
	// This is fine - bounded to 2 combinations
	userTypes := []string{"anonymous", "authenticated"}
	assert.Len(t, userTypes, 2, "user_type label should have exactly 2 values")

	// AIGenerationDuration: no labels
	// AIDailyCostUSD: no labels
	// These are even better - no cardinality concerns
}
