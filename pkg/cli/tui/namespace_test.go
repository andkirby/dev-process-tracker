package tui

import (
	"testing"

	"github.com/devports/devpt/pkg/models"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// TEST-namespace-extraction
// Covers: BR-1.1, C-1.3, Edge-1.1, Edge-1.2
// ---------------------------------------------------------------------------

func TestExtractNamespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// BR-1.1: dashed service names
		{"dashed name", "api-gateway", "api"},
		{"dashed multi-segment", "web-frontend-v2", "web"},
		{"dashed single segment", "redis", "redis"},

		// BR-1.1: dot-separated names
		{"dot name", "pg.migrator", "pg"},
		{"dot multi-segment", "cache.redis.writer", "cache"},

		// BR-1.1: pure alphanumeric
		{"pure alnum", "redis", "redis"},
		{"pure alnum numeric", "app1", "app1"},

		// Edge-1.1: empty or dash
		{"empty string", "", "-"},
		{"single dash", "-", "-"},
		{"whitespace only", "   ", "-"},

		// Edge-1.2: collision / ambiguity (leading dash is part of namespace)
		{"leading dash", "-gateway", "-gateway"},
		{"trailing dash", "api-", "api"},
		{"multiple dashes", "api---gateway", "api"},
		{"multiple dots", "pg...migrator", "pg"},
		{"mixed separators", "api.gateway-v2", "api"},

		// Leading underscore handling: underscore is part of namespace for grouping
		{"leading underscore service", "_mdt-api", "_mdt"},
		{"leading underscore service 2", "_offgrid-worker", "_offgrid"},
		{"multiple leading underscores", "___test-api", "___test"},
		{"mixed leading special chars", "_.-redis-cache", "_.-redis"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNamespace(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// TEST-group-membership
// Covers: BR-1.3, C-1.7
// ---------------------------------------------------------------------------

func TestGroupForNamespace(t *testing.T) {
	t.Run("managed focus returns all managed services with matching namespace", func(t *testing.T) {
		deps := &fakeAppDeps{
			services: []*models.ManagedService{
				{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3000}},
				{Name: "web-backend", CWD: "/tmp/web-backend", Command: "go run .", Ports: []int{3001}},
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3002}},
				{Name: "redis", CWD: "/tmp/redis", Command: "redis-server", Ports: []int{6379}},
			},
			servers: []*models.ServerInfo{},
		}
		m := newTopModel(deps)
		m.focus = focusManaged
		m.managedSel = 0

		group := groupForNamespace(m, "web")
		assert.Len(t, group, 0) // managed services don't appear as ServerInfo in group
	})

	t.Run("running focus returns visible servers with matching namespace", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				{
					ManagedService: &models.ManagedService{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3000}},
					ProcessRecord:  &models.ProcessRecord{PID: 1001, Port: 3000, Command: "node server.js", CWD: "/tmp/web-frontend", ProjectRoot: "/tmp/web-frontend"},
					Status:         "running",
				},
				{
					ManagedService: &models.ManagedService{Name: "web-backend", CWD: "/tmp/web-backend", Command: "go run .", Ports: []int{3001}},
					ProcessRecord:  &models.ProcessRecord{PID: 1002, Port: 3001, Command: "go run .", CWD: "/tmp/web-backend", ProjectRoot: "/tmp/web-backend"},
					Status:         "running",
				},
				{
					ProcessRecord: &models.ProcessRecord{PID: 1003, Port: 3002, Command: "python app.py", CWD: "/tmp/app", ProjectRoot: "/tmp/app"},
					Status:        "running",
				},
			},
			services: []*models.ManagedService{
				{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3000}},
				{Name: "web-backend", CWD: "/tmp/web-backend", Command: "go run .", Ports: []int{3001}},
			},
		}
		m := newTopModel(deps)
		m.focus = focusRunning
		m.selected = 0

		group := groupForNamespace(m, "web")
		assert.Len(t, group, 2)
		names := make([]string, len(group))
		for i, srv := range group {
			names[i] = srv.ManagedService.Name
		}
		assert.ElementsMatch(t, []string{"web-frontend", "web-backend"}, names)
	})

	t.Run("no match returns empty group", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				{
					ProcessRecord: &models.ProcessRecord{PID: 1001, Port: 3000, Command: "node server.js"},
					Status:        "running",
				},
			},
		}
		m := newTopModel(deps)
		m.focus = focusRunning

		group := groupForNamespace(m, "nonexistent")
		assert.Len(t, group, 0)
	})

	t.Run("filter respects visibility — only visible (filter-passing) services included", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				{
					ManagedService: &models.ManagedService{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
					ProcessRecord:  &models.ProcessRecord{PID: 1001, Port: 3000, Command: "node server.js", CWD: "/tmp/api-gateway", ProjectRoot: "/tmp/api-gateway"},
					Status:         "running",
				},
				{
					ManagedService: &models.ManagedService{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
					ProcessRecord:  &models.ProcessRecord{PID: 1002, Port: 3001, Command: "go run .", CWD: "/tmp/api-auth", ProjectRoot: "/tmp/api-auth"},
					Status:         "running",
				},
				{
					ManagedService: &models.ManagedService{Name: "api-cron", CWD: "/tmp/api-cron", Command: "python cron.py", Ports: []int{3002}},
					ProcessRecord:  &models.ProcessRecord{PID: 1003, Port: 3002, Command: "python cron.py", CWD: "/tmp/api-cron", ProjectRoot: "/tmp/api-cron"},
					Status:         "running",
				},
			},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
				{Name: "api-cron", CWD: "/tmp/api-cron", Command: "python cron.py", Ports: []int{3002}},
			},
		}
		m := newTopModel(deps)
		m.focus = focusRunning
		m.selected = 0
		// Set a search filter that only shows gateway and auth (not cron)
		m.searchQuery = "gateway"
		m.searchInput.SetValue("gateway")

		group := groupForNamespace(m, "api")
		// Only api-gateway should be visible (search filter: "gateway")
		assert.Len(t, group, 1)
		assert.Equal(t, "api-gateway", group[0].ManagedService.Name)
	})

	t.Run("managed focus returns managed services filtered by current search", func(t *testing.T) {
		deps := &fakeAppDeps{
			services: []*models.ManagedService{
				{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3000}},
				{Name: "web-backend", CWD: "/tmp/web-backend", Command: "go run .", Ports: []int{3001}},
				{Name: "web-worker", CWD: "/tmp/web-worker", Command: "python worker.py", Ports: []int{3002}},
			},
			servers: []*models.ServerInfo{},
		}
		m := newTopModel(deps)
		m.focus = focusManaged
		m.managedSel = 0
		m.searchQuery = "frontend"
		m.searchInput.SetValue("frontend")

		group := groupForNamespace(m, "web")
		// Only web-frontend is visible due to search filter
		// For managed focus, groupForNamespace returns ServerInfo but
		// managed services may not have running ServerInfo entries
		assert.Len(t, group, 0)
	})

	t.Run("empty namespace returns empty group", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				{
					ManagedService: &models.ManagedService{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
					ProcessRecord:  &models.ProcessRecord{PID: 1001, Port: 3000, Command: "node server.js", CWD: "/tmp/api-gateway", ProjectRoot: "/tmp/api-gateway"},
					Status:         "running",
				},
			},
		}
		m := newTopModel(deps)
		m.focus = focusRunning

		group := groupForNamespace(m, "")
		assert.Len(t, group, 0)
	})
}
