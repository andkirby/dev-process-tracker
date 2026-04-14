package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/devports/devpt/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// PrintServerTable
// ---------------------------------------------------------------------------

func TestPrintServerTable_EmptyServers(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := PrintServerTable(&buf, nil, false)
	require.NoError(t, err)

	// Should contain at least the header line
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.GreaterOrEqual(t, len(lines), 1, "header must be written even with empty servers")
}

func TestPrintServerTable_MultipleServers(t *testing.T) {
	t.Parallel()

	servers := []*models.ServerInfo{
		{
			ManagedService: &models.ManagedService{Name: "api", Command: "go run main.go", Ports: []int{3000}},
			ProcessRecord:  &models.ProcessRecord{PID: 1001, Port: 3000},
			Status:         "running",
		},
		{
			ManagedService: &models.ManagedService{Name: "worker", Command: "node server.js", Ports: []int{4000}},
			ProcessRecord:  &models.ProcessRecord{PID: 1002, Port: 4000},
			Status:         "running",
		},
	}

	var buf bytes.Buffer
	err := PrintServerTable(&buf, servers, false)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "api")
	assert.Contains(t, output, "worker")
	assert.Contains(t, output, "3000")
	assert.Contains(t, output, "4000")
}

func TestPrintServerTable_DetailedMode(t *testing.T) {
	t.Parallel()

	servers := []*models.ServerInfo{
		{
			ManagedService: &models.ManagedService{Name: "api", Command: "go run main.go", Ports: []int{3000}},
			ProcessRecord:  &models.ProcessRecord{PID: 1001, Port: 3000},
			Status:         "running",
		},
	}

	// Detailed mode includes Command column
	var detailed bytes.Buffer
	err := PrintServerTable(&detailed, servers, true)
	require.NoError(t, err)
	assert.Contains(t, detailed.String(), "Command")

	// Non-detailed mode does not include Command column header (only 6 columns)
	var normal bytes.Buffer
	err = PrintServerTable(&normal, servers, false)
	require.NoError(t, err)
	normalLines := strings.Split(strings.TrimSpace(normal.String()), "\n")
	require.GreaterOrEqual(t, len(normalLines), 1)
	// Non-detailed has 6 columns: Name, Port, PID, Project, Source, Status
	fields := strings.Split(normalLines[0], "\t")
	assert.Equal(t, 6, len(fields), "non-detailed header should have 6 columns")
}

// ---------------------------------------------------------------------------
// FormatServerRow
// ---------------------------------------------------------------------------

func TestFormatServerRow_NilManagedService(t *testing.T) {
	t.Parallel()

	srv := &models.ServerInfo{
		ProcessRecord: &models.ProcessRecord{PID: 9999, Port: 8080},
		Status:        "running",
		Source:        models.SourceManual,
	}

	row := FormatServerRow(srv, false)
	assert.Contains(t, row, "-") // name should be dash when no ManagedService
}

func TestFormatServerRow_FullProcessRecord(t *testing.T) {
	t.Parallel()

	srv := &models.ServerInfo{
		ManagedService: &models.ManagedService{
			Name:    "db",
			CWD:     "/workspace/db",
			Command: "postgres",
			Ports:   []int{5432},
		},
		ProcessRecord: &models.ProcessRecord{
			PID:         2001,
			Port:        5432,
			ProjectRoot: "/workspace/db",
		},
		Status: "running",
	}

	row := FormatServerRow(srv, false)
	assert.Contains(t, row, "db")
	assert.Contains(t, row, "5432")
	assert.Contains(t, row, "2001")
	assert.Contains(t, row, "/workspace/db")
}

func TestFormatServerRow_DetailedMode(t *testing.T) {
	t.Parallel()

	srv := &models.ServerInfo{
		ManagedService: &models.ManagedService{
			Name:    "api",
			Command: "go run main.go",
			Ports:   []int{3000},
		},
		ProcessRecord: &models.ProcessRecord{PID: 1001, Port: 3000},
		Status:        "running",
	}

	rowDetailed := FormatServerRow(srv, true)
	rowNormal := FormatServerRow(srv, false)

	// Detailed should include Command
	assert.Contains(t, rowDetailed, "go run main.go")
	// Detailed should have 7 columns vs 6 in normal
	detailedFields := strings.Split(rowDetailed, "\t")
	normalFields := strings.Split(rowNormal, "\t")
	assert.Equal(t, 7, len(detailedFields))
	assert.Equal(t, 6, len(normalFields))
}

// ---------------------------------------------------------------------------
// PrintServerStatus
// ---------------------------------------------------------------------------

func TestPrintServerStatus_ManagedService(t *testing.T) {
	t.Parallel()

	srv := &models.ServerInfo{
		ManagedService: &models.ManagedService{
			Name:    "api",
			Command: "go run main.go",
			CWD:     "/workspace/api",
			Ports:   []int{3000, 3001},
		},
		Status: "stopped",
	}

	var buf bytes.Buffer
	err := PrintServerStatus(&buf, srv, nil)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "api")
	assert.Contains(t, output, "go run main.go")
	assert.Contains(t, output, "/workspace/api")
	assert.Contains(t, output, "3000")
}

func TestPrintServerStatus_WithProcessRecord(t *testing.T) {
	t.Parallel()

	srv := &models.ServerInfo{
		ManagedService: &models.ManagedService{Name: "worker", Command: "node", CWD: "/app", Ports: []int{4000}},
		ProcessRecord:  &models.ProcessRecord{PID: 5000, Port: 4000, PPID: 1, User: "dev", Command: "node server.js", CWD: "/app"},
		Status:         "running",
	}

	var buf bytes.Buffer
	err := PrintServerStatus(&buf, srv, nil)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "5000")
	assert.Contains(t, output, "4000")
	assert.Contains(t, output, "dev")
}

func TestPrintServerStatus_DisplayCrashedWithReason(t *testing.T) {
	t.Parallel()

	srv := &models.ServerInfo{
		ManagedService: &models.ManagedService{Name: "flaky"},
		Status:         "crashed",
		CrashReason:    "panic: runtime error",
		CrashLogTail:   []string{"panic: runtime error", "goroutine 1 [running]", "main.main()"},
	}

	var buf bytes.Buffer
	err := PrintServerStatus(&buf, srv, nil)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "CRASH DETAILS")
	assert.Contains(t, output, "panic: runtime error")
}

func TestPrintServerStatus_CrashedWithoutReason(t *testing.T) {
	t.Parallel()

	srv := &models.ServerInfo{
		ManagedService: &models.ManagedService{Name: "mystery"},
		Status:         "crashed",
		CrashReason:    "",
	}

	var buf bytes.Buffer
	err := PrintServerStatus(&buf, srv, nil)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "unavailable")
}

// ---------------------------------------------------------------------------
// Interface contract: no App receiver
// ---------------------------------------------------------------------------

// Compile-time check: PrintServerTable, FormatServerRow, PrintServerStatus
// are package-level functions, not methods on *App.
// PrintServerStatus accepts io.Writer and a health check result (may be nil).
// If anyone adds an App receiver, the compile-time checks below will fail.
var _ = func(w io.Writer, servers []*models.ServerInfo, detailed bool) error {
	return PrintServerTable(w, servers, detailed)
}
var _ = func(srv *models.ServerInfo, detailed bool) string {
	return FormatServerRow(srv, detailed)
}
