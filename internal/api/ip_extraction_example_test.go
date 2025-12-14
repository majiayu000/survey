package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
)

// Example demonstrates the secure IP extraction behavior
func Example_getClientIP_attackPrevention() {
	e := echo.New()

	// Scenario 1: Attacker tries to spoof IP from public internet
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "203.0.113.50:12345" // Real attacker IP (public)
	req1.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8") // Spoofed IPs
	c1 := e.NewContext(req1, httptest.NewRecorder())
	fmt.Printf("Attack blocked: %s\n", getClientIP(c1))

	// Scenario 2: Legitimate request through trusted load balancer
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "10.0.0.1:12345" // Our load balancer (trusted)
	req2.Header.Set("X-Forwarded-For", "203.0.113.100") // Real client IP
	c2 := e.NewContext(req2, httptest.NewRecorder())
	fmt.Printf("Legitimate request: %s\n", getClientIP(c2))

	// Scenario 3: Direct connection (no proxy)
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.RemoteAddr = "198.51.100.25:12345" // Direct client
	c3 := e.NewContext(req3, httptest.NewRecorder())
	fmt.Printf("Direct connection: %s\n", getClientIP(c3))

	// Output:
	// Attack blocked: 203.0.113.50
	// Legitimate request: 203.0.113.100
	// Direct connection: 198.51.100.25
}

// Example demonstrates rightmost untrusted IP selection
func Example_getClientIP_rightmostUntrusted() {
	e := echo.New()

	// X-Forwarded-For: client -> external-proxy -> our-load-balancer
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345" // Our load balancer (trusted)
	req.Header.Set("X-Forwarded-For", "203.0.113.100, 198.51.100.50")
	//                                  ^              ^
	//                                  Client         External proxy (rightmost untrusted)
	c := e.NewContext(req, httptest.NewRecorder())
	fmt.Printf("Rightmost untrusted IP: %s\n", getClientIP(c))

	// Output:
	// Rightmost untrusted IP: 198.51.100.50
}
