package lifecycle

import (
	"fmt"

	"github.com/devports/devpt/pkg/models"
)

// StopService executes the stop flow:
// resolve → lock → reconcile → verify identity → SIGTERM → wait → SIGKILL if needed → confirm gone → clear metadata → release.
func StopService(deps Deps, svc *models.ManagedService) Result {
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
	case string(models.StatusStopped):
		return Result{
			Outcome: OutcomeNoop,
			Message: fmt.Sprintf("No-op: %q is already stopped.", svc.Name),
		}
	case string(models.StatusUnknown):
		return Result{
			Outcome: OutcomeBlocked,
			Message: fmt.Sprintf("Blocked: PID cannot be proven to belong to %q; refusing to kill.", svc.Name),
		}
	case string(models.StatusCrashed):
		// Stale metadata — clear it
		_ = deps.ClearServicePID(svc.Name)
		return Result{
			Outcome: OutcomeNoop,
			Message: fmt.Sprintf("No-op: stale PID was cleared for %q.", svc.Name),
		}
	case string(models.StatusRunning):
		if !reconciled.Verified || reconciled.Process == nil {
			return Result{
				Outcome: OutcomeBlocked,
				Message: fmt.Sprintf("Blocked: PID cannot be proven to belong to %q; refusing to kill.", svc.Name),
			}
		}
		// Proceed to stop
	default:
		return Result{
			Outcome: OutcomeInvalid,
			Message: fmt.Sprintf("Invalid: %q has unrecognized status %q.", svc.Name, reconciled.Status),
		}
	}

	// We have a verified process — stop it
	pid := reconciled.Process.PID
	if err := deps.StopProcess(pid); err != nil {
		return Result{
			Outcome: OutcomeFailed,
			Message: fmt.Sprintf("Failed: PID %d did not exit after SIGTERM and SIGKILL. Sudo may be required.", pid),
			PID:     pid,
		}
	}

	// Confirm process is gone
	if deps.IsRunning(pid) {
		return Result{
			Outcome: OutcomeFailed,
			Message: fmt.Sprintf("Failed: PID %d did not exit after SIGTERM and SIGKILL. Sudo may be required.", pid),
			PID:     pid,
		}
	}

	// Clear confirmed run metadata (C6: only after confirmed gone)
	_ = deps.ClearServicePID(svc.Name)

	return Result{
		Outcome: OutcomeSuccess,
		Message: fmt.Sprintf("Success: stopped %q (PID %d).", svc.Name, pid),
		PID:     pid,
	}
}
