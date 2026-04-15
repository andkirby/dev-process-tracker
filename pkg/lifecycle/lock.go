package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// FileLock implements per-service exclusive locks using file-based primitives.
// Locks are daemonless and recoverable by timeout.
type FileLock struct {
	lockDir string
	timeout time.Duration
}

// NewFileLock creates a new FileLock with the given base directory.
func NewFileLock(dir string) *FileLock {
	return &FileLock{
		lockDir: dir,
		timeout: 30 * time.Second,
	}
}

// Acquire attempts to acquire an exclusive lock for the given service.
// Returns an error if the lock is already held by another process.
func (lk *FileLock) Acquire(serviceName string, pid int) error {
	lockDir := filepath.Join(lk.lockDir, "locks")
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return err
	}

	lockPath := filepath.Join(lockDir, serviceName+".lock")

	// Try atomic creation
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		// Successfully created - we own the lock
		lk.writeLockFile(file, pid)
		return nil
	}

	// Lock file exists — check if it's stale by timeout or dead owner
	if lk.isStaleLock(lockPath) {
		// Stale — reclaim
		os.Remove(lockPath)
		file, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			lk.writeLockFile(file, pid)
			return nil
		}
		return err
	}

	// Lock is actively held — blocked
	return ErrLockBlocked
}

// writeLockFile writes the lock file content with timestamp and PID.
func (lk *FileLock) writeLockFile(file *os.File, pid int) {
	content := fmt.Sprintf("%s\nPID=%d", time.Now().Format(time.RFC3339), pid)
	file.WriteString(content)
	file.Close()
}

// isStaleLock returns true if the lock file's owner is dead
// or the lock has exceeded the configured timeout.
func (lk *FileLock) isStaleLock(lockPath string) bool {
	// Check timeout first — if lock file is older than timeout, it's stale
	info, err := os.Stat(lockPath)
	if err != nil {
		return true
	}
	if lk.timeout > 0 && time.Since(info.ModTime()) > lk.timeout {
		return true
	}

	// Check if owner process is alive
	return !lk.isOwnerAlive(lockPath)
}

// Release releases the lock for the given service.
// Returns nil if the lock was not held (idempotent).
func (lk *FileLock) Release(serviceName string) error {
	lockPath := filepath.Join(lk.lockDir, "locks", serviceName+".lock")
	err := os.Remove(lockPath)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

// IsLocked checks whether a lock exists for the given service.
func (lk *FileLock) IsLocked(serviceName string) bool {
	lockPath := filepath.Join(lk.lockDir, "locks", serviceName+".lock")
	_, err := os.Stat(lockPath)
	return err == nil
}

func (lk *FileLock) isOwnerAlive(lockPath string) bool {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return false
	}
	// Parse PID from lock file
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PID=") {
			pidStr := strings.TrimPrefix(line, "PID=")
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				return false
			}
			// Check if process is alive
			return isProcessAlive(pid)
		}
	}
	return true // Conservative: assume alive if we can't determine
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Use syscall.Kill(pid, 0) which is the standard Unix way to check
	// if a process exists. Signal 0 doesn't actually send a signal but
	// checks if the process is alive and accessible.
	return syscallKill(pid, syscall.Signal(0)) == nil
}

// syscallKill sends signal 0 to check process liveness.
// Extracted as a function for testability.
var syscallKill = syscall.Kill

// ErrLockBlocked is returned when a lock cannot be acquired.
var ErrLockBlocked = fmt.Errorf("operation blocked: another operation is already in progress for this service")
