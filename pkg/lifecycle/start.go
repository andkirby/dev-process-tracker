package lifecycle

import (
	"fmt"
	"os"
	"strings"

	"github.com/devports/devpt/pkg/models"
)

// Deps provides the external dependencies needed by lifecycle flows.
// Using an interface allows testing without real process spawning.
type Deps interface {
	// Registry operations
	GetService(name string) *models.ManagedService
	UpdateServicePID(name string, pid int) error
	ClearServicePID(name string) error

	// Process operations
	StartProcess(svc *models.ManagedService) (int, error)
	StopProcess(pid int) error
	IsRunning(pid int) bool

	// Scanning
	ScanProcesses() ([]*models.ProcessRecord, error)
	ListServices() []*models.ManagedService

	// Health checking
	CheckHealth(port int) bool

	// Log access
	GetLogTail(name string, lines int) []string

	// Locking
	AcquireLock(serviceName string) error
	ReleaseLock(serviceName string)

	// Identity resolution
	ResolveProjectRoot(cwd string) string
}

// StartService executes the start flow:
// resolve → lock → reconcile → preflight → spawn → verify identity → wait readiness → persist → release.
func StartService(deps Deps, svc *models.ManagedService) Result {
	if deps == nil || svc == nil {
		return Result{Outcome: OutcomeInvalid, Message: "invalid: nil dependencies or service"}
	}

	// Acquire lock
	if err := deps.AcquireLock(svc.Name); err != nil {
		return Result{
			Outcome: OutcomeBlocked,
			Message: fmt.Sprintf("Blocked: another operation is already in progress for %q. Retry after it completes.", svc.Name),
		}
	}
	defer deps.ReleaseLock(svc.Name)

	// Scan live processes
	processes, err := deps.ScanProcesses()
	if err != nil {
		return Result{
			Outcome: OutcomeFailed,
			Message: fmt.Sprintf("Failed: could not scan live processes for %q: %v", svc.Name, err),
		}
	}

	allServices := deps.ListServices()

	// Reconcile
	reconciled := ReconcileWithResolver(svc, processes, allServices, deps.ResolveProjectRoot)

	switch reconciled.Status {
	case string(models.StatusRunning):
		if reconciled.Verified && reconciled.Process != nil {
			return Result{
				Outcome: OutcomeNoop,
				Message: fmt.Sprintf("No-op: %q is already running (PID %d).", svc.Name, reconciled.Process.PID),
				PID:     reconciled.Process.PID,
			}
		}
	case string(models.StatusUnknown):
		return Result{
			Outcome: OutcomeBlocked,
			Message: fmt.Sprintf("Blocked: identity for %q is ambiguous; refusing to start a potentially duplicate instance.", svc.Name),
		}
	case string(models.StatusCrashed):
		// Stale metadata detected — proceed with fresh start (callers clear it)
	}

	// Preflight checks
	if err := preflightCheck(svc, processes); err != nil {
		outcome := OutcomeInvalid
		if isPortConflict(err) {
			outcome = OutcomeBlocked
		}
		return Result{
			Outcome: outcome,
			Message: fmt.Sprintf("%s: %s", capitalizeOutcome(string(outcome)), err.Error()),
		}
	}

	// Spawn process
	pid, err := deps.StartProcess(svc)
	if err != nil {
		return Result{
			Outcome: OutcomeFailed,
			Message: fmt.Sprintf("Failed: could not start %q: %v", svc.Name, err),
		}
	}

	// Verify process is alive
	if !deps.IsRunning(pid) {
		return Result{
			Outcome: OutcomeFailed,
			Message: fmt.Sprintf("Failed: %q exited immediately after start. Check logs with devpt logs %s.", svc.Name, svc.Name),
			Diagnostics: deps.GetLogTail(svc.Name, 10),
		}
	}

	// Wait for readiness
	policy := SelectReadinessPolicy(svc.Readiness, svc.Ports)
	readinessErr := policy.Wait(
		pid,
		svc.Ports,
		&depsProcessChecker{deps: deps},
		&depsHealthChecker{deps: deps},
		func() []string { return deps.GetLogTail(svc.Name, 5) },
	)

	if readinessErr != nil {
		// Readiness failed — collect diagnostics and kill the child
		diagnostics := deps.GetLogTail(svc.Name, 20)
		_ = deps.StopProcess(pid)
		return Result{
			Outcome: OutcomeFailed,
			Message: fmt.Sprintf("Failed: %q did not become ready within %v. Check logs with devpt logs %s.",
				svc.Name, policy.Timeout, svc.Name),
			PID:         pid,
			Diagnostics: diagnostics,
		}
	}

	// Persist confirmed run (C6: only after identity and readiness confirmed)
	if err := deps.UpdateServicePID(svc.Name, pid); err != nil {
		return Result{
			Outcome: OutcomeSuccess,
			Message: fmt.Sprintf("Success: started %q (PID %d), but failed to update registry: %v", svc.Name, pid, err),
			PID:     pid,
		}
	}

	portMsg := ""
	if len(svc.Ports) > 0 {
		portMsg = fmt.Sprintf(" on port %d", svc.Ports[0])
	}
	return Result{
		Outcome: OutcomeSuccess,
		Message: fmt.Sprintf("Success: started %q%s (PID %d).", svc.Name, portMsg, pid),
		PID:     pid,
	}
}

func preflightCheck(svc *models.ManagedService, processes []*models.ProcessRecord) error {
	// Check working directory exists and is a directory
	if fi, err := os.Stat(svc.CWD); err != nil {
		return fmt.Errorf("%q has a missing working directory: %s", svc.Name, svc.CWD)
	} else if !fi.IsDir() {
		return fmt.Errorf("%q has an invalid working directory: %s is not a directory", svc.Name, svc.CWD)
	}

	// Check command is not empty
	cmd := strings.TrimSpace(svc.Command)
	if cmd == "" {
		return fmt.Errorf("%q has an empty command definition", svc.Name)
	}

	// Check declared ports are free
	for _, port := range svc.Ports {
		for _, proc := range processes {
			if proc != nil && proc.Port == port {
				return fmt.Errorf("port %d is in use by PID %d (%s). Stop it or change the service port.",
					port, proc.PID, proc.Command)
			}
		}
	}

	return nil
}

func isPortConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "port ")
}

func capitalizeOutcome(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// depsProcessChecker adapts Deps to ProcessChecker interface.
type depsProcessChecker struct {
	deps Deps
}

func (d *depsProcessChecker) IsRunning(pid int) bool {
	return d.deps.IsRunning(pid)
}

// depsHealthChecker adapts Deps to HealthChecker interface.
type depsHealthChecker struct {
	deps Deps
}

func (d *depsHealthChecker) Check(port int) bool {
	return d.deps.CheckHealth(port)
}
