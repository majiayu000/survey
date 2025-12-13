package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

// IPRateLimiter manages rate limiters per IP address
type IPRateLimiter struct {
	limiters map[string]*rateLimiterEntry
	mu       sync.Mutex
	rate     rate.Limit
	burst    int
	duration time.Duration
}

// rateLimiterEntry holds a rate limiter and its last access time for cleanup
type rateLimiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// NewIPRateLimiter creates a new IP-based rate limiter
// requestsPerDuration: number of requests allowed
// duration: time window for the rate limit
func NewIPRateLimiter(requestsPerDuration int, duration time.Duration) *IPRateLimiter {
	limiter := &IPRateLimiter{
		limiters: make(map[string]*rateLimiterEntry),
		rate:     rate.Limit(float64(requestsPerDuration) / duration.Seconds()),
		burst:    requestsPerDuration,
		duration: duration,
	}

	// Start background cleanup goroutine
	go limiter.cleanupLoop()

	return limiter
}

// getLimiter retrieves or creates a rate limiter for the given IP
func (rl *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.limiters[ip]
	if !exists {
		limiter := rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[ip] = &rateLimiterEntry{
			limiter:    limiter,
			lastAccess: time.Now(),
		}
		return limiter
	}

	// Update last access time
	entry.lastAccess = time.Now()
	return entry.limiter
}

// cleanupLoop removes rate limiters that haven't been accessed recently
func (rl *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.duration)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup removes stale rate limiters
func (rl *IPRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, entry := range rl.limiters {
		if now.Sub(entry.lastAccess) > rl.duration*2 {
			delete(rl.limiters, ip)
		}
	}
}

// getIP extracts the IP address from the request
// Priority: X-Forwarded-For header, then RemoteAddr
func getIP(c echo.Context) string {
	// Check X-Forwarded-For header (proxy/load balancer)
	forwarded := c.Request().Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can contain multiple IPs: "client, proxy1, proxy2"
		// We want the leftmost (original client) IP
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			return clientIP
		}
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(c.Request().RemoteAddr)
	if err != nil {
		return c.Request().RemoteAddr
	}
	return ip
}

// Middleware returns an Echo middleware function that enforces rate limiting
func (rl *IPRateLimiter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := getIP(c)
			limiter := rl.getLimiter(ip)

			if !limiter.Allow() {
				// Calculate retry after duration
				reservation := limiter.Reserve()
				delay := reservation.Delay()
				reservation.Cancel()

				c.Response().Header().Set("Retry-After", delay.String())

				return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
					"error":   "Rate limit exceeded",
					"message": "Too many requests. Please try again later.",
				})
			}

			return next(c)
		}
	}
}

// RateLimiterConfig holds different rate limiters for different endpoint types
type RateLimiterConfig struct {
	SurveyCreation *IPRateLimiter
	VoteSubmission *IPRateLimiter
	GeneralAPI     *IPRateLimiter
	OAuth          *IPRateLimiter
}

// NewRateLimiterConfig creates rate limiters with the specified limits
func NewRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		SurveyCreation: NewIPRateLimiter(5, time.Minute),   // 5 requests per minute
		VoteSubmission: NewIPRateLimiter(10, time.Minute),  // 10 requests per minute
		GeneralAPI:     NewIPRateLimiter(60, time.Minute),  // 60 requests per minute
		OAuth:          NewIPRateLimiter(10, time.Minute),  // 10 requests per minute
	}
}
