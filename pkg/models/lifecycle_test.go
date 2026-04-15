package models

import (
	"testing"
	"time"
)

func TestLifecycleStatusConstants(t *testing.T) {
	t.Parallel()

	if StatusRunning == "" {
		t.Error("StatusRunning should not be empty")
	}
	if StatusStopped == "" {
		t.Error("StatusStopped should not be empty")
	}
	if StatusCrashed == "" {
		t.Error("StatusCrashed should not be empty")
	}
	if StatusUnknown == "" {
		t.Error("StatusUnknown should not be empty")
	}
}

func TestReadinessModeConstants(t *testing.T) {
	t.Parallel()

	if ReadinessProcessOnly == "" {
		t.Error("ReadinessProcessOnly should not be empty")
	}
	if ReadinessPortBound == "" {
		t.Error("ReadinessPortBound should not be empty")
	}
	if ReadinessHTTPHealth == "" {
		t.Error("ReadinessHTTPHealth should not be empty")
	}
	if ReadinessLogSignal == "" {
		t.Error("ReadinessLogSignal should not be empty")
	}
	if ReadinessMultiCheck == "" {
		t.Error("ReadinessMultiCheck should not be empty")
	}
}

func TestReadinessConfigZeroValues(t *testing.T) {
	t.Parallel()

	var cfg ReadinessConfig
	if cfg.Mode != "" {
		t.Errorf("zero-value Mode = %q, want empty", cfg.Mode)
	}
	if cfg.Timeout != 0 {
		t.Errorf("zero-value Timeout = %v, want 0", cfg.Timeout)
	}
	if cfg.Endpoint != "" {
		t.Errorf("zero-value Endpoint = %q, want empty", cfg.Endpoint)
	}
	if cfg.LogPattern != "" {
		t.Errorf("zero-value LogPattern = %q, want empty", cfg.LogPattern)
	}
}

func TestManagedServiceReadinessBackwardCompat(t *testing.T) {
	t.Parallel()

	svc := &ManagedService{
		Name:    "test",
		CWD:     "/tmp",
		Command: "echo hi",
		CreatedAt: time.Time{},
		UpdatedAt: time.Time{},
	}
	if svc.Readiness != nil {
		t.Error("new ManagedService should have nil Readiness by default")
	}
}

func TestManagedServiceWithReadinessConfig(t *testing.T) {
	t.Parallel()

	svc := &ManagedService{
		Name:    "api",
		CWD:     "/app",
		Command: "npm start",
		Ports:   []int{3000},
		Readiness: &ReadinessConfig{
			Mode:     ReadinessHTTPHealth,
			Timeout:  5,
			Endpoint: "http://localhost:3000/health",
		},
	}
	if svc.Readiness == nil {
		t.Fatal("Readiness should not be nil")
	}
	if svc.Readiness.Mode != ReadinessHTTPHealth {
		t.Errorf("Mode = %q, want %q", svc.Readiness.Mode, ReadinessHTTPHealth)
	}
	if svc.Readiness.Timeout != 5 {
		t.Errorf("Timeout = %v, want 5", svc.Readiness.Timeout)
	}
}
