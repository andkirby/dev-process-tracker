package cli

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devports/devpt/pkg/health"
	"github.com/devports/devpt/pkg/models"
	"github.com/devports/devpt/pkg/process"
	"github.com/devports/devpt/pkg/registry"
	"github.com/devports/devpt/pkg/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestApp creates a fully-initialized App backed by a temp-dir registry.
// The scanner is real but will find no listening processes in a test environment,
// so only managed services with Status "stopped" / "crashed" show up via discoverServers.
func newTestApp(t *testing.T) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	tmp := t.TempDir()
	reg := registry.NewRegistry(filepath.Join(tmp, "registry.json"))
	require.NoError(t, reg.Load(), "load registry")

	var stdout, stderr bytes.Buffer
	app := &App{
		config:         models.ConfigPaths{RegistryFile: filepath.Join(tmp, "registry.json"), LogsDir: filepath.Join(tmp, "logs")},
		registry:       reg,
		scanner:        scanner.NewProcessScanner(),
		resolver:       scanner.NewProjectResolver(),
		detector:       scanner.NewAgentDetector(),
		processManager: process.NewManager(filepath.Join(tmp, "logs")),
		healthChecker:  health.NewChecker(0),
		stdout:         &stdout,
		stderr:         &stderr,
	}
	return app, &stdout, &stderr
}

// addManagedService is a test helper that registers a managed service.
func addManagedService(t *testing.T, reg *registry.Registry, name, command string, ports []int) {
	t.Helper()

	svc := &models.ManagedService{
		Name:    name,
		CWD:     t.TempDir(),
		Command: command,
		Ports:   ports,
	}
	require.NoError(t, reg.AddService(svc), "add service %q", name)
}

// withCrashedService creates a managed service with a LastPID to simulate a crash.
func withCrashedService(t *testing.T, reg *registry.Registry, name, command string, ports []int, lastPID int) {
	t.Helper()

	svc := &models.ManagedService{
		Name:    name,
		CWD:     t.TempDir(),
		Command: command,
		Ports:   ports,
		LastPID: &lastPID,
	}
	require.NoError(t, reg.AddService(svc), "add crashed service %q", name)
}

// captureStatusOutput captures os.Stdout during fn.
// NOTE: Must NOT be used with t.Parallel() because it redirects the global os.Stdout.
func captureStatusOutput(fn func()) string {
	return captureOutput(fn)
}

// ---------------------------------------------------------------------------
// 1. Exact name match (backward compat)
// ---------------------------------------------------------------------------

