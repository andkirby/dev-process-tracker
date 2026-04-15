package lifecycle

import (
	"fmt"
	"testing"

	"github.com/devports/devpt/pkg/models"
)

func TestStop_VerifiedRunning(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{Name: "api", CWD: "/project"}
	proc := &models.ProcessRecord{PID: 1234, CWD: "/project", Port: 3000}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{proc}
	deps.runningPIDs[1234] = true

	result := StopService(deps, svc)
	if result.Outcome != OutcomeSuccess {
		t.Errorf("verified running should return success, got %q: %s", result.Outcome, result.Message)
	}
	if result.PID != 1234 {
		t.Errorf("success should include stopped PID, got %d", result.PID)
	}
}

func TestStop_AlreadyStopped(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{Name: "api", CWD: "/project"}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{}

	result := StopService(deps, svc)
	if result.Outcome != OutcomeNoop {
		t.Errorf("already stopped should return noop, got %q", result.Outcome)
	}
}

func TestStop_AmbiguousIdentity(t *testing.T) {
	t.Parallel()

	svc1 := &models.ManagedService{Name: "api", CWD: "/shared"}
	svc2 := &models.ManagedService{Name: "worker", CWD: "/shared"}
	proc := &models.ProcessRecord{PID: 1234, CWD: "/shared", Port: 3000}

	deps := newMockDeps()
	deps.services["api"] = svc1
	deps.services["worker"] = svc2
	deps.processes = []*models.ProcessRecord{proc}
	deps.runningPIDs[1234] = true

	result := StopService(deps, svc1)
	if result.Outcome != OutcomeBlocked {
		t.Errorf("ambiguous identity should return blocked, got %q", result.Outcome)
	}
}

func TestStop_StaleMetadata(t *testing.T) {
	t.Parallel()

	pid := 9999
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     "/project",
		LastPID: &pid,
	}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{}

	result := StopService(deps, svc)
	if result.Outcome != OutcomeNoop {
		t.Errorf("stale metadata should return noop, got %q", result.Outcome)
	}
}

func TestStop_SigkillFailure(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{Name: "api", CWD: "/project"}
	proc := &models.ProcessRecord{PID: 1234, CWD: "/project", Port: 3000}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{proc}
	deps.runningPIDs[1234] = true
	deps.stopErr = fmt.Errorf("process still alive")

	result := StopService(deps, svc)
	if result.Outcome == OutcomeSuccess {
		t.Error("SIGKILL failure should not return success")
	}
}

func TestStop_LockContention(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{Name: "api", CWD: "/project"}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{}
	deps.locked["api"] = true

	result := StopService(deps, svc)
	if result.Outcome != OutcomeBlocked {
		t.Errorf("lock contention should return blocked, got %q", result.Outcome)
	}
}

func TestStop_NilDeps(t *testing.T) {
	t.Parallel()

	result := StopService(nil, &models.ManagedService{Name: "api"})
	if result.Outcome != OutcomeInvalid {
		t.Errorf("nil deps should return invalid, got %q", result.Outcome)
	}
}

func TestStop_NilService(t *testing.T) {
	t.Parallel()

	deps := newMockDeps()
	result := StopService(deps, nil)
	if result.Outcome != OutcomeInvalid {
		t.Errorf("nil service should return invalid, got %q", result.Outcome)
	}
}

func TestStop_MetadataClearedOnSuccess(t *testing.T) {
	t.Parallel()

	pid := 1234
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     "/project",
		LastPID: &pid,
	}
	proc := &models.ProcessRecord{PID: 1234, CWD: "/project", Port: 3000}

	deps := newMockDeps()
	deps.services["api"] = svc
	deps.processes = []*models.ProcessRecord{proc}
	deps.runningPIDs[1234] = true

	result := StopService(deps, svc)
	if result.Outcome == OutcomeSuccess {
		// Verify PID was cleared
		if svc.LastPID != nil {
			t.Error("LastPID should be cleared after successful stop")
		}
	}
}
