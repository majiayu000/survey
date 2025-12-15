package generator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_Anonymous(t *testing.T) {
	limiter := NewRateLimiter()

	t.Run("allows up to 2 requests per hour for anonymous", func(t *testing.T) {
		ip := "192.168.1.1"

		// First request should succeed
		allowed := limiter.AllowAnonymous(ip)
		assert.True(t, allowed, "first request should be allowed")

		// Second request should succeed
		allowed = limiter.AllowAnonymous(ip)
		assert.True(t, allowed, "second request should be allowed")

		// Third request should be denied
		allowed = limiter.AllowAnonymous(ip)
		assert.False(t, allowed, "third request should be denied (exceeds 2/hr limit)")
	})

	t.Run("different IPs have independent limits", func(t *testing.T) {
		ip1 := "192.168.1.10"
		ip2 := "192.168.1.11"

		// Both IPs should be able to make requests
		assert.True(t, limiter.AllowAnonymous(ip1))
		assert.True(t, limiter.AllowAnonymous(ip2))
		assert.True(t, limiter.AllowAnonymous(ip1))
		assert.True(t, limiter.AllowAnonymous(ip2))

		// Both should hit their limits independently
		assert.False(t, limiter.AllowAnonymous(ip1))
		assert.False(t, limiter.AllowAnonymous(ip2))
	})

	t.Run("reset after window expires", func(t *testing.T) {
		// Create a limiter with custom window for testing
		limiter := &RateLimiter{
			anonLimit:     2,
			anonWindow:    100 * time.Millisecond, // Short window for testing
			authLimit:     10,
			authWindow:    time.Hour,
			anonTracking:  make(map[string]*rateLimitEntry),
			authTracking:  make(map[string]*rateLimitEntry),
		}

		ip := "192.168.1.20"

		// Use up the limit
		assert.True(t, limiter.AllowAnonymous(ip))
		assert.True(t, limiter.AllowAnonymous(ip))
		assert.False(t, limiter.AllowAnonymous(ip))

		// Wait for window to expire
		time.Sleep(150 * time.Millisecond)

		// Should be able to make requests again
		assert.True(t, limiter.AllowAnonymous(ip))
	})
}

func TestRateLimiter_Authenticated(t *testing.T) {
	limiter := NewRateLimiter()

	t.Run("allows up to 10 requests per day for authenticated", func(t *testing.T) {
		did := "did:plc:test123"

		// Should allow 10 requests
		for i := 0; i < 10; i++ {
			allowed := limiter.AllowAuthenticated(did)
			assert.True(t, allowed, "request %d should be allowed", i+1)
		}

		// 11th request should be denied
		allowed := limiter.AllowAuthenticated(did)
		assert.False(t, allowed, "11th request should be denied (exceeds 10/day limit)")
	})

	t.Run("different DIDs have independent limits", func(t *testing.T) {
		did1 := "did:plc:user1"
		did2 := "did:plc:user2"

		// Both DIDs should be able to make requests
		assert.True(t, limiter.AllowAuthenticated(did1))
		assert.True(t, limiter.AllowAuthenticated(did2))
		assert.True(t, limiter.AllowAuthenticated(did1))
		assert.True(t, limiter.AllowAuthenticated(did2))
	})

	t.Run("reset after window expires", func(t *testing.T) {
		limiter := &RateLimiter{
			anonLimit:     2,
			anonWindow:    time.Hour,
			authLimit:     10,
			authWindow:    100 * time.Millisecond, // Short window for testing
			anonTracking:  make(map[string]*rateLimitEntry),
			authTracking:  make(map[string]*rateLimitEntry),
		}

		did := "did:plc:test456"

		// Use up some of the limit
		assert.True(t, limiter.AllowAuthenticated(did))
		assert.True(t, limiter.AllowAuthenticated(did))
		assert.True(t, limiter.AllowAuthenticated(did))

		// Wait for window to expire
		time.Sleep(150 * time.Millisecond)

		// Counter should be reset
		// Make 10 requests to verify full limit is available
		for i := 0; i < 10; i++ {
			allowed := limiter.AllowAuthenticated(did)
			assert.True(t, allowed, "request %d should be allowed after reset", i+1)
		}
	})
}

func TestRateLimiter_Concurrent(t *testing.T) {
	t.Run("thread-safe for concurrent requests", func(t *testing.T) {
		limiter := NewRateLimiter()
		ip := "192.168.1.100"

		// Make concurrent requests
		results := make(chan bool, 5)
		for i := 0; i < 5; i++ {
			go func() {
				results <- limiter.AllowAnonymous(ip)
			}()
		}

		// Collect results
		allowed := 0
		denied := 0
		for i := 0; i < 5; i++ {
			if <-results {
				allowed++
			} else {
				denied++
			}
		}

		// Should allow exactly 2 and deny 3
		assert.Equal(t, 2, allowed, "should allow exactly 2 requests")
		assert.Equal(t, 3, denied, "should deny exactly 3 requests")
	})
}
