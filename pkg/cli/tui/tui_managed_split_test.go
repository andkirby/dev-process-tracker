package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/devports/devpt/pkg/models"
	"github.com/stretchr/testify/assert"
)

func managedSplitTestModel() *topModel {
	stoppedAt := time.Date(2026, 3, 27, 21, 54, 25, 0, time.UTC)
	deps := &fakeAppDeps{
		services: []*models.ManagedService{
			{
				Name:     "test-go-basic-fake",
				CWD:      "/Users/kirby/.config/dev-process-tracker/sandbox/servers/go-basic",
				Command:  "go run .",
				Ports:    []int{3401},
				LastStop: &stoppedAt,
			},
			{
				Name:    "docs-preview",
				CWD:     "/tmp/docs-preview",
				Command: "npm run dev",
				Ports:   []int{3001},
			},
		},
		servers: []*models.ServerInfo{
			{
				ManagedService: &models.ManagedService{Name: "test-go-basic-fake", CWD: "/Users/kirby/.config/dev-process-tracker/sandbox/servers/go-basic", Command: "go run .", Ports: []int{3401}},
				Status:         "crashed",
				Source:         models.SourceManaged,
				CrashReason:    "exit status 1",
				CrashLogTail: []string{
					"2026/03/27 21:54:25 [go-basic] listening on http://localhost:3400",
					"2026/03/27 21:54:25 listen tcp :3400: bind: address already in use",
					"exit status 1",
				},
			},
		},
		logPaths: map[string]string{
			"test-go-basic-fake": "~/.config/devpt/logs/test-go-basic-fake/2026-03-12T22-14-37.log",
		},
	}

	model := newTopModel(deps)
	model.width = 120
	model.height = 30
	model.mode = viewModeTable
	model.focus = focusManaged
	model.managedSel = 0
	return model
}

func TestManagedSplitView_SelectedServiceShowsDedicatedDetailsPane(t *testing.T) {
	model := managedSplitTestModel()
	// Services are sorted alphabetically, so test-go-basic-fake is at index 1
	model.managedSel = 1

	output := model.View().Content
	assert.Contains(t, output, "Managed Services")
	assert.Contains(t, output, "Selected service details")
	assert.Contains(t, output, "Headline: exit status 1")
	assert.Contains(t, output, "test-go-basic-fake")
}

func TestManagedSplitView_NoSelectionShowsPlaceholderPane(t *testing.T) {
	model := managedSplitTestModel()
	model.managedSel = -1

	output := model.View().Content
	assert.Contains(t, output, "Selected service details")
	assert.Contains(t, output, "Select a managed service to inspect status")
}

func TestManagedSplitView_StoppedServiceRemainsStopped(t *testing.T) {
	model := managedSplitTestModel()
	model.managedSel = 0

	output := model.View().Content
	assert.Contains(t, output, "docs-preview [stopped]")
	assert.NotContains(t, output, "docs-preview crashed")
}

func TestManagedSplitView_NarrowWidthPreservesPrimarySignals(t *testing.T) {
	model := managedSplitTestModel()
	model.width = 72
	model.managedSel = 1

	output := model.View().Content
	assert.Contains(t, output, "✘")
	assert.Contains(t, output, "exit status 1")
}

func TestManagedSplitView_ServiceMetadataShowsCWDPortsCommand(t *testing.T) {
	model := managedSplitTestModel()
	model.managedSel = 0 // docs-preview (stopped, not crashed)

	output := model.View().Content
	assert.Contains(t, output, "docs-preview")
	assert.Contains(t, output, "/tmp/docs-preview")
	assert.Contains(t, output, "npm run dev")
	assert.Contains(t, output, "3001")
}

func TestManagedSplitView_CrashedServiceShowsMetadataBeforeCrashContext(t *testing.T) {
	model := managedSplitTestModel()
	// Services sorted alphabetically, test-go-basic-fake at index 1
	model.managedSel = 1

	output := model.View().Content

	// Metadata must be visible (may be truncated by fitLine)
	assert.Contains(t, output, "go-basic")
	assert.Contains(t, output, "go run .")
	assert.Contains(t, output, "3401")

	// Crash context must also be visible
	assert.Contains(t, output, "Headline: exit status 1")

	// Verify render order: Dir/Port/Cmd appear before Headline in the output
	stripped := ansi.Strip(output)
	dirPos := strings.Index(stripped, "Dir:")
	headlinePos := strings.Index(stripped, "Headline:")
	assert.Greater(t, headlinePos, dirPos, "crash headline must appear after metadata (Dir)")

	portPos := strings.Index(stripped, "Port:")
	assert.Greater(t, headlinePos, portPos, "crash headline must appear after metadata (Port)")

	cmdPos := strings.Index(stripped, "Cmd:")
	assert.Greater(t, headlinePos, cmdPos, "crash headline must appear after metadata (Cmd)")
}

func TestManagedSplitView_MissingMetadataFieldsNoBlankLines(t *testing.T) {
	deps := &fakeAppDeps{
		services: []*models.ManagedService{
			{
				Name:    "empty-meta-svc",
				CWD:     "",
				Command: "",
				Ports:   []int{},
			},
		},
	}
	model := newTopModel(deps)
	model.width = 120
	model.height = 30
	model.mode = viewModeTable
	model.focus = focusManaged
	model.managedSel = 0

	output := model.View().Content
	stripped := ansi.Strip(output)

	// Service name should be visible
	assert.Contains(t, stripped, "empty-meta-svc")

	// No Dir:/Port:/Cmd: labels should appear for empty fields
	assert.NotContains(t, stripped, "Dir:")
	assert.NotContains(t, stripped, "Port:")
	assert.NotContains(t, stripped, "Cmd:")
}

func TestManagedSplitView_MultiPortMetadataCompact(t *testing.T) {
	deps := &fakeAppDeps{
		services: []*models.ManagedService{
			{
				Name:    "multi-port-svc",
				CWD:     "/app/service",
				Command: "node server.js",
				Ports:   []int{3000, 3001, 3443},
			},
		},
	}
	model := newTopModel(deps)
	model.width = 120
	model.height = 30
	model.mode = viewModeTable
	model.focus = focusManaged
	model.managedSel = 0

	output := model.View().Content
	assert.Contains(t, output, "/app/service")
	assert.Contains(t, output, "node server.js")
	// All ports should be visible somewhere
	assert.Contains(t, output, "3000")
	assert.Contains(t, output, "3001")
	assert.Contains(t, output, "3443")
}

func TestManagedSplitView_SelectedManagedRowHighlightsWholeLine(t *testing.T) {
	model := managedSplitTestModel()
	model.managedSel = 0
	_ = model.View()

	var selectedLine string
	for _, line := range strings.Split(model.table.managedListVP.View(), "\n") {
		if strings.Contains(ansi.Strip(line), "docs-preview [stopped]") {
			selectedLine = line
			break
		}
	}

	assert.NotEmpty(t, selectedLine)
	assert.Contains(t, selectedLine, "48;5;57")
	assert.NotContains(t, selectedLine, "\x1b[m docs-preview")
}
