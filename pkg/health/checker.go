package health

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

// Health status levels
type HealthStatus string

const (
	HealthOK      HealthStatus = "ok"
	HealthSlow    HealthStatus = "slow"
	HealthTimeout HealthStatus = "timeout"
	HealthDown    HealthStatus = "down"
	HealthUnknown HealthStatus = "unknown"
)

// HealthCheck represents the result of a health check
type HealthCheck struct {
	Port       int
	Status     HealthStatus
	ResponseMs int
	Message    string
	LastCheck  time.Time
}

// Checker performs health checks on services
type Checker struct {
	timeout time.Duration
}

// NewChecker creates a new health checker
func NewChecker(timeout time.Duration) *Checker {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &Checker{timeout: timeout}
}

// Check performs a health check on a port
func (c *Checker) Check(port int) *HealthCheck {
	result := &HealthCheck{
		Port:      port,
		LastCheck: time.Now(),
	}

	// Try HTTP first
	if ok, ms := c.checkHTTP(port); ok {
		result.Status = categorizeResponse(ms)
		result.ResponseMs = ms
		result.Message = fmt.Sprintf("HTTP responding in %dms", ms)
		return result
	}

	// Fall back to TCP
	if ok, ms := c.checkTCP(port); ok {
		result.Status = categorizeResponse(ms)
		result.ResponseMs = ms
		result.Message = fmt.Sprintf("TCP responding in %dms", ms)
		return result
	}

	// Port is listening but not responding
	result.Status = HealthDown
	result.Message = "Port listening but no response"
	return result
}

// checkHTTP attempts an HTTP connection
func (c *Checker) checkHTTP(port int) (bool, int) {
	url := fmt.Sprintf("http://localhost:%d", port)
	client := &http.Client{
		Timeout: c.timeout,
	}

	start := time.Now()
	resp, err := client.Get(url)
	elapsed := int(time.Since(start).Milliseconds())

	if err != nil {
		return false, 0
	}
	defer resp.Body.Close()

	return true, elapsed
}

// checkTCP attempts a TCP connection
func (c *Checker) checkTCP(port int) (bool, int) {
	addr := fmt.Sprintf("localhost:%d", port)

	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, c.timeout)
	elapsed := int(time.Since(start).Milliseconds())

	if err != nil {
		return false, 0
	}
	defer conn.Close()

	return true, elapsed
}

// categorizeResponse categorizes response time into status
func categorizeResponse(ms int) HealthStatus {
	if ms > 2000 {
		return HealthSlow
	}
	if ms > 5000 {
		return HealthTimeout
	}
	return HealthOK
}

// StatusIcon returns an emoji for the health status
func StatusIcon(status HealthStatus) string {
	switch status {
	case HealthOK:
		return "✅"
	case HealthSlow:
		return "⚠️"
	case HealthTimeout:
		return "🐢"
	case HealthDown:
		return "❌"
	default:
		return "❓"
	}
}
