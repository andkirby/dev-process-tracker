package lifecycle

import (
	"fmt"
	"testing"
	"time"

	"github.com/devports/devpt/pkg/models"
)

// mockProcessChecker implements ProcessChecker for testing.
type mockProcessChecker struct {
	alive bool
}

func (m *mockProcessChecker) IsRunning(pid int) bool {
	return m.alive
}

// mockHealthChecker implements HealthChecker for testing.
type mockHealthChecker struct {
	healthy bool
}

func (m *mockHealthChecker) Check(port int) bool {
	return m.healthy
}

func TestWaitForReadiness_ProcessOnly(t *testing.T) {
	t.Parallel()

	policy := &ReadinessPolicy{
		Mode:    models.ReadinessProcessOnly,
		Timeout: 2 * time.Second,
	}

	err := policy.Wait(1234, nil, &mockProcessChecker{alive: true}, nil, nil)
	if err != nil {
		t.Errorf("WaitForReadiness(process-only) should succeed for alive process, got error: %v", err)
	}
}

func TestWaitForReadiness_PortBound(t *testing.T) {
	t.Parallel()

	policy := &ReadinessPolicy{
		Mode:     models.ReadinessPortBound,
		Timeout:  2 * time.Second,
		Endpoint: "localhost:19999", // unlikely to be listening
	}

	err := policy.Wait(1234, nil, &mockProcessChecker{alive: true}, nil, nil)
	// Port 19999 is unlikely to be bound, so this should timeout
	if err == nil {
		t.Log("Port-bound succeeded (port was actually bound)")
	} else {
		if err != ErrReadinessTimeout {
			t.Errorf("expected ErrReadinessTimeout, got %v", err)
		}
	}
}

func TestWaitForReadiness_HTTPHealth(t *testing.T) {
	t.Parallel()

	policy := &ReadinessPolicy{
		Mode:     models.ReadinessHTTPHealth,
		Timeout:  2 * time.Second,
		Endpoint: "http://localhost:19999/health",
	}

	err := policy.Wait(1234, nil, &mockProcessChecker{alive: true}, &mockHealthChecker{healthy: false}, nil)
	// No server running, should timeout
	if err == nil {
		t.Log("HTTP health check succeeded (server was running)")
	} else if err != ErrReadinessTimeout {
		t.Errorf("expected ErrReadinessTimeout, got %v", err)
	}
}

func TestWaitForReadiness_LogSignal(t *testing.T) {
	t.Parallel()

	policy := &ReadinessPolicy{
		Mode:       models.ReadinessLogSignal,
		Timeout:    2 * time.Second,
		LogPattern: "Server started",
	}

	logs := func() []string {
		return []string{"listening on port 3000", "Server started on port 3000"}
	}

	err := policy.Wait(1234, nil, &mockProcessChecker{alive: true}, nil, logs)
	if err != nil {
		t.Errorf("WaitForReadiness(log-signal) should succeed when pattern found in logs, got error: %v", err)
	}
}

func TestWaitForReadiness_MultiCheck(t *testing.T) {
	t.Parallel()

	policy := &ReadinessPolicy{
		Mode:       models.ReadinessMultiCheck,
		Timeout:    2 * time.Second,
		LogPattern: "ready",
	}

	err := policy.Wait(1234, nil, &mockProcessChecker{alive: true}, nil, func() []string {
		return []string{"ready"}
	})
	if err != nil {
		t.Errorf("WaitForReadiness(multi-check) should succeed when all checks pass, got error: %v", err)
	}
}

func TestWaitForReadiness_Timeout(t *testing.T) {
	t.Parallel()

	policy := &ReadinessPolicy{
		Mode:    models.ReadinessProcessOnly,
		Timeout: 200 * time.Millisecond,
	}

	err := policy.Wait(1234, nil, &mockProcessChecker{alive: false}, nil, nil)
	if err == nil {
		t.Error("WaitForReadiness should return error when process is dead and timeout exceeded")
	}
	if err != ErrReadinessTimeout {
		t.Errorf("expected ErrReadinessTimeout, got %v", err)
	}
}

func TestFallbackPolicy_NilWithPorts(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{
		Name:  "api",
		Ports: []int{3000},
	}

	policy := SelectReadinessPolicy(svc.Readiness, svc.Ports)
	if policy.Mode != models.ReadinessPortBound {
		t.Errorf("fallback for service with ports should be port-bound, got %q", policy.Mode)
	}
}

func TestFallbackPolicy_NilWithoutPorts(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{
		Name:  "worker",
		Ports: []int{},
	}

	policy := SelectReadinessPolicy(svc.Readiness, svc.Ports)
	if policy.Mode != models.ReadinessProcessOnly {
		t.Errorf("fallback for service without ports should be process-only, got %q", policy.Mode)
	}
}

