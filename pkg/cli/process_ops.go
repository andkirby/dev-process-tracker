package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devports/devpt/pkg/models"
	"github.com/devports/devpt/pkg/process"
)

// defaultStopTimeout is the sole source of truth for stop operation timeouts.
const defaultStopTimeout time.Duration = 5 * time.Second

// StopResult holds the outcome of a StopProcess call.
type StopResult struct {
	Stopped      bool
	AlreadyDead  bool
	SudoRequired bool
	ClearedPID   bool
	ClearError   error
}

// StopProcess stops a process by PID using the given process manager.
// It returns a structured StopResult without any IO side-effects.
func StopProcess(pm *process.Manager, pid int, timeout time.Duration) StopResult {
	err := pm.Stop(pid, timeout)

	if err == nil {
		return StopResult{Stopped: true}
	}

	if errors.Is(err, process.ErrNeedSudo) {
		return StopResult{SudoRequired: true}
	}

	if isProcessFinishedErr(err) {
		return StopResult{AlreadyDead: true}
	}

	return StopResult{
		Stopped: false,
		ClearError: fmt.Errorf("failed to stop process: %w", err),
	}
}

// isProcessFinishedErr reports whether err indicates the process had already exited.
func isProcessFinishedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "process already finished") || strings.Contains(msg, "no such process")
}

// ValidateRunningPID resolves the current PID for a managed service.
// It checks live server info first, then falls back to LastPID with
// an ambiguity guard.
func ValidateRunningPID(
	svc *models.ManagedService,
	servers []*models.ServerInfo,
	isRunning func(int) bool,
) (int, error) {
	return validatedManagedPIDFromServers(svc, servers, isRunning)
}

// managedServicePID returns the PID for a named service from live server info.
func managedServicePID(servers []*models.ServerInfo, serviceName string) int {
	for _, srv := range servers {
		if srv == nil || srv.ManagedService == nil || srv.ProcessRecord == nil {
			continue
		}
		if srv.ManagedService.Name == serviceName {
			return srv.ProcessRecord.PID
		}
	}
	return 0
}

// validatedManagedPIDFromServers resolves a service's PID, guarding against
// stale LastPID values that are still running under an unmanaged process.
func validatedManagedPIDFromServers(
	svc *models.ManagedService,
	servers []*models.ServerInfo,
	isRunning func(int) bool,
) (int, error) {
	if svc == nil {
		return 0, nil
	}

	if pid := managedServicePID(servers, svc.Name); pid != 0 {
		return pid, nil
	}

	if svc.LastPID != nil && *svc.LastPID > 0 && isRunning != nil && isRunning(*svc.LastPID) {
		return 0, fmt.Errorf(
			"cannot safely determine PID for service %q; stored PID is no longer validated against a live managed process",
			svc.Name,
		)
	}

	return 0, nil
}
