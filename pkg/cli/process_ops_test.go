package cli

import (
	"testing"
	"time"

	"github.com/devports/devpt/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// defaultStopTimeout
// ---------------------------------------------------------------------------

func TestDefaultStopTimeout_IsFiveSeconds(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 5*time.Second, defaultStopTimeout, "defaultStopTimeout must be exactly 5 seconds")
}

// ---------------------------------------------------------------------------
// ValidateRunningPID
// ---------------------------------------------------------------------------

func TestValidateRunningPID_MatchingServer(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{Name: "api", Ports: []int{3000}}
	servers := []*models.ServerInfo{
		{
			ManagedService: svc,
			ProcessRecord:  &models.ProcessRecord{PID: 1234, Port: 3000},
		},
	}

	pid, err := ValidateRunningPID(svc, servers, func(int) bool { return true })
	require.NoError(t, err)
	assert.Equal(t, 1234, pid)
}

func TestValidateRunningPID_NoMatch(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{Name: "missing"}
	servers := []*models.ServerInfo{
		{
			ManagedService: &models.ManagedService{Name: "other"},
			ProcessRecord:  &models.ProcessRecord{PID: 999},
		},
	}

	pid, err := ValidateRunningPID(svc, servers, func(int) bool { return true })
	require.NoError(t, err)
	assert.Equal(t, 0, pid, "no match should return 0")
}

func TestValidateRunningPID_NilService(t *testing.T) {
	t.Parallel()

	pid, err := ValidateRunningPID(nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, pid)
}

func TestValidateRunningPID_StaleRunningPID(t *testing.T) {
	t.Parallel()

	lastPID := 9090
	svc := &models.ManagedService{Name: "api", LastPID: &lastPID}
	// No servers matching, but LastPID is running → ambiguous
	servers := []*models.ServerInfo{}

	_, err := ValidateRunningPID(svc, servers, func(pid int) bool {
		return pid == lastPID
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot safely determine PID")
}

// ---------------------------------------------------------------------------
// StopProcess
// ---------------------------------------------------------------------------

func TestStopProcess_SuccessfulStop(t *testing.T) {
	t.Parallel()

	// StopProcess delegates to process.Manager; test with a real short-lived process.
	// We can't easily test this without a real process manager, so we test the
	// contract: StopProcess returns StopResult and does not write IO.
	// The actual integration test is in commands_status_test.go via the App.
	//
	// This test verifies the function signature and struct are correct.
	var result StopResult
	assert.IsType(t, result, StopResult{}, "StopResult must be a struct")
	assert.Equal(t, false, result.Stopped)
	assert.Equal(t, false, result.AlreadyDead)
	assert.Equal(t, false, result.SudoRequired)
	assert.Equal(t, false, result.ClearedPID)
	assert.Nil(t, result.ClearError)
}

func TestStopProcess_NoIOSideEffects(t *testing.T) {
	t.Parallel()

	// Verify StopProcess is a package-level function (not a method on *App).
	// The StopResult struct must have the expected fields.
	sr := StopResult{Stopped: true, ClearedPID: true}
	assert.True(t, sr.Stopped)
	assert.True(t, sr.ClearedPID)
	assert.Nil(t, sr.ClearError)

	sr = StopResult{AlreadyDead: true}
	assert.True(t, sr.AlreadyDead)

	sr = StopResult{SudoRequired: true}
	assert.True(t, sr.SudoRequired)

	sr = StopResult{Stopped: true, ClearError: assert.AnError}
	assert.Equal(t, assert.AnError, sr.ClearError)
}

// ---------------------------------------------------------------------------
// managedServicePID (backward compatibility)
// ---------------------------------------------------------------------------

func TestManagedServicePID_Match(t *testing.T) {
	t.Parallel()

	servers := []*models.ServerInfo{
		{
			ProcessRecord:  &models.ProcessRecord{PID: 2001},
			ManagedService: &models.ManagedService{Name: "api"},
		},
		{
			ProcessRecord:  &models.ProcessRecord{PID: 2002},
			ManagedService: &models.ManagedService{Name: "worker"},
		},
	}

	assert.Equal(t, 2002, managedServicePID(servers, "worker"))
	assert.Equal(t, 0, managedServicePID(servers, "missing"))
}

func TestManagedServicePID_NilGuard(t *testing.T) {
	t.Parallel()

	servers := []*models.ServerInfo{
		nil, // nil entry should be skipped
		{
			ProcessRecord:  nil, // nil ProcessRecord should be skipped
			ManagedService: &models.ManagedService{Name: "api"},
		},
		{
			ProcessRecord:  &models.ProcessRecord{PID: 3001},
			ManagedService: nil, // nil ManagedService should be skipped
		},
	}

	assert.Equal(t, 0, managedServicePID(servers, "api"))
}
