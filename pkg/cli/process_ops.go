package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

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
// This is the low-level PID kill used by the lifecycle adapter and
// the TUI for raw (unmanaged) process termination.
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
		Stopped:    false,
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
