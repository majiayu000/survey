package generator

import (
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	// Anonymous rate limits: 5 requests per hour (default)
	// This protects our OpenAI budget from anonymous abuse
	// Override with AI_RATE_LIMIT_ANON_LIMIT and AI_RATE_LIMIT_ANON_WINDOW_HOURS
	DefaultAnonLimit  = 5
	DefaultAnonWindow = time.Hour

	// Authenticated rate limits: 20 requests per day (default)
	// More generous for authenticated users but still prevents abuse
	// Override with AI_RATE_LIMIT_AUTH_LIMIT and AI_RATE_LIMIT_AUTH_WINDOW_HOURS
	DefaultAuthLimit  = 20
	DefaultAuthWindow = 24 * time.Hour
)

// rateLimitEntry tracks requests for a single identifier (IP or DID)
type rateLimitEntry struct {
	count       int
	windowStart time.Time
}

// RateLimiter tracks and enforces rate limits for AI generation
// This is separate from OpenAI's API rate limits - this is business logic
// to prevent abuse of our OpenAI budget by limiting how many AI generations
// a user can request.
type RateLimiter struct {
	mu           sync.RWMutex
	anonLimit    int
	anonWindow   time.Duration
	authLimit    int
	authWindow   time.Duration
	anonTracking map[string]*rateLimitEntry // keyed by IP
	authTracking map[string]*rateLimitEntry // keyed by DID
}

// RateLimiterConfig holds configuration for rate limiting
type RateLimiterConfig struct {
	AnonLimit    int
	AnonWindow   time.Duration
	AuthLimit    int
	AuthWindow   time.Duration
}

// RateLimiterConfigFromEnv creates a RateLimiterConfig from environment variables
// Environment variables:
//   - AI_RATE_LIMIT_ANON_LIMIT: requests per window for anonymous users (default: 5)
//   - AI_RATE_LIMIT_ANON_WINDOW_HOURS: window duration in hours for anonymous users (default: 1)
//   - AI_RATE_LIMIT_AUTH_LIMIT: requests per window for authenticated users (default: 20)
//   - AI_RATE_LIMIT_AUTH_WINDOW_HOURS: window duration in hours for authenticated users (default: 24)
func RateLimiterConfigFromEnv() RateLimiterConfig {
	config := RateLimiterConfig{
		AnonLimit:  DefaultAnonLimit,
		AnonWindow: DefaultAnonWindow,
		AuthLimit:  DefaultAuthLimit,
		AuthWindow: DefaultAuthWindow,
	}

	if v := os.Getenv("AI_RATE_LIMIT_ANON_LIMIT"); v != "" {
		if val, err := strconv.Atoi(v); err == nil && val > 0 {
			config.AnonLimit = val
		}
	}

	if v := os.Getenv("AI_RATE_LIMIT_ANON_WINDOW_HOURS"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil && val > 0 {
			config.AnonWindow = time.Duration(val * float64(time.Hour))
		}
	}

	if v := os.Getenv("AI_RATE_LIMIT_AUTH_LIMIT"); v != "" {
		if val, err := strconv.Atoi(v); err == nil && val > 0 {
			config.AuthLimit = val
		}
	}

	if v := os.Getenv("AI_RATE_LIMIT_AUTH_WINDOW_HOURS"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil && val > 0 {
			config.AuthWindow = time.Duration(val * float64(time.Hour))
		}
	}

	return config
}

// NewRateLimiter creates a new rate limiter with configuration from environment variables
func NewRateLimiter() *RateLimiter {
	return NewRateLimiterWithConfig(RateLimiterConfigFromEnv())
}

// NewRateLimiterWithConfig creates a new rate limiter with the given configuration
func NewRateLimiterWithConfig(config RateLimiterConfig) *RateLimiter {
	return &RateLimiter{
		anonLimit:    config.AnonLimit,
		anonWindow:   config.AnonWindow,
		authLimit:    config.AuthLimit,
		authWindow:   config.AuthWindow,
		anonTracking: make(map[string]*rateLimitEntry),
		authTracking: make(map[string]*rateLimitEntry),
	}
}

// AllowAnonymous checks if an anonymous request (by IP) is allowed
func (rl *RateLimiter) AllowAnonymous(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	return rl.checkLimit(ip, rl.anonTracking, rl.anonLimit, rl.anonWindow)
}

// AllowAuthenticated checks if an authenticated request (by DID) is allowed
func (rl *RateLimiter) AllowAuthenticated(did string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	return rl.checkLimit(did, rl.authTracking, rl.authLimit, rl.authWindow)
}

// checkLimit is the internal logic for checking and updating rate limits
func (rl *RateLimiter) checkLimit(key string, tracking map[string]*rateLimitEntry, limit int, window time.Duration) bool {
	now := time.Now()

	entry, exists := tracking[key]
	if !exists {
		// First request
		tracking[key] = &rateLimitEntry{
			count:       1,
			windowStart: now,
		}
		return true
	}

	// Check if window has expired
	if now.Sub(entry.windowStart) > window {
		// Reset window
		entry.count = 1
		entry.windowStart = now
		return true
	}

	// Window still valid, check limit
	if entry.count >= limit {
		return false
	}

	// Increment counter
	entry.count++
	return true
}

// ResetAnonymous resets the rate limit for a specific IP (for testing)
func (rl *RateLimiter) ResetAnonymous(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.anonTracking, ip)
}

// ResetAuthenticated resets the rate limit for a specific DID (for testing)
func (rl *RateLimiter) ResetAuthenticated(did string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.authTracking, did)
}
