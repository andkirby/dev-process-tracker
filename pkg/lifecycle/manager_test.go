package lifecycle

import (
	"testing"

	"github.com/devports/devpt/pkg/models"
)

func TestLifecycleManager_HoldsDependencies(t *testing.T) {
	t.Parallel()

	deps := newMockDeps()
	mgr := NewLifecycleManager(deps)
	if mgr == nil {
		t.Error("LifecycleManager should be creatable")
	}
}

func TestLifecycleManager_StartDelegates(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     tmpDir,
		Command: "echo hi",
		Readiness: &models.ReadinessConfig{
			Mode:    models.ReadinessProcessOnly,
			Timeout: 1,
		},
	}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{}

	mgr := NewLifecycleManager(deps)
	result := mgr.Start(svc)
	if result.Outcome != OutcomeSuccess {
		t.Errorf("Manager.Start should succeed, got %q: %s", result.Outcome, result.Message)
	}
}

func TestLifecycleManager_StopDelegates(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{Name: "api", CWD: "/project"}
	proc := &models.ProcessRecord{PID: 1234, CWD: "/project", Port: 3000}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{proc}
	deps.runningPIDs[1234] = true

	mgr := NewLifecycleManager(deps)
	result := mgr.Stop(svc)
	if result.Outcome != OutcomeSuccess {
		t.Errorf("Manager.Stop should succeed for running service, got %q: %s", result.Outcome, result.Message)
	}
}

func TestLifecycleManager_RestartDelegates(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     tmpDir,
		Command: "echo hi",
		Readiness: &models.ReadinessConfig{
			Mode:    models.ReadinessProcessOnly,
			Timeout: 1,
		},
	}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{}

	mgr := NewLifecycleManager(deps)
	result := mgr.Restart(svc)
	if result.Outcome != OutcomeSuccess {
		t.Errorf("Manager.Restart should succeed, got %q: %s", result.Outcome, result.Message)
	}
}

func TestLifecycleManager_NilDeps(t *testing.T) {
	t.Parallel()

	mgr := NewLifecycleManager(nil)
	svc := &models.ManagedService{Name: "api", CWD: "/project", Command: "echo hi"}

	startResult := mgr.Start(svc)
	if startResult.Outcome != OutcomeInvalid {
		t.Errorf("Manager.Start with nil deps should return invalid, got %q", startResult.Outcome)
	}

	stopResult := mgr.Stop(svc)
	if stopResult.Outcome != OutcomeInvalid {
		t.Errorf("Manager.Stop with nil deps should return invalid, got %q", stopResult.Outcome)
	}

	restartResult := mgr.Restart(svc)
	if restartResult.Outcome != OutcomeInvalid {
		t.Errorf("Manager.Restart with nil deps should return invalid, got %q", restartResult.Outcome)
	}
}

func TestLifecycleManager_ConcurrentLockBlocked(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     tmpDir,
		Command: "echo hi",
		Readiness: &models.ReadinessConfig{
			Mode:    models.ReadinessProcessOnly,
			Timeout: 1,
		},
	}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{}
	deps.locked["api"] = true

	mgr := NewLifecycleManager(deps)

	result := mgr.Start(svc)
	if result.Outcome != OutcomeBlocked {
		t.Errorf("concurrent lock should block start, got %q", result.Outcome)
	}

	result = mgr.Stop(svc)
	if result.Outcome != OutcomeBlocked {
		t.Errorf("concurrent lock should block stop, got %q", result.Outcome)
	}

	result = mgr.Restart(svc)
	if result.Outcome != OutcomeBlocked {
		t.Errorf("concurrent lock should block restart, got %q", result.Outcome)
	}
}
