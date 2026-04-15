package lifecycle

import (
	"fmt"
	"time"

	"github.com/devports/devpt/pkg/models"
)

// RestartService executes the restart flow:
// resolve → lock → reconcile → stop old → confirm gone → preflight → spawn new → verify identity+readiness → persist → release.
func RestartService(deps Deps, svc *models.ManagedService) Result {
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

	oldPID := 0
	hadOldInstance := false

	switch reconciled.Status {
	case string(models.StatusRunning):
		if !reconciled.Verified || reconciled.Process == nil {
			return Result{
				Outcome: OutcomeBlocked,
				Message: fmt.Sprintf("Blocked: identity for %q is ambiguous; refusing to restart.", svc.Name),
			}
		}
		oldPID = reconciled.Process.PID
		hadOldInstance = true

		// Stop the old instance
		if err := deps.StopProcess(oldPID); err != nil {
			return Result{
				Outcome: OutcomeBlocked,
				Message: fmt.Sprintf("Blocked: could not stop old instance (PID %d) of %q: %v", oldPID, svc.Name, err),
				PID:     oldPID,
			}
		}

		// Confirm old instance is gone
		if deps.IsRunning(oldPID) {
			return Result{
				Outcome: OutcomeBlocked,
				Message: fmt.Sprintf("Blocked: old instance of %q still owns resources (PID %d).", svc.Name, oldPID),
				PID:     oldPID,
			}
		}

	case string(models.StatusUnknown):
		return Result{
			Outcome: OutcomeBlocked,
			Message: fmt.Sprintf("Blocked: identity for %q is ambiguous; refusing to restart.", svc.Name),
		}

	case string(models.StatusCrashed):
		// Clear stale metadata
		_ = deps.ClearServicePID(svc.Name)
		// Fall through to start fresh

	case string(models.StatusStopped):
		// No old instance — fall through to start fresh
	}

	// Clear any remaining stale metadata before fresh start
	if !hadOldInstance {
		_ = deps.ClearServicePID(svc.Name)
	}

	// Wait briefly for resources (ports) to be released after stopping old instance
	if hadOldInstance {
		portReleasePause()
	}

	// Preflight checks — when we just stopped the old instance, skip port conflict
	// checks for the service's own declared ports (they may not be freed yet).
	processesAfterStop, _ := deps.ScanProcesses()
	if err := preflightCheckForRestart(svc, processesAfterStop); err != nil {
		outcome := OutcomeBlocked
		if !isPortConflict(err) {
			outcome = OutcomeInvalid
		}
		return Result{
			Outcome: outcome,
			Message: fmt.Sprintf("%s: %s", capitalizeOutcome(string(outcome)), err.Error()),
		}
	}

	// Spawn new instance
	newPID, err := deps.StartProcess(svc)
	if err != nil {
		msg := fmt.Sprintf("Failed: could not start new instance of %q: %v", svc.Name, err)
		if hadOldInstance {
			msg = fmt.Sprintf("Failed: %q was stopped, but the replacement instance could not start: %v", svc.Name, err)
		}
		return Result{
			Outcome: OutcomeFailed,
			Message: msg,
		}
	}

	// Verify process is alive
	if !deps.IsRunning(newPID) {
		return Result{
			Outcome: OutcomeFailed,
			Message: fmt.Sprintf("Failed: new instance of %q exited immediately. Check logs with devpt logs %s.", svc.Name, svc.Name),
			Diagnostics: deps.GetLogTail(svc.Name, 10),
		}
	}

	// Freshness rule: new PID must differ from old
	if hadOldInstance && newPID == oldPID {
		return Result{
			Outcome: OutcomeFailed,
			Message: fmt.Sprintf("Failed: new instance of %q has the same PID as the old one (PID %d); restart is not valid.", svc.Name, newPID),
		}
	}

	// Wait for readiness
	policy := SelectReadinessPolicy(svc.Readiness, svc.Ports)
	readinessErr := policy.Wait(
		newPID,
		svc.Ports,
		&depsProcessChecker{deps: deps},
		&depsHealthChecker{deps: deps},
		func() []string { return deps.GetLogTail(svc.Name, 5) },
	)

	if readinessErr != nil {
		diagnostics := deps.GetLogTail(svc.Name, 20)
		_ = deps.StopProcess(newPID)
		msg := fmt.Sprintf("Failed: %q was stopped, but the replacement instance did not become ready within %v.", svc.Name, policy.Timeout)
		if !hadOldInstance {
			msg = fmt.Sprintf("Failed: %q did not become ready within %v. Check logs with devpt logs %s.", svc.Name, policy.Timeout, svc.Name)
		}
		return Result{
			Outcome:     OutcomeFailed,
			Message:     msg,
			PID:         newPID,
			Diagnostics: diagnostics,
		}
	}

	// Persist confirmed run
	if err := deps.UpdateServicePID(svc.Name, newPID); err != nil {
		return Result{
			Outcome: OutcomeSuccess,
			Message: fmt.Sprintf("Success: started %q (PID %d), but failed to update registry: %v", svc.Name, newPID, err),
			PID:     newPID,
		}
	}

	// Format message based on whether we had an old instance
	var message string
	if hadOldInstance {
		portMsg := ""
		if len(svc.Ports) > 0 {
			portMsg = fmt.Sprintf(" on port %d", svc.Ports[0])
		}
		message = fmt.Sprintf("Success: restarted %q%s (old PID %d, new PID %d).", svc.Name, portMsg, oldPID, newPID)
	} else {
		portMsg := ""
		if len(svc.Ports) > 0 {
			portMsg = fmt.Sprintf(" on port %d", svc.Ports[0])
		}
		message = fmt.Sprintf("Success: started %q because no verified instance was running%s (PID %d).", svc.Name, portMsg, newPID)
	}

	return Result{
		Outcome: OutcomeSuccess,
		Message: message,
		PID:     newPID,
	}
}

// preflightCheckForRestart runs CWD and command validation but skips port
// conflict checks. During restart, the service's own ports may not be freed
// yet after stopping the old instance, and we don't want to falsely report
// a conflict.
func preflightCheckForRestart(svc *models.ManagedService, _ []*models.ProcessRecord) error {
	return preflightCheck(svc, nil)
}

// portReleasePause waits briefly for the OS to release resources
// (e.g., TCP ports in TIME_WAIT) after stopping a process.
func portReleasePause() {
	time.Sleep(500 * time.Millisecond)
}
