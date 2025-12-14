package api

import (
	"net"
	"strings"

	"github.com/labstack/echo/v4"
)

// Trusted proxy CIDR ranges (private networks + localhost)
// These are the only sources we trust to set X-Forwarded-For
var trustedProxyCIDRs = []string{
	"10.0.0.0/8",       // Private network (Class A)
	"172.16.0.0/12",    // Private network (Class B)
	"192.168.0.0/16",   // Private network (Class C)
	"127.0.0.0/8",      // Loopback IPv4
	"::1/128",          // Loopback IPv6
	"fc00::/7",         // Unique local address (IPv6 private)
	"fe80::/10",        // Link-local address (IPv6)
}

var trustedProxyNets []*net.IPNet

func init() {
	// Parse all trusted proxy CIDRs at startup
	for _, cidr := range trustedProxyCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			// This should never happen with hardcoded CIDRs
			panic("Failed to parse trusted proxy CIDR: " + cidr + ": " + err.Error())
		}
		trustedProxyNets = append(trustedProxyNets, ipNet)
	}
}

// isTrustedProxy checks if an IP address is in the trusted proxy ranges
func isTrustedProxy(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, ipNet := range trustedProxyNets {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// parseIP extracts an IP address from a string that might include a port
// Returns the IP part only, or empty string if invalid
func parseIP(addr string) string {
	// Try to split host:port first
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// No port, treat as plain IP
		host = addr
	}

	// Validate it's a real IP
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return ""
	}

	return ip.String()
}

// getClientIP extracts the real client IP from the request using secure logic
//
// Security considerations:
// 1. Only trusts X-Forwarded-For when request comes from a trusted proxy (private network)
// 2. Uses rightmost untrusted IP from X-Forwarded-For (more secure than leftmost)
// 3. Falls back to RemoteAddr when X-Forwarded-For is not from trusted source
// 4. Validates all IPs to prevent injection attacks
//
// Why rightmost untrusted IP?
// X-Forwarded-For format: "client, proxy1, proxy2, proxy3"
// Each proxy appends to the right. We can only trust IPs added by OUR proxies.
// Walk from right to left, skip our trusted proxies, return first untrusted IP.
//
// Example: "spoofed-ip, real-client, untrusted-proxy, 10.0.0.1"
// - 10.0.0.1 is our load balancer (trusted)
// - untrusted-proxy is the rightmost untrusted IP (return this)
// - real-client and spoofed-ip were added by untrusted sources
func getClientIP(c echo.Context) string {
	remoteIP := parseIP(c.Request().RemoteAddr)

	// If RemoteAddr is not from a trusted proxy, don't trust X-Forwarded-For
	if !isTrustedProxy(remoteIP) {
		return remoteIP
	}

	// RemoteAddr is trusted, check X-Forwarded-For
	xff := c.Request().Header.Get("X-Forwarded-For")
	if xff == "" {
		return remoteIP
	}

	// Parse X-Forwarded-For chain
	ips := strings.Split(xff, ",")
	if len(ips) == 0 {
		return remoteIP
	}

	// Walk from right to left to find the rightmost untrusted IP
	for i := len(ips) - 1; i >= 0; i-- {
		ipStr := strings.TrimSpace(ips[i])

		// Skip empty or invalid IPs
		if ipStr == "" {
			continue
		}

		// Parse and validate the IP
		parsedIP := parseIP(ipStr)
		if parsedIP == "" {
			continue // Skip invalid IPs
		}

		// If this IP is not trusted, it's the rightmost untrusted IP
		if !isTrustedProxy(parsedIP) {
			return parsedIP
		}
	}

	// All IPs in the chain are trusted (internal traffic)
	// Use the leftmost IP as the client
	for i := 0; i < len(ips); i++ {
		ipStr := strings.TrimSpace(ips[i])
		parsedIP := parseIP(ipStr)
		if parsedIP != "" {
			return parsedIP
		}
	}

	// Fallback to RemoteAddr if we couldn't parse anything
	return remoteIP
}
