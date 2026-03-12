package cli

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFormatBatchResult_Success formats successful start result
func TestFormatBatchResult_Success(t *testing.T) {
	result := BatchResult{
		Service: "api",
		Action:  "start",
		Success: true,
		PID:     12345,
	}

	output := captureOutput(func() {
		FormatBatchResult(result)
	})

	assert.Contains(t, output, "api", "Should show service name")
	assert.Contains(t, output, "started", "Should show action")
	assert.Contains(t, output, "12345", "Should show PID")
}

// TestFormatBatchResult_Stop formats successful stop result
func TestFormatBatchResult_Stop(t *testing.T) {
	result := BatchResult{
		Service: "worker",
		Action:  "stop",
		Success: true,
	}

	output := captureOutput(func() {
		FormatBatchResult(result)
	})

	assert.Contains(t, output, "worker", "Should show service name")
	assert.Contains(t, output, "stopped", "Should show action")
}

// TestFormatBatchResult_Restart formats successful restart result
func TestFormatBatchResult_Restart(t *testing.T) {
	result := BatchResult{
		Service: "frontend",
		Action:  "restart",
		Success: true,
		PID:     54321,
	}

	output := captureOutput(func() {
		FormatBatchResult(result)
	})

	assert.Contains(t, output, "frontend", "Should show service name")
	assert.Contains(t, output, "restarted", "Should show action")
	assert.Contains(t, output, "54321", "Should show new PID")
}

// TestFormatBatchResult_Failure formats error result
func TestFormatBatchResult_Failure(t *testing.T) {
	result := BatchResult{
		Service: "database",
		Action:  "start",
		Success: false,
		Error:   "service not found",
	}

	output := captureOutput(func() {
		FormatBatchResult(result)
	})

	assert.Contains(t, output, "database", "Should show service name")
	assert.Contains(t, output, "not found", "Should show error message")
}

// TestFormatBatchResult_Warning formats warning result
func TestFormatBatchResult_Warning(t *testing.T) {
	result := BatchResult{
		Service: "api",
		Action:  "start",
		Success: false,
		Warning: "already running with PID 12345",
	}

	output := captureOutput(func() {
		FormatBatchResult(result)
	})

	assert.Contains(t, output, "api", "Should show service name")
	assert.Contains(t, output, "Warning", "Should indicate warning")
	assert.Contains(t, output, "already running", "Should show warning message")
}

// TestFormatBatchResults_Multiple formats multiple results in order
func TestFormatBatchResults_Multiple(t *testing.T) {
	results := []BatchResult{
		{Service: "api", Action: "start", Success: true, PID: 11111},
		{Service: "worker", Action: "start", Success: true, PID: 22222},
		{Service: "frontend", Action: "start", Success: false, Error: "not found"},
	}

	output := captureOutput(func() {
		FormatBatchResults(results)
	})

	// Check that results appear in order
	apiPos := findSubstring(output, "api")
	workerPos := findSubstring(output, "worker")
	frontendPos := findSubstring(output, "frontend")

	assert.Less(t, apiPos, workerPos, "api should appear before worker")
	assert.Less(t, workerPos, frontendPos, "worker should appear before frontend")
}

// TestFormatBatchResults_PatternExpansion shows pattern match count
func TestFormatBatchResults_PatternExpansion(t *testing.T) {
	results := []BatchResult{
		{Service: "web-api", Action: "start", Success: true, PID: 11111},
		{Service: "web-frontend", Action: "start", Success: true, PID: 22222},
	}

	output := captureOutput(func() {
		FormatBatchResultsWithPattern(results, "web-*")
	})

	assert.Contains(t, output, "Pattern 'web-*' matched 2 services", "Should show pattern match count")
	assert.Contains(t, output, "web-api", "Should show first service")
	assert.Contains(t, output, "web-frontend", "Should show second service")
}

// TestFormatBatchResults_AllSuccess shows summary
func TestFormatBatchResults_AllSuccess(t *testing.T) {
	results := []BatchResult{
		{Service: "api", Action: "start", Success: true, PID: 11111},
		{Service: "worker", Action: "start", Success: true, PID: 22222},
	}

	output := captureOutput(func() {
		FormatBatchResults(results)
	})

	assert.Contains(t, output, "All services started successfully", "Should show success summary")
}

// TestFormatBatchResults_PartialFailure shows failure count
func TestFormatBatchResults_PartialFailure(t *testing.T) {
	results := []BatchResult{
		{Service: "api", Action: "start", Success: true, PID: 11111},
		{Service: "invalid", Action: "start", Success: false, Error: "not found"},
	}

	output := captureOutput(func() {
		FormatBatchResults(results)
	})

	assert.Contains(t, output, "1 of 2 services failed", "Should show failure summary")
}

// TestFormatBatchResults_AllFailure shows error summary
func TestFormatBatchResults_AllFailure(t *testing.T) {
	results := []BatchResult{
		{Service: "svc1", Action: "start", Success: false, Error: "error1"},
		{Service: "svc2", Action: "start", Success: false, Error: "error2"},
	}

	output := captureOutput(func() {
		FormatBatchResults(results)
	})

	assert.Contains(t, output, "All 2 services failed", "Should show all failed summary")
}

// Helper function to capture stdout
func captureOutput(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// Helper function to find substring position
func findSubstring(s, substr string) int {
	return bytes.Index([]byte(s), []byte(substr))
}
