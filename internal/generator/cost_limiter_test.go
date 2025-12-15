package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCostLimiter(t *testing.T) {
	t.Run("allows requests within budget", func(t *testing.T) {
		limiter := NewCostLimiter(10.0) // $10 budget

		// Simulate spending $5
		allowed := limiter.AllowRequest(5.0)
		assert.True(t, allowed, "should allow $5 request with $10 budget")
		assert.Equal(t, 5.0, limiter.GetSpent())
		assert.Equal(t, 5.0, limiter.GetRemaining())

		// Simulate spending another $3
		allowed = limiter.AllowRequest(3.0)
		assert.True(t, allowed, "should allow $3 request with $5 remaining")
		assert.Equal(t, 8.0, limiter.GetSpent())
		assert.Equal(t, 2.0, limiter.GetRemaining())
	})

	t.Run("denies requests exceeding budget", func(t *testing.T) {
		limiter := NewCostLimiter(10.0)

		// Spend up to budget
		limiter.AllowRequest(9.0)
		assert.Equal(t, 9.0, limiter.GetSpent())

		// Try to exceed budget
		allowed := limiter.AllowRequest(2.0)
		assert.False(t, allowed, "should deny request that would exceed budget")
		assert.Equal(t, 9.0, limiter.GetSpent(), "spending should not increase when denied")
	})

	t.Run("allows request exactly at budget", func(t *testing.T) {
		limiter := NewCostLimiter(10.0)

		// Spend exactly to budget
		allowed := limiter.AllowRequest(10.0)
		assert.True(t, allowed, "should allow request exactly at budget")
		assert.Equal(t, 10.0, limiter.GetSpent())
		assert.Equal(t, 0.0, limiter.GetRemaining())

		// Next request should fail
		allowed = limiter.AllowRequest(0.01)
		assert.False(t, allowed, "should deny request after budget exhausted")
	})

	t.Run("tracks multiple small requests", func(t *testing.T) {
		limiter := NewCostLimiter(1.0)

		// Make 10 requests of $0.10 each
		for i := 0; i < 10; i++ {
			allowed := limiter.AllowRequest(0.10)
			assert.True(t, allowed, "request %d should be allowed", i+1)
		}

		assert.InDelta(t, 1.0, limiter.GetSpent(), 0.001)

		// Next request should fail
		allowed := limiter.AllowRequest(0.01)
		assert.False(t, allowed, "should deny request after budget exhausted")
	})

	t.Run("reset clears spending", func(t *testing.T) {
		limiter := NewCostLimiter(10.0)

		// Spend some money
		limiter.AllowRequest(5.0)
		assert.Equal(t, 5.0, limiter.GetSpent())

		// Reset
		limiter.Reset()
		assert.Equal(t, 0.0, limiter.GetSpent())
		assert.Equal(t, 10.0, limiter.GetRemaining())

		// Should be able to spend full budget again
		allowed := limiter.AllowRequest(10.0)
		assert.True(t, allowed)
	})

	t.Run("thread-safe concurrent requests", func(t *testing.T) {
		limiter := NewCostLimiter(10.0)

		// Make 20 concurrent requests of $1 each
		results := make(chan bool, 20)
		for i := 0; i < 20; i++ {
			go func() {
				results <- limiter.AllowRequest(1.0)
			}()
		}

		// Collect results
		allowed := 0
		denied := 0
		for i := 0; i < 20; i++ {
			if <-results {
				allowed++
			} else {
				denied++
			}
		}

		// Should allow exactly 10 (budget) and deny 10
		assert.Equal(t, 10, allowed, "should allow exactly 10 $1 requests")
		assert.Equal(t, 10, denied, "should deny 10 requests exceeding budget")
		assert.Equal(t, 10.0, limiter.GetSpent())
	})

	t.Run("estimate token cost", func(t *testing.T) {
		limiter := NewCostLimiter(10.0)

		// GPT-4o mini pricing: $0.150/1M input tokens, $0.600/1M output tokens
		cost := limiter.EstimateTokenCost(1000, 500)
		// (1000 * 0.150 / 1_000_000) + (500 * 0.600 / 1_000_000)
		// = 0.00015 + 0.0003 = 0.00045
		assert.InDelta(t, 0.00045, cost, 0.000001)
	})
}
