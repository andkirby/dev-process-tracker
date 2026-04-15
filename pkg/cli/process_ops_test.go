package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// defaultStopTimeout
// ---------------------------------------------------------------------------

func TestDefaultStopTimeout_IsFiveSeconds(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 5*time.Second, defaultStopTimeout, "defaultStopTimeout must be exactly 5 seconds")
}

// ---------------------------------------------------------------------------
// StopProcess / StopResult
// ---------------------------------------------------------------------------

func TestStopProcess_ResultFields(t *testing.T) {
	t.Parallel()

	var result StopResult
	assert.IsType(t, result, StopResult{}, "StopResult must be a struct")
	assert.Equal(t, false, result.Stopped)
	assert.Equal(t, false, result.AlreadyDead)
	assert.Equal(t, false, result.SudoRequired)
	assert.Equal(t, false, result.ClearedPID)
	assert.Nil(t, result.ClearError)

	// Verify all field combinations
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
