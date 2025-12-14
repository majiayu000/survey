package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// TestGetClientIP_DirectConnection tests that RemoteAddr is used when no proxy is involved
func TestGetClientIP_DirectConnection(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "203.0.113.100:12345" // Direct client connection
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	ip := getClientIP(c)
	assert.Equal(t, "203.0.113.100", ip, "Should use RemoteAddr for direct connections")
}

// TestGetClientIP_SpoofedXFF_FromPublicIP tests that X-Forwarded-For is ignored from non-trusted sources
func TestGetClientIP_SpoofedXFF_FromPublicIP(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "203.0.113.100:12345" // Public IP trying to spoof
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8") // Attacker-controlled header
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	ip := getClientIP(c)
	assert.Equal(t, "203.0.113.100", ip, "Should ignore X-Forwarded-For from non-trusted source")
}

// TestGetClientIP_TrustedProxy_PrivateNetwork tests that X-Forwarded-For is trusted from private networks
func TestGetClientIP_TrustedProxy_PrivateNetwork(t *testing.T) {
	testCases := []struct {
		name       string
		remoteAddr string
		xff        string
		expected   string
	}{
		{
			name:       "10.x.x.x private network",
			remoteAddr: "10.0.0.1:12345",
			xff:        "203.0.113.100",
			expected:   "203.0.113.100",
		},
		{
			name:       "172.16.x.x private network",
			remoteAddr: "172.16.0.1:12345",
			xff:        "203.0.113.100",
			expected:   "203.0.113.100",
		},
		{
			name:       "192.168.x.x private network",
			remoteAddr: "192.168.1.1:12345",
			xff:        "203.0.113.100",
			expected:   "203.0.113.100",
		},
		{
			name:       "localhost IPv4",
			remoteAddr: "127.0.0.1:12345",
			xff:        "203.0.113.100",
			expected:   "203.0.113.100",
		},
		{
			name:       "localhost IPv6",
			remoteAddr: "[::1]:12345",
			xff:        "203.0.113.100",
			expected:   "203.0.113.100",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tc.remoteAddr
			req.Header.Set("X-Forwarded-For", tc.xff)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			ip := getClientIP(c)
			assert.Equal(t, tc.expected, ip, "Should trust X-Forwarded-For from private network")
		})
	}
}

// TestGetClientIP_RightmostUntrustedIP tests that we use the rightmost untrusted IP from X-Forwarded-For
func TestGetClientIP_RightmostUntrustedIP(t *testing.T) {
	testCases := []struct {
		name       string
		remoteAddr string
		xff        string
		expected   string
		reason     string
	}{
		{
			name:       "Single client IP through private proxy",
			remoteAddr: "10.0.0.1:12345",
			xff:        "203.0.113.100",
			expected:   "203.0.113.100",
			reason:     "Only IP is the client",
		},
		{
			name:       "Client -> Public Proxy -> Private Proxy",
			remoteAddr: "10.0.0.1:12345",
			xff:        "203.0.113.100, 198.51.100.50",
			expected:   "198.51.100.50",
			reason:     "Rightmost untrusted IP is the last public proxy",
		},
		{
			name:       "Client -> Public Proxy1 -> Public Proxy2 -> Private Proxy",
			remoteAddr: "192.168.1.1:12345",
			xff:        "203.0.113.100, 198.51.100.50, 198.51.100.75",
			expected:   "198.51.100.75",
			reason:     "Rightmost untrusted IP is the last public proxy before our network",
		},
		{
			name:       "Client -> Private Proxy1 -> Private Proxy2 (internal chain)",
			remoteAddr: "10.0.0.2:12345",
			xff:        "203.0.113.100, 10.0.0.1",
			expected:   "203.0.113.100",
			reason:     "Skip trusted proxies, use rightmost untrusted",
		},
		{
			name:       "All private IPs (internal traffic)",
			remoteAddr: "10.0.0.2:12345",
			xff:        "192.168.1.100, 10.0.0.1",
			expected:   "192.168.1.100",
			reason:     "If all IPs are private, use leftmost as client",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tc.remoteAddr
			req.Header.Set("X-Forwarded-For", tc.xff)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			ip := getClientIP(c)
			assert.Equal(t, tc.expected, ip, tc.reason)
		})
	}
}

// TestGetClientIP_InvalidIPFormat tests handling of malformed IP addresses
func TestGetClientIP_InvalidIPFormat(t *testing.T) {
	testCases := []struct {
		name       string
		remoteAddr string
		xff        string
		expected   string
	}{
		{
			name:       "Empty X-Forwarded-For",
			remoteAddr: "10.0.0.1:12345",
			xff:        "",
			expected:   "10.0.0.1",
		},
		{
			name:       "Whitespace only",
			remoteAddr: "10.0.0.1:12345",
			xff:        "   ",
			expected:   "10.0.0.1",
		},
		{
			name:       "Invalid IP in chain",
			remoteAddr: "10.0.0.1:12345",
			xff:        "not-an-ip, 203.0.113.100",
			expected:   "203.0.113.100",
		},
		{
			name:       "Extra commas",
			remoteAddr: "10.0.0.1:12345",
			xff:        "203.0.113.100,,,",
			expected:   "203.0.113.100",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tc.remoteAddr
			req.Header.Set("X-Forwarded-For", tc.xff)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			ip := getClientIP(c)
			assert.Equal(t, tc.expected, ip)
		})
	}
}

// TestGetClientIP_AttackScenarios tests specific attack scenarios
func TestGetClientIP_AttackScenarios(t *testing.T) {
	testCases := []struct {
		name       string
		remoteAddr string
		xff        string
		expected   string
		attack     string
	}{
		{
			name:       "Attacker spoofs private IP",
			remoteAddr: "203.0.113.50:12345", // Real attacker IP (public)
			xff:        "10.0.0.1",           // Spoofed private IP
			expected:   "203.0.113.50",       // Should use RemoteAddr
			attack:     "Trying to appear as internal traffic",
		},
		{
			name:       "Attacker spoofs multiple IPs to bypass rate limit",
			remoteAddr: "203.0.113.50:12345",
			xff:        "1.2.3.4, 5.6.7.8, 9.10.11.12",
			expected:   "203.0.113.50",
			attack:     "Multiple fake IPs to appear as proxy chain",
		},
		{
			name:       "Attacker spoofs localhost",
			remoteAddr: "203.0.113.50:12345",
			xff:        "127.0.0.1",
			expected:   "203.0.113.50",
			attack:     "Trying to appear as local connection",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tc.remoteAddr
			req.Header.Set("X-Forwarded-For", tc.xff)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			ip := getClientIP(c)
			assert.Equal(t, tc.expected, ip, "Attack: %s", tc.attack)
		})
	}
}

// TODO: Add test for custom trusted proxies via environment variable
// func TestGetClientIP_CustomTrustedProxies(t *testing.T) {
// 	// Could be added via TRUSTED_PROXIES env var
// }