func TestExplicitReadinessPolicy(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{
		Name: "api",
		Readiness: &models.ReadinessConfig{
			Mode:     models.ReadinessHTTPHealth,
			Timeout:  5,
			Endpoint: "http://localhost:3000/health",
		},
	}

	policy := SelectReadinessPolicy(svc.Readiness, svc.Ports)
	if policy.Mode != models.ReadinessHTTPHealth {
		t.Errorf("explicit policy should override fallback, got %q", policy.Mode)
	}
}

func TestWait_PortBound(t *testing.T) {
	t.Parallel()

	policy := &ReadinessPolicy{
		Mode:    models.ReadinessPortBound,
		Timeout: 200 * time.Millisecond,
	}

	err := policy.Wait(1234, []int{19998, 19999}, &mockProcessChecker{alive: true}, nil, nil)
	// Ports unlikely to be bound
	if err == nil {
		t.Log("Port-bound with ports succeeded (port was actually bound)")
	} else if err != ErrReadinessTimeout {
		t.Errorf("expected ErrReadinessTimeout, got %v", err)
	}
}

func TestParsePortFromEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected int
	}{
		{"localhost:3000", 3000},
		{":8080", 8080},
		{"", 0},
		{"invalid", 0},
		{"http://localhost:3000/health", 3000},
	}

	for _, tt := range tests {
		got := parsePortFromEndpoint(tt.input)
		if got != tt.expected {
			t.Errorf("parsePortFromEndpoint(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestContainsPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line    string
		pattern string
		want    bool
	}{
		{"Server started on port 3000", "Server started", true},
		{"listening on :3000", "ready", false},
		{"", "anything", false},
		{"ready", "", false},
	}

	for _, tt := range tests {
		got := containsPattern(tt.line, tt.pattern)
		if got != tt.want {
			t.Errorf("containsPattern(%q, %q) = %v, want %v", tt.line, tt.pattern, got, tt.want)
		}
	}
}

func TestSelectReadinessPolicy_CustomTimeout(t *testing.T) {
	t.Parallel()

	cfg := &models.ReadinessConfig{
		Mode:    models.ReadinessPortBound,
		Timeout: 10,
	}

	policy := SelectReadinessPolicy(cfg, []int{3000})
	if policy.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", policy.Timeout)
	}
}

func TestWaitForReadiness_ProcessOnlyDead(t *testing.T) {
	t.Parallel()

	policy := &ReadinessPolicy{
		Mode:    models.ReadinessProcessOnly,
		Timeout: 500 * time.Millisecond,
	}

	err := policy.Wait(1234, nil, &mockProcessChecker{alive: false}, nil, nil)
	if err == nil {
		t.Error("should timeout when process is dead")
	}
}

func TestWait_LogSignalNoMatch(t *testing.T) {
	t.Parallel()

	policy := &ReadinessPolicy{
		Mode:       models.ReadinessLogSignal,
		Timeout:    500 * time.Millisecond,
		LogPattern: "NEVER_MATCH_THIS",
	}

	logs := func() []string {
		return []string{"listening on port 3000"}
	}

	err := policy.Wait(1234, nil, &mockProcessChecker{alive: true}, nil, logs)
	if err == nil {
		t.Error("should timeout when log pattern is never found")
	}
}

func TestWait_MultiCheckPartialFail(t *testing.T) {
	t.Parallel()

	policy := &ReadinessPolicy{
		Mode:       models.ReadinessMultiCheck,
		Timeout:    500 * time.Millisecond,
		LogPattern: "NEVER_MATCH",
	}

	err := policy.Wait(1234, nil, &mockProcessChecker{alive: true}, nil, func() []string {
		return []string{"other stuff"}
	})
	if err == nil {
		t.Error("multi-check should fail when one check fails")
	}
}

func TestWait_MultiCheckAllPass(t *testing.T) {
	t.Parallel()

	policy := &ReadinessPolicy{
		Mode:       models.ReadinessMultiCheck,
		Timeout:    2 * time.Second,
		LogPattern: "ready",
	}

	err := policy.Wait(1234, nil, &mockProcessChecker{alive: true}, nil, func() []string {
		return []string{"ready"}
	})
	if err != nil {
		t.Errorf("multi-check should pass when all checks succeed, got: %v", err)
	}
}

func TestSelectReadinessPolicy_DefaultTimeout(t *testing.T) {
	t.Parallel()

	policy := SelectReadinessPolicy(nil, []int{3000})
	if policy.Timeout != 5*time.Second {
		t.Errorf("default port-bound timeout should be 5s, got %v", policy.Timeout)
	}

	policy2 := SelectReadinessPolicy(nil, nil)
	if policy2.Timeout != 3*time.Second {
		t.Errorf("default process-only timeout should be 3s, got %v", policy2.Timeout)
	}
}

func TestErrReadinessTimeout(t *testing.T) {
	t.Parallel()

	if ErrReadinessTimeout == nil {
		t.Error("ErrReadinessTimeout should not be nil")
	}
	_ = fmt.Sprintf("timeout error: %v", ErrReadinessTimeout)
}
