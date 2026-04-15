package lifecycle

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/devports/devpt/pkg/models"
)

// ErrReadinessTimeout is returned when a service does not become ready within the timeout.
var ErrReadinessTimeout = fmt.Errorf("service did not become ready within the timeout")

// ProcessChecker checks if a process is alive.
type ProcessChecker interface {
	IsRunning(pid int) bool
}

// HealthChecker checks health endpoints.
type HealthChecker interface {
	Check(port int) bool
}

// ReadinessPolicy defines how to wait for a service to become ready.
type ReadinessPolicy struct {
	Mode       models.ReadinessMode
	Timeout    time.Duration
	Endpoint   string
	LogPattern string
}

// Wait blocks until the service is ready or the timeout expires.
// Ports are used for port-bound, http-health, and multi-check modes.
// The processChk parameter checks process liveness (may be nil).
// The healthChk parameter checks HTTP health (may be nil).
// The logsTail parameter returns recent log lines (may be nil).
func (p *ReadinessPolicy) Wait(
	pid int,
	ports []int,
	processChk ProcessChecker,
	healthChk HealthChecker,
	logsTail func() []string,
) error {
	if p.Timeout <= 0 {
		p.Timeout = 5 * time.Second
	}

	deadline := time.Now().Add(p.Timeout)
	interval := 100 * time.Millisecond

	for time.Now().Before(deadline) {
		switch p.Mode {
		case models.ReadinessProcessOnly:
			if processChk != nil && processChk.IsRunning(pid) {
				return nil
			}

		case models.ReadinessPortBound:
			for _, port := range ports {
				if port > 0 && checkTCPPort(fmt.Sprintf("127.0.0.1:%d", port)) {
					return nil
				}
			}

		case models.ReadinessHTTPHealth:
			if healthChk != nil {
				for _, port := range ports {
					if port > 0 && healthChk.Check(port) {
						return nil
					}
				}
			}

		case models.ReadinessLogSignal:
			if logsTail != nil && p.LogPattern != "" {
				lines := logsTail()
				for _, line := range lines {
					if containsPattern(line, p.LogPattern) {
						return nil
					}
				}
			}

		case models.ReadinessMultiCheck:
			allPass := true
			if processChk != nil && !processChk.IsRunning(pid) {
				allPass = false
			}
			if len(ports) > 0 {
				portBound := false
				for _, port := range ports {
					if port > 0 && checkTCPPort(fmt.Sprintf("localhost:%d", port)) {
						portBound = true
						break
					}
				}
				if !portBound {
					allPass = false
				}
			}
			if logsTail != nil && p.LogPattern != "" {
				found := false
				lines := logsTail()
				for _, line := range lines {
					if containsPattern(line, p.LogPattern) {
						found = true
						break
					}
				}
				if !found {
					allPass = false
				}
			}
			if allPass {
				return nil
			}
		}

		time.Sleep(interval)
	}

	return ErrReadinessTimeout
}

// SelectReadinessPolicy returns the appropriate readiness policy.
// If the service has an explicit config, use it.
// Otherwise, fall back to port-bound for services with ports, process-only for those without.
func SelectReadinessPolicy(cfg *models.ReadinessConfig, ports []int) ReadinessPolicy {
	if cfg != nil && cfg.Mode != "" {
		return ReadinessPolicy{
			Mode:       cfg.Mode,
			Timeout:    time.Duration(cfg.Timeout) * time.Second,
			Endpoint:   cfg.Endpoint,
			LogPattern: cfg.LogPattern,
		}
	}

	if len(ports) > 0 {
		return ReadinessPolicy{
			Mode:    models.ReadinessPortBound,
			Timeout: 5 * time.Second,
		}
	}

	return ReadinessPolicy{
		Mode:    models.ReadinessProcessOnly,
		Timeout: 3 * time.Second,
	}
}

func checkTCPPort(addr string) bool {
	// If addr is "localhost:port", also try "127.0.0.1:port"
	// to handle macOS where localhost may resolve to IPv6 first.
	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		// Try 127.0.0.1 as fallback
		for i := len(addr) - 1; i >= 0; i-- {
			if addr[i] == ':' {
				fallback := "127.0.0.1" + addr[i:]
				conn, err = net.DialTimeout("tcp", fallback, 200*time.Millisecond)
				break
			}
		}
	}
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func parsePortFromEndpoint(endpoint string) int {
	if endpoint == "" {
		return 0
	}
	// Find the last colon that precedes a port number
	// Handle "localhost:3000", ":3000", "http://localhost:3000/health"
	lastColon := -1
	for i := len(endpoint) - 1; i >= 0; i-- {
		if endpoint[i] == ':' {
			lastColon = i
			break
		}
	}
	if lastColon < 0 {
		return 0
	}
	portStr := endpoint[lastColon+1:]
	// Trim any path suffix
	for i, c := range portStr {
		if c == '/' {
			portStr = portStr[:i]
			break
		}
	}
	port := 0
	for _, c := range portStr {
		if c < '0' || c > '9' {
			return 0
		}
		port = port*10 + int(c-'0')
	}
	return port
}

func containsPattern(line, pattern string) bool {
	return pattern != "" && strings.Contains(line, pattern)
}
