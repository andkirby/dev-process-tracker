package lifecycle

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireLock_Fresh(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lk := NewFileLock(dir)

	err := lk.Acquire("test-service", os.Getpid())
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer lk.Release("test-service")

	if !lk.IsLocked("test-service") {
		t.Error("IsLocked() should return true after acquire")
	}
}

func TestAcquireLock_Concurrent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lk := NewFileLock(dir)

	err := lk.Acquire("test-service", os.Getpid())
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}
	defer lk.Release("test-service")

	// Second acquire on same service should fail
	err = lk.Acquire("test-service", os.Getpid()+99999)
	if err == nil {
		t.Error("second Acquire() should return error (blocked)")
	}
}

func TestAcquireLock_DifferentServices(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lk := NewFileLock(dir)

	err1 := lk.Acquire("service-a", os.Getpid())
	if err1 != nil {
		t.Fatalf("Acquire(service-a) error = %v", err1)
	}
	defer lk.Release("service-a")

	err2 := lk.Acquire("service-b", os.Getpid())
	if err2 != nil {
		t.Fatalf("Acquire(service-b) error = %v", err2)
	}
	defer lk.Release("service-b")
}

func TestReleaseLock(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lk := NewFileLock(dir)

	lk.Acquire("test-service", os.Getpid())

	err := lk.Release("test-service")
	if err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	if lk.IsLocked("test-service") {
		t.Error("IsLocked() should return false after release")
	}
}

func TestReleaseLock_NotHeld(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lk := NewFileLock(dir)

	// Releasing a non-held lock should be a no-op
	err := lk.Release("nonexistent-service")
	if err != nil {
		t.Fatalf("Release() on non-held lock should be no-op, got error = %v", err)
	}
}

func TestIsLocked_NotLocked(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lk := NewFileLock(dir)

	if lk.IsLocked("nonexistent-service") {
		t.Error("IsLocked() should return false for non-existent lock")
	}
}

func TestLockFileContents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lk := NewFileLock(dir)
	pid := os.Getpid()

	lk.Acquire("test-service", pid)
	defer lk.Release("test-service")

	lockPath := filepath.Join(dir, "locks", "test-service.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("failed to read lock file: %v", err)
	}

	if len(data) == 0 {
		t.Error("lock file should contain PID and timestamp")
	}
}

func TestStaleLockRecovery_DeadOwner(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lk := NewFileLock(dir)

	// Create a stale lock with a PID that doesn't exist
	stalePID := 999999 // Very unlikely to be running
	lk.Acquire("test-service", stalePID)

	// Attempt to acquire with a different PID should succeed after timeout recovery
	err := lk.Acquire("test-service", os.Getpid())
	if err != nil {
		t.Fatalf("Acquire() on stale lock with dead owner should succeed, got error = %v", err)
	}
	defer lk.Release("test-service")
}

func TestStaleLockRecovery_AliveOwner(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lk := NewFileLock(dir)

	// Hold lock with current PID
	lk.Acquire("test-service", os.Getpid())
	defer lk.Release("test-service")

	// Attempt to acquire with a different (fake) PID should fail
	// because the owner (current process) is still alive
	err := lk.Acquire("test-service", os.Getpid()+99999)
	if err == nil {
		t.Error("Acquire() should fail when owner PID is still alive")
	}
}

func TestLockTimeoutRecovery(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lk := &FileLock{
		lockDir: dir,
		timeout: 1 * time.Second,
	}

	// Create stale lock with dead PID
	stalePID := 999999
	lk.Acquire("test-service", stalePID)

	// Wait briefly then try to reclaim
	time.Sleep(100 * time.Millisecond)
	err := lk.Acquire("test-service", os.Getpid())
	if err != nil {
		t.Fatalf("Acquire() should succeed after timeout with dead owner, got error = %v", err)
	}
	defer lk.Release("test-service")
}
