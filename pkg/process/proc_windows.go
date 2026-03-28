//go:build windows

package process

import (
	"os/exec"
	"strconv"
)

func setProcessGroup(cmd *exec.Cmd) {
	// Windows: no special process group setup needed for basic use
	// The process will be managed by its PID
}

func terminateProcess(pid int) error {
	return terminateProcessFallback(pid)
}

func terminateProcessFallback(pid int) error {
	// On Windows, use taskkill for graceful termination
	return exec.Command("taskkill", "/PID", strconv.Itoa(pid)).Run()
}

func killProcess(pid int) error {
	return killProcessFallback(pid)
}

func killProcessFallback(pid int) error {
	// On Windows, use taskkill /F for forceful termination
	return exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run()
}

func isProcessAlive(pid int) bool {
	// Check if process exists using tasklist
	err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid)).Run()
	return err == nil
}
