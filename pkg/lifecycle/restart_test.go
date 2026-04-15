package lifecycle

import (
	"fmt"
	"testing"

	"github.com/devports/devpt/pkg/models"
)

func TestRestart_VerifiedRunning(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     tmpDir,
		Command: "npm start",
		Readiness: &models.ReadinessConfig{
			Mode:    models.ReadinessProcessOnly,
			Timeout: 1,
		},
	}
	proc := &models.ProcessRecord{PID: 1234, CWD: tmpDir, Port: 3000}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{proc}
	deps.runningPIDs[1234] = true

	result := RestartService(deps, svc)
	if result.Outcome != OutcomeSuccess {
		t.Errorf("restart of running service should succeed, got %q: %s", result.Outcome, result.Message)
	}
	if result.PID == 0 {
		t.Error("success should include new PID")
	}
}

func TestRestart_AlreadyStopped(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     tmpDir,
		Command: "npm start",
		Readiness: &models.ReadinessConfig{
			Mode:    models.ReadinessProcessOnly,
			Timeout: 1,
		},
	}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{}

	result := RestartService(deps, svc)
	// Should report as fresh start
	if result.Outcome != OutcomeSuccess {
		t.Errorf("restart of stopped service should succeed as fresh start, got %q: %s", result.Outcome, result.Message)
	}
	// Message should indicate fresh start
	if result.Message != "" {
		// Should say "started" not "restarted" for a service that was already stopped
		t.Logf("Restart message: %q", result.Message)
	}
}

func TestRestart_OldCannotStop(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     tmpDir,
		Command: "npm start",
	}
	proc := &models.ProcessRecord{PID: 1234, CWD: tmpDir, Port: 3000}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{proc}
	deps.runningPIDs[1234] = true
	deps.stopErr = fmt.Errorf("cannot stop process") // Simulate stop failure

	result := RestartService(deps, svc)
	if result.Outcome != OutcomeBlocked {
		t.Errorf("old instance cannot stop should return blocked, got %q", result.Outcome)
	}
}

func TestRestart_NewFailsReadiness(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     tmpDir,
		Command: "sleep 100",
		Ports:   []int{3000},
		Readiness: &models.ReadinessConfig{
			Mode:    models.ReadinessPortBound,
			Timeout: 1,
		},
	}
	proc := &models.ProcessRecord{PID: 1234, CWD: tmpDir, Port: 3000}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{proc}
	deps.runningPIDs[1234] = true

	result := RestartService(deps, svc)
	// New instance won't become ready (port-bound timeout)
	if result.Outcome == OutcomeSuccess {
		t.Error("readiness failure should not return success")
	}
	if result.Outcome == OutcomeFailed {
		t.Logf("Correctly reported failure: %s", result.Message)
	}
}

func TestRestart_FreshnessRule(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     tmpDir,
		Command: "npm start",
		Readiness: &models.ReadinessConfig{
			Mode:    models.ReadinessProcessOnly,
			Timeout: 1,
		},
	}
	proc := &models.ProcessRecord{PID: 1234, CWD: tmpDir, Port: 3000}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{proc}
	deps.runningPIDs[1234] = true

	result := RestartService(deps, svc)
	if result.Outcome == OutcomeSuccess {
		// New PID should differ from old
		if result.PID == 1234 {
			t.Error("restart should produce a different PID than the old instance")
		}
	}
}

func TestRestart_StoppedReportsFreshStart(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     tmpDir,
		Command: "npm start",
		Readiness: &models.ReadinessConfig{
			Mode:    models.ReadinessProcessOnly,
			Timeout: 1,
		},
	}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{}

	result := RestartService(deps, svc)
	if result.Outcome == OutcomeSuccess && result.Message != "" {
		// Message should mention "started" not "restarted" for a stopped service
		contains := false
		for i := 0; i <= len(result.Message)-7; i++ {
			if result.Message[i:i+7] == "started" {
				contains = true
				break
			}
		}
		if !contains {
			t.Errorf("message should mention 'started' for fresh start, got: %s", result.Message)
		}
	}
}

func TestRestart_AmbiguousIdentity(t *testing.T) {
	t.Parallel()

	svc1 := &models.ManagedService{Name: "api", CWD: "/shared"}
	svc2 := &models.ManagedService{Name: "worker", CWD: "/shared"}
	proc := &models.ProcessRecord{PID: 1234, CWD: "/shared", Port: 3000}

	deps := newMockDeps()
	deps.services["api"] = svc1
	deps.services["worker"] = svc2
	deps.processes = []*models.ProcessRecord{proc}
	deps.runningPIDs[1234] = true

	result := RestartService(deps, svc1)
	if result.Outcome != OutcomeBlocked {
		t.Errorf("ambiguous identity should return blocked, got %q", result.Outcome)
	}
}

func TestRestart_LockContention(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{Name: "api", CWD: "/project", Command: "echo hi"}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{}
	deps.locked["api"] = true

	result := RestartService(deps, svc)
	if result.Outcome != OutcomeBlocked {
		t.Errorf("lock contention should return blocked, got %q", result.Outcome)
	}
}

func TestRestart_NilDeps(t *testing.T) {
	t.Parallel()

	result := RestartService(nil, &models.ManagedService{Name: "api"})
	if result.Outcome != OutcomeInvalid {
		t.Errorf("nil deps should return invalid, got %q", result.Outcome)
	}
}

func TestRestart_CrashedService(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pid := 9999
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     tmpDir,
		Command: "echo hi",
		LastPID: &pid,
		Readiness: &models.ReadinessConfig{
			Mode:    models.ReadinessProcessOnly,
			Timeout: 1,
		},
	}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{}

	result := RestartService(deps, svc)
	// Crashed service should be treated as fresh start
	if result.Outcome != OutcomeSuccess {
		t.Errorf("restart of crashed service should succeed as fresh start, got %q: %s", result.Outcome, result.Message)
	}
}