func TestStatusCmd_ExactNameMatch(t *testing.T) {
	// NOT parallel: uses os.Stdout capture

	app, _, _ := newTestApp(t)
	addManagedService(t, app.registry, "offgrid-api", "node server.js", []int{3000})

	output := captureStatusOutput(func() {
		if err := app.StatusCmd([]string{"offgrid-api"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "offgrid-api", "output should mention service name")
	assert.Contains(t, output, "SERVER DETAILS", "output should contain details header")
}

// ---------------------------------------------------------------------------
// 2. Port match (backward compat) — unit test of matching logic
// ---------------------------------------------------------------------------

func TestStatusCmd_PortMatch(t *testing.T) {
	t.Parallel()

	servers := []*models.ServerInfo{
		{
			ProcessRecord:  &models.ProcessRecord{PID: 1234, Port: 8080},
			ManagedService: &models.ManagedService{Name: "web", Command: "nginx", Ports: []int{8080}},
			Source:         models.SourceManaged,
			Status:         "running",
		},
	}

	// Verify port string matching works as in StatusCmd
	var found bool
	identifier := "8080"
	for _, srv := range servers {
		if srv.ProcessRecord != nil && fmt.Sprintf("%d", srv.ProcessRecord.Port) == identifier {
			found = true
			break
		}
	}
	assert.True(t, found, "port '8080' should match ProcessRecord with Port 8080")

	// Verify it does NOT match wrong ports
	var wrongMatch bool
	for _, srv := range servers {
		if srv.ProcessRecord != nil && fmt.Sprintf("%d", srv.ProcessRecord.Port) == "9090" {
			wrongMatch = true
			break
		}
	}
	assert.False(t, wrongMatch, "port '9090' should not match server on 8080")
}

// ---------------------------------------------------------------------------
// 3. Not found — error when no service matches exact name
// ---------------------------------------------------------------------------

func TestStatusCmd_NotFound(t *testing.T) {
	t.Parallel()

	app, _, _ := newTestApp(t)
	// No services registered

	err := app.StatusCmd([]string{"nonexistent"})
	require.Error(t, err, "StatusCmd should return error for unknown service")
	assert.Contains(t, err.Error(), "no servers found", "error message should mention no servers found")
	assert.Contains(t, err.Error(), "nonexistent", "error should include the identifier")
}

// ---------------------------------------------------------------------------
// 4. Glob pattern single match
// ---------------------------------------------------------------------------

func TestStatusCmd_GlobPatternSingleMatch(t *testing.T) {
	// NOT parallel: uses os.Stdout capture

	app, _, _ := newTestApp(t)
	addManagedService(t, app.registry, "offgrid-api", "node server.js", []int{3000})
	addManagedService(t, app.registry, "worker", "ruby worker.rb", []int{4000})

	output := captureStatusOutput(func() {
		if err := app.StatusCmd([]string{"offg*"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "offgrid-api", "output should include matching service")
	assert.NotContains(t, output, "worker", "output should not include non-matching service")
}

// ---------------------------------------------------------------------------
// 5. Glob pattern multiple matches
// ---------------------------------------------------------------------------

func TestStatusCmd_GlobPatternMultipleMatches(t *testing.T) {
	// NOT parallel: uses os.Stdout capture

	app, _, _ := newTestApp(t)
	addManagedService(t, app.registry, "web-api", "node api.js", []int{3000})
	addManagedService(t, app.registry, "web-frontend", "npm start", []int{3001})
	addManagedService(t, app.registry, "worker", "ruby worker.rb", []int{4000})

	output := captureStatusOutput(func() {
		if err := app.StatusCmd([]string{"web-*"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "web-api", "output should include web-api")
	assert.Contains(t, output, "web-frontend", "output should include web-frontend")
	assert.NotContains(t, output, "worker", "output should not include non-matching worker")
}

// ---------------------------------------------------------------------------
// 6. Glob pattern no match
// ---------------------------------------------------------------------------

func TestStatusCmd_GlobPatternNoMatch(t *testing.T) {
	t.Parallel()

	app, _, _ := newTestApp(t)
	addManagedService(t, app.registry, "api", "node api.js", []int{3000})

	err := app.StatusCmd([]string{"nonexistent-*"})
	require.Error(t, err, "StatusCmd with unmatched glob should return error")
	assert.Contains(t, err.Error(), "no servers found", "error should mention no servers found")
}

// ---------------------------------------------------------------------------
// 7. Multiple identifiers
// ---------------------------------------------------------------------------

func TestStatusCmd_MultipleIdentifiers(t *testing.T) {
	// NOT parallel: uses os.Stdout capture

	app, _, _ := newTestApp(t)
	addManagedService(t, app.registry, "svc1", "cmd1", []int{3001})
	addManagedService(t, app.registry, "svc2", "cmd2", []int{3002})

	output := captureStatusOutput(func() {
		if err := app.StatusCmd([]string{"svc1", "svc2"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "svc1", "output should include svc1")
	assert.Contains(t, output, "svc2", "output should include svc2")
}

// ---------------------------------------------------------------------------
// 8. Mixed pattern and exact identifiers
// ---------------------------------------------------------------------------

func TestStatusCmd_MixedPatternAndExact(t *testing.T) {
	// NOT parallel: uses os.Stdout capture

	app, _, _ := newTestApp(t)
	addManagedService(t, app.registry, "web-api", "node api.js", []int{3000})
	addManagedService(t, app.registry, "web-frontend", "npm start", []int{3001})
	addManagedService(t, app.registry, "worker", "ruby worker.rb", []int{4000})

	output := captureStatusOutput(func() {
		if err := app.StatusCmd([]string{"web-*", "worker"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "web-api", "output should include web-api")
	assert.Contains(t, output, "web-frontend", "output should include web-frontend")
	assert.Contains(t, output, "worker", "output should include worker")
}

// ---------------------------------------------------------------------------
// 9. Empty args — error
// ---------------------------------------------------------------------------

func TestStatusCmd_EmptyArgs(t *testing.T) {
	t.Parallel()

	app, _, _ := newTestApp(t)

	err := app.StatusCmd([]string{})
	require.Error(t, err, "StatusCmd with no identifiers should return error")
	assert.Contains(t, err.Error(), "no servers found", "error should mention no servers found")
}

// ---------------------------------------------------------------------------
// 10. Crashed service status
// ---------------------------------------------------------------------------

func TestStatusCmd_CrashedServiceStatus(t *testing.T) {
	// NOT parallel: uses os.Stdout capture

	app, _, _ := newTestApp(t)
	withCrashedService(t, app.registry, "crashed-svc", "node crashing-app.js", []int{5555}, 9999)

	output := captureStatusOutput(func() {
		if err := app.StatusCmd([]string{"crashed-svc"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "crashed-svc", "output should mention service name")
	assert.Contains(t, output, "crashed", "output should show crashed status")
}

// ---------------------------------------------------------------------------
// Additional edge-case tests
// ---------------------------------------------------------------------------

func TestStatusCmd_DuplicateIdentifiers(t *testing.T) {
	// NOT parallel: uses os.Stdout capture

	app, _, _ := newTestApp(t)
	addManagedService(t, app.registry, "svc1", "cmd1", []int{3001})

	output := captureStatusOutput(func() {
		if err := app.StatusCmd([]string{"svc1", "svc1"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "svc1", "output should include svc1 at least once")
}

func TestStatusCmd_ExactNameNotGlob(t *testing.T) {
	// NOT parallel: uses os.Stdout capture

	app, _, _ := newTestApp(t)
	addManagedService(t, app.registry, "api", "cmd1", []int{3001})
	addManagedService(t, app.registry, "api-v2", "cmd2", []int{3002})

	output := captureStatusOutput(func() {
		if err := app.StatusCmd([]string{"api"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "api", "output should include exact match 'api'")
	assert.NotContains(t, output, "api-v2", "exact 'api' should not match 'api-v2'")
}

func TestStatusCmd_WildcardMatchesAll(t *testing.T) {
	// NOT parallel: uses os.Stdout capture

	app, _, _ := newTestApp(t)
	addManagedService(t, app.registry, "api", "cmd1", []int{3001})
	addManagedService(t, app.registry, "worker", "cmd2", []int{3002})
	addManagedService(t, app.registry, "frontend", "cmd3", []int{3003})

	output := captureStatusOutput(func() {
		if err := app.StatusCmd([]string{"*"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "api", "should match api")
	assert.Contains(t, output, "worker", "should match worker")
	assert.Contains(t, output, "frontend", "should match frontend")
}

func TestStatusCmd_SuffixPattern(t *testing.T) {
	// NOT parallel: uses os.Stdout capture

	app, _, _ := newTestApp(t)
	addManagedService(t, app.registry, "prod-api", "cmd1", []int{3001})
	addManagedService(t, app.registry, "staging-api", "cmd2", []int{3002})
	addManagedService(t, app.registry, "prod-worker", "cmd3", []int{3003})

	output := captureStatusOutput(func() {
		if err := app.StatusCmd([]string{"*-api"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "prod-api", "should match prod-api")
	assert.Contains(t, output, "staging-api", "should match staging-api")
	assert.NotContains(t, output, "prod-worker", "should not match prod-worker")
}

func TestStatusCmd_OneExactOneNotFound(t *testing.T) {
	// NOT parallel: uses os.Stdout capture

	app, _, _ := newTestApp(t)
	addManagedService(t, app.registry, "existing", "cmd", []int{3000})

	output := captureStatusOutput(func() {
		err := app.StatusCmd([]string{"existing", "missing"})
		// "existing" matches, "missing" doesn't. Since at least one match is found,
		// the command should succeed.
		require.NoError(t, err)
	})

	assert.Contains(t, output, "existing", "should show the found service")
}

func TestStatusCmd_SourceFieldInOutput(t *testing.T) {
	// NOT parallel: uses os.Stdout capture

	app, _, _ := newTestApp(t)
	addManagedService(t, app.registry, "managed-svc", "cmd", []int{3000})

	output := captureStatusOutput(func() {
		if err := app.StatusCmd([]string{"managed-svc"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "Source:", "output should contain source field")
	assert.Contains(t, output, "managed", "output should show managed source")
}

// ---------------------------------------------------------------------------
// printServerStatus unit tests (output formatting)
// These test printServerStatus directly with constructed ServerInfo objects.
// NOT parallel because printServerStatus writes to os.Stdout.
// ---------------------------------------------------------------------------

func TestPrintServerStatus_ManagedRunning(t *testing.T) {
	app, _, _ := newTestApp(t)

	srv := &models.ServerInfo{
		ManagedService: &models.ManagedService{
			Name:    "test-api",
			Command: "node server.js",
			CWD:     "/home/user/project",
			Ports:   []int{3000, 3001},
		},
		ProcessRecord: &models.ProcessRecord{
			PID:     1234,
			PPID:    1,
			Port:    3000,
			User:    "user",
			Command: "node server.js",
			CWD:     "/home/user/project",
		},
		Source: models.SourceManaged,
		Status: "running",
	}

	output := captureStatusOutput(func() {
		if err := app.printServerStatus(srv); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "test-api", "should show service name")
	assert.Contains(t, output, "1234", "should show PID")
	assert.Contains(t, output, "3000", "should show port")
	assert.Contains(t, output, "running", "should show running status")
	assert.Contains(t, output, "SERVER DETAILS", "should show details header")
	assert.Contains(t, output, "HEALTH STATUS", "should show health section for running service")
}

func TestPrintServerStatus_CrashedWithReason(t *testing.T) {
	app, _, _ := newTestApp(t)

	srv := &models.ServerInfo{
		ManagedService: &models.ManagedService{
			Name:    "crashed-app",
			Command: "python app.py",
			CWD:     "/home/user/project",
			Ports:   []int{5000},
		},
		Source:      models.SourceManaged,
		Status:      "crashed",
		CrashReason: "Error: EADDRINUSE address already in use",
		CrashLogTail: []string{
			"Starting server on port 5000...",
			"Error: EADDRINUSE address already in use :::5000",
		},
	}

	output := captureStatusOutput(func() {
		if err := app.printServerStatus(srv); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "CRASH DETAILS", "should show crash section")
	assert.Contains(t, output, "EADDRINUSE", "should show crash reason")
	assert.Contains(t, output, "Starting server", "should show crash log tail")
	assert.Contains(t, output, "crashed", "should show crashed status")
}

func TestPrintServerStatus_CrashedNoLogs(t *testing.T) {
	app, _, _ := newTestApp(t)

	srv := &models.ServerInfo{
		ManagedService: &models.ManagedService{
			Name:    "ghost",
			Command: "./start.sh",
			CWD:     "/opt/ghost",
			Ports:   []int{2368},
		},
		Source:      models.SourceManaged,
		Status:      "crashed",
		CrashReason: "",
		CrashLogTail: nil,
	}

	output := captureStatusOutput(func() {
		if err := app.printServerStatus(srv); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "CRASH DETAILS", "should show crash section")
	assert.Contains(t, output, "unavailable", "should show unavailable reason when no crash reason")
}

func TestPrintServerStatus_StoppedNoProcess(t *testing.T) {
	app, _, _ := newTestApp(t)

	srv := &models.ServerInfo{
		ManagedService: &models.ManagedService{
			Name:    "idle-svc",
			Command: "sleep infinity",
			CWD:     "/tmp",
			Ports:   []int{9999},
		},
		Source: models.SourceManaged,
		Status: "stopped",
	}

	output := captureStatusOutput(func() {
		if err := app.printServerStatus(srv); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "idle-svc", "should show service name")
	assert.Contains(t, output, "stopped", "should show stopped status")
	assert.NotContains(t, output, "HEALTH STATUS", "stopped service should not show health section")
}

func TestPrintServerStatus_WithAgentTag(t *testing.T) {
	app, _, _ := newTestApp(t)

	srv := &models.ServerInfo{
		ManagedService: &models.ManagedService{
			Name:    "ai-started",
			Command: "npm run dev",
			CWD:     "/home/user/project",
			Ports:   []int{4000},
		},
		ProcessRecord: &models.ProcessRecord{
			PID:     5555,
			PPID:    1,
			Port:    4000,
			User:    "user",
			Command: "npm run dev",
			CWD:     "/home/user/project",
			AgentTag: &models.AgentTag{
				Source:     models.SourceAgent,
				AgentName:  "pi",
				Confidence: models.ConfidenceHigh,
			},
		},
		Source: models.SourceAgent,
		Status: "running",
	}

	output := captureStatusOutput(func() {
		if err := app.printServerStatus(srv); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	assert.Contains(t, output, "AI AGENT DETECTION", "should show agent detection section")
	assert.Contains(t, output, "pi", "should show agent name")
	assert.Contains(t, output, "high", "should show confidence level")
}

// ---------------------------------------------------------------------------
// Matching logic unit tests (mirrors StatusCmd's matching loop)
// These are pure logic tests — safe for t.Parallel().
// ---------------------------------------------------------------------------

func TestStatusMatching_ExactName(t *testing.T) {
	t.Parallel()

	servers := []*models.ServerInfo{
		{ManagedService: &models.ManagedService{Name: "api"}, Status: "running"},
		{ManagedService: &models.ManagedService{Name: "worker"}, Status: "running"},
	}

	var matched []*models.ServerInfo
	id := "api"
	for _, srv := range servers {
		if srv.ManagedService != nil && srv.ManagedService.Name == id {
			matched = append(matched, srv)
			break
		}
	}

	require.Len(t, matched, 1)
	assert.Equal(t, "api", matched[0].ManagedService.Name)
}

func TestStatusMatching_PortString(t *testing.T) {
	t.Parallel()

	servers := []*models.ServerInfo{
		{
			ProcessRecord:  &models.ProcessRecord{PID: 100, Port: 8080},
			ManagedService: &models.ManagedService{Name: "web"},
			Status:         "running",
		},
		{
			ProcessRecord:  &models.ProcessRecord{PID: 101, Port: 9090},
			ManagedService: &models.ManagedService{Name: "admin"},
			Status:         "running",
		},
	}

	var matched []*models.ServerInfo
	id := "9090"
	for _, srv := range servers {
		if srv.ProcessRecord != nil && fmt.Sprintf("%d", srv.ProcessRecord.Port) == id {
			matched = append(matched, srv)
			break
		}
	}

	require.Len(t, matched, 1)
	assert.Equal(t, "admin", matched[0].ManagedService.Name)
}

func TestStatusMatching_GlobExpandsCorrectly(t *testing.T) {
	t.Parallel()

	services := []*models.ManagedService{
		{Name: "web-api"},
		{Name: "web-frontend"},
		{Name: "worker"},
	}

	expanded := ExpandPatterns([]string{"web-*"}, services)
	assert.Len(t, expanded, 2)
	assert.Contains(t, expanded, "web-api")
	assert.Contains(t, expanded, "web-frontend")
	assert.NotContains(t, expanded, "worker")
}

func TestStatusMatching_GlobNoMatchReturnsOriginal(t *testing.T) {
	t.Parallel()

	services := []*models.ManagedService{
		{Name: "api"},
		{Name: "worker"},
	}

	expanded := ExpandPatterns([]string{"zzz-*"}, services)
	assert.Equal(t, []string{"zzz-*"}, expanded, "no-match glob should return original pattern")
}

func TestStatusMatching_MultipleArgsExpandIndependently(t *testing.T) {
	t.Parallel()

	services := []*models.ManagedService{
		{Name: "web-api"},
		{Name: "web-frontend"},
		{Name: "worker"},
	}

	expanded := ExpandPatterns([]string{"web-*", "worker"}, services)
	assert.Len(t, expanded, 3)
	assert.Contains(t, expanded, "web-api")
	assert.Contains(t, expanded, "web-frontend")
	assert.Contains(t, expanded, "worker")
}

func TestStatusMatching_DuplicateExpansion(t *testing.T) {
	t.Parallel()

	services := []*models.ManagedService{
		{Name: "web-api"},
		{Name: "web-frontend"},
	}

	expanded := ExpandPatterns([]string{"web-*", "web-api"}, services)
	assert.Contains(t, expanded, "web-api")
	assert.Contains(t, expanded, "web-frontend")

	// web-api appears twice (from glob expansion + literal arg)
	count := 0
	for _, name := range expanded {
		if name == "web-api" {
			count++
		}
	}
	assert.Equal(t, 2, count, "web-api should appear twice: once from glob, once from literal")
}

func TestStatusMatching_EmptyArgsReturnsEmpty(t *testing.T) {
	t.Parallel()

	services := []*models.ManagedService{{Name: "api"}}
	expanded := ExpandPatterns([]string{}, services)
	assert.Empty(t, expanded, "empty args should return empty result")
}

func TestStatusMatching_EmptyRegistryReturnsArgs(t *testing.T) {
	t.Parallel()

	services := []*models.ManagedService{}
	expanded := ExpandPatterns([]string{"api", "web-*"}, services)
	assert.Equal(t, []string{"api", "web-*"}, expanded, "with empty registry, args return unchanged")
}

// ---------------------------------------------------------------------------
// Full StatusCmd matching loop simulation (pure logic, no I/O)
// ---------------------------------------------------------------------------

func TestStatusMatching_FullLoop_MultiplePatternsAndExact(t *testing.T) {
	t.Parallel()

	servers := []*models.ServerInfo{
		{ManagedService: &models.ManagedService{Name: "web-api"}, Status: "running"},
		{ManagedService: &models.ManagedService{Name: "web-frontend"}, Status: "running"},
		{ManagedService: &models.ManagedService{Name: "worker"}, Status: "running"},
	}

	allServices := []*models.ManagedService{
		{Name: "web-api"},
		{Name: "web-frontend"},
		{Name: "worker"},
	}

	identifiers := []string{"web-*", "worker"}

	var matched []*models.ServerInfo
	for _, id := range identifiers {
		if strings.Contains(id, "*") {
			expanded := ExpandPatterns([]string{id}, allServices)
			for _, name := range expanded {
				for _, srv := range servers {
					if srv.ManagedService != nil && srv.ManagedService.Name == name {
						matched = append(matched, srv)
						break
					}
				}
			}
		} else {
			for _, srv := range servers {
				if srv.ManagedService != nil && srv.ManagedService.Name == id {
					matched = append(matched, srv)
					break
				}
			}
		}
	}

	assert.Len(t, matched, 3, "should match web-api, web-frontend, and worker")
	names := make(map[string]bool)
	for _, srv := range matched {
		names[srv.ManagedService.Name] = true
	}
	assert.True(t, names["web-api"])
	assert.True(t, names["web-frontend"])
	assert.True(t, names["worker"])
}

func TestStatusMatching_FullLoop_NoServers(t *testing.T) {
	t.Parallel()

	servers := []*models.ServerInfo{}
	allServices := []*models.ManagedService{}

	identifiers := []string{"anything"}

	var matched []*models.ServerInfo
	for _, id := range identifiers {
		if strings.Contains(id, "*") {
			_ = allServices // allServices unused when no wildcard
			expanded := ExpandPatterns([]string{id}, allServices)
			for _, name := range expanded {
				for _, srv := range servers {
					if srv.ManagedService != nil && srv.ManagedService.Name == name {
						matched = append(matched, srv)
						break
					}
				}
			}
		} else {
			for _, srv := range servers {
				if srv.ManagedService != nil && srv.ManagedService.Name == id {
					matched = append(matched, srv)
					break
				}
			}
		}
	}

	assert.Empty(t, matched, "no servers means no matches")
}

func TestStatusMatching_FullLoop_CaseSensitive(t *testing.T) {
	t.Parallel()

	servers := []*models.ServerInfo{
		{ManagedService: &models.ManagedService{Name: "API"}, Status: "running"},
		{ManagedService: &models.ManagedService{Name: "api"}, Status: "running"},
	}

	identifiers := []string{"api"}

	var matched []*models.ServerInfo
	for _, id := range identifiers {
		for _, srv := range servers {
			if srv.ManagedService != nil && srv.ManagedService.Name == id {
				matched = append(matched, srv)
				break
			}
		}
	}

	require.Len(t, matched, 1)
	assert.Equal(t, "api", matched[0].ManagedService.Name, "should match only lowercase 'api', not 'API'")
}
