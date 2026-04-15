package lifecycle

import (
	"testing"

	"github.com/devports/devpt/pkg/models"
)

func TestReconcile_VerifiedRunning_CWD(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{
		Name: "api",
		CWD:  "/project/app",
	}
	proc := &models.ProcessRecord{
		PID:  1234,
		CWD:  "/project/app",
		Port: 3000,
	}

	result := Reconcile(svc, []*models.ProcessRecord{proc}, []*models.ManagedService{svc})
	if result.Status != "running" {
		t.Errorf("expected status running for CWD match, got %q", result.Status)
	}
}

func TestReconcile_VerifiedRunning_ProjectRoot(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{
		Name: "api",
		CWD:  "/project/app/src",
	}
	proc := &models.ProcessRecord{
		PID:         1234,
		CWD:         "/project/app/src/server",
		ProjectRoot: "/project/app",
		Port:        3000,
	}

	resolver := func(cwd string) string {
		if cwd == "/project/app/src" {
			return "/project/app"
		}
		return cwd
	}

	result := ReconcileWithResolver(svc, []*models.ProcessRecord{proc}, []*models.ManagedService{svc}, resolver)
	if result.Status != "running" {
		t.Errorf("expected status running for project root match, got %q", result.Status)
	}
}

func TestReconcile_VerifiedRunning_UniquePort(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{
		Name:  "api",
		CWD:   "/project/app",
		Ports: []int{3000},
	}
	// Process has no CWD info (common with lsof), but is on the service's unique port
	proc := &models.ProcessRecord{
		PID:  1234,
		CWD:  "",
		Port: 3000,
	}

	result := Reconcile(svc, []*models.ProcessRecord{proc}, []*models.ManagedService{svc})
	if result.Status != "running" {
		t.Errorf("expected status running for unique port match, got %q", result.Status)
	}
}

func TestReconcile_Stopped(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{
		Name: "api",
		CWD:  "/project/app",
	}

	result := Reconcile(svc, []*models.ProcessRecord{}, []*models.ManagedService{svc})
	if result.Status != "stopped" {
		t.Errorf("expected status stopped, got %q", result.Status)
	}
}

func TestReconcile_Crashed_StalePID(t *testing.T) {
	t.Parallel()

	pid := 9999 // Not running
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     "/project/app",
		LastPID: &pid,
	}

	result := Reconcile(svc, []*models.ProcessRecord{}, []*models.ManagedService{svc})
	if result.Status != "crashed" {
		t.Errorf("expected status crashed for stale PID with no live process, got %q", result.Status)
	}
}

func TestReconcile_Unknown_AmbiguousIdentity(t *testing.T) {
	t.Parallel()

	svc1 := &models.ManagedService{
		Name: "api",
		CWD:  "/shared",
	}
	svc2 := &models.ManagedService{
		Name: "worker",
		CWD:  "/shared",
	}
	proc := &models.ProcessRecord{
		PID:  1234,
		CWD:  "/shared",
		Port: 3000,
	}

	result := Reconcile(svc1, []*models.ProcessRecord{proc}, []*models.ManagedService{svc1, svc2})
	if result.Status != "unknown" {
		t.Errorf("expected status unknown for ambiguous identity, got %q", result.Status)
	}
}

func TestReconcile_ClearsStaleMetadata(t *testing.T) {
	t.Parallel()

	pid := 9999
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     "/project/app",
		LastPID: &pid,
	}

	result := Reconcile(svc, []*models.ProcessRecord{}, []*models.ManagedService{svc})
	if !result.HasStaleMetadata {
		t.Error("Reconcile should clear stale metadata when PID no longer exists")
	}
}

func TestReconcile_PIDReuse_Unknown(t *testing.T) {
	t.Parallel()

	pid := 1234
	svc := &models.ManagedService{
		Name:    "api",
		CWD:     "/project/app",
		LastPID: &pid,
	}
	// Same PID but completely different process
	proc := &models.ProcessRecord{
		PID:     1234,
		CWD:     "/other/app",
		Command: "python server.py",
		Port:    5000,
	}

	result := Reconcile(svc, []*models.ProcessRecord{proc}, []*models.ManagedService{svc})
	if result.Verified {
		t.Error("PID reuse should NOT verify the service")
	}
}
