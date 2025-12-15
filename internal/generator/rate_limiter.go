package generator

import (
	"sync"
	"time"
)

const (
	// Anonymous rate limits: 2 requests per hour
	// This protects our OpenAI budget from anonymous abuse
	DefaultAnonLimit  = 2
	DefaultAnonWindow = time.Hour

	// Authenticated rate limits: 10 requests per day
	// More generous for authenticated users but still prevents abuse
	DefaultAuthLimit  = 10
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

// NewRateLimiter creates a new rate limiter with default limits
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		anonLimit:    DefaultAnonLimit,
		anonWindow:   DefaultAnonWindow,
		authLimit:    DefaultAuthLimit,
		authWindow:   DefaultAuthWindow,
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
