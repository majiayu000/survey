package generator

import "sync"

const (
	// GPT-4o mini pricing (as of Dec 2024)
	// https://openai.com/api/pricing/
	InputTokenCostPer1M  = 0.150 // $0.150 per 1M input tokens
	OutputTokenCostPer1M = 0.600 // $0.600 per 1M output tokens
)

// CostLimiter tracks OpenAI API spending and enforces daily budget limits.
// This is the kill switch mentioned in the security requirements - if we hit
// the daily budget, we stop making OpenAI calls.
type CostLimiter struct {
	mu     sync.Mutex
	budget float64 // Daily budget in USD
	spent  float64 // Amount spent so far today
}

// NewCostLimiter creates a new cost limiter with the specified daily budget
func NewCostLimiter(dailyBudget float64) *CostLimiter {
	return &CostLimiter{
		budget: dailyBudget,
		spent:  0,
	}
}

// AllowRequest checks if a request with the given cost can be made
// Returns true and increments spending if within budget, false otherwise
func (cl *CostLimiter) AllowRequest(cost float64) bool {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if cl.spent+cost > cl.budget {
		return false
	}

	cl.spent += cost
	return true
}

// GetSpent returns the total amount spent so far
func (cl *CostLimiter) GetSpent() float64 {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return cl.spent
}

// GetRemaining returns the remaining budget
func (cl *CostLimiter) GetRemaining() float64 {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return cl.budget - cl.spent
}

// Reset clears the spending counter (typically called daily)
func (cl *CostLimiter) Reset() {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.spent = 0
}

// EstimateTokenCost calculates the estimated cost for a given number of tokens
// Based on GPT-4o mini pricing
func (cl *CostLimiter) EstimateTokenCost(inputTokens, outputTokens int) float64 {
	inputCost := float64(inputTokens) * InputTokenCostPer1M / 1_000_000
	outputCost := float64(outputTokens) * OutputTokenCostPer1M / 1_000_000
	return inputCost + outputCost
}
