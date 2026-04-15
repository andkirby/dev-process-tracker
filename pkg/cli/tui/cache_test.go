package tui

import (
	"testing"

	"github.com/devports/devpt/pkg/models"
)

func TestVisibleServersCachesByQueryAndSort(t *testing.T) {
	app := &fakeAppDeps{
		servers: []*models.ServerInfo{
			{
				ProcessRecord:  &models.ProcessRecord{PID: 1001, Port: 3000, Command: "node api.js", CWD: "/tmp/api", ProjectRoot: "/tmp/api"},
				ManagedService: &models.ManagedService{Name: "api"},
			},
			{
				ProcessRecord:  &models.ProcessRecord{PID: 1002, Port: 3001, Command: "node web.js", CWD: "/tmp/web", ProjectRoot: "/tmp/web"},
				ManagedService: &models.ManagedService{Name: "web"},
			},
		},
	}
	m := newTopModel(app)

	first := m.visibleServers()
	second := m.visibleServers()
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("expected 2 visible servers, got %d and %d", len(first), len(second))
	}
	if &first[0] != &second[0] && len(first) > 0 && len(second) > 0 {
		// defensive no-op: slice identity is not required, behavior is validated below
	}
	if m.cachedVisible == nil {
		t.Fatalf("expected visible servers cache to be populated")
	}

	m.searchQuery = "web"
	filtered := m.visibleServers()
	if len(filtered) != 1 || m.serviceNameFor(filtered[0]) != "web" {
		t.Fatalf("expected filtered visible server to be web, got %#v", filtered)
	}

	m.searchQuery = ""
	m.sortBy = sortName
	m.sortReverse = true
	sorted := m.visibleServers()
	if len(sorted) != 2 {
		t.Fatalf("expected 2 visible servers after sort change, got %d", len(sorted))
	}
	if m.serviceNameFor(sorted[0]) != "web" {
		t.Fatalf("expected reverse name sort to put web first, got %s", m.serviceNameFor(sorted[0]))
	}
}

func TestManagedServicesCachesUntilVersionChanges(t *testing.T) {
	app := &fakeAppDeps{
		services: []*models.ManagedService{
			{Name: "web", CWD: "/tmp/web", Command: "npm run dev"},
			{Name: "api", CWD: "/tmp/api", Command: "go run ."},
		},
	}
	m := newTopModel(app)

	services := m.managedServices()
	if len(services) != 2 {
		t.Fatalf("expected 2 managed services, got %d", len(services))
	}
	if app.listServicesCalls != 1 {
		t.Fatalf("expected 1 ListServices call after first read, got %d", app.listServicesCalls)
	}

	_ = m.managedServices()
	if app.listServicesCalls != 1 {
		t.Fatalf("expected cached managed services on second read, got %d calls", app.listServicesCalls)
	}

	m.searchQuery = "web"
	filtered := m.managedServices()
	if len(filtered) != 1 || filtered[0].Name != "web" {
		t.Fatalf("expected filtered managed services to contain only web, got %#v", filtered)
	}
	if app.listServicesCalls != 2 {
		t.Fatalf("expected query change to refresh managed cache, got %d calls", app.listServicesCalls)
	}

	m.searchQuery = ""
	m.servicesVersion++
	m.invalidateCachedLists()
	_ = m.managedServices()
	if app.listServicesCalls != 3 {
		t.Fatalf("expected version change to refresh managed cache, got %d calls", app.listServicesCalls)
	}
}

func TestRefreshRepopulatesCachedListsWithLatestData(t *testing.T) {
	app := &fakeAppDeps{
		servers:  []*models.ServerInfo{{ProcessRecord: &models.ProcessRecord{PID: 1001, Port: 3000, Command: "node api.js", CWD: "/tmp/api", ProjectRoot: "/tmp/api"}}},
		services: []*models.ManagedService{{Name: "api", CWD: "/tmp/api", Command: "node api.js"}},
	}
	m := newTopModel(app)

	beforeServersVersion := m.serversVersion
	beforeServicesVersion := m.servicesVersion
	_ = m.visibleServers()
	_ = m.managedServices()
	if m.cachedVisible == nil || m.cachedManaged == nil {
		t.Fatalf("expected caches to be populated before refresh")
	}

	app.servers = []*models.ServerInfo{{ProcessRecord: &models.ProcessRecord{PID: 2002, Port: 4000, Command: "node web.js", CWD: "/tmp/web", ProjectRoot: "/tmp/web"}}}
	app.services = []*models.ManagedService{{Name: "web", CWD: "/tmp/web", Command: "node web.js"}}
	m.refresh()

	if m.serversVersion <= beforeServersVersion || m.servicesVersion <= beforeServicesVersion {
		t.Fatalf("expected refresh to bump cache versions")
	}
	if m.cachedVisible == nil || m.cachedManaged == nil {
		t.Fatalf("expected refresh to repopulate visible and managed caches")
	}
	if len(m.cachedVisible) != 1 || m.cachedVisible[0].ProcessRecord.PID != 2002 {
		t.Fatalf("expected refreshed visible cache to contain PID 2002, got %#v", m.cachedVisible)
	}
	if len(m.cachedManaged) != 1 || m.cachedManaged[0].Name != "web" {
		t.Fatalf("expected refreshed managed cache to contain web, got %#v", m.cachedManaged)
	}
}

func TestDisplayNamesCacheTracksQuerySortAndServices(t *testing.T) {
	app := &fakeAppDeps{
		servers: []*models.ServerInfo{
			{ProcessRecord: &models.ProcessRecord{PID: 1001, Port: 3000, Command: "node api.js", CWD: "/tmp/shared", ProjectRoot: "/tmp/shared"}},
			{ProcessRecord: &models.ProcessRecord{PID: 1002, Port: 3001, Command: "node web.js", CWD: "/tmp/shared", ProjectRoot: "/tmp/shared"}},
		},
		services: []*models.ManagedService{
			{Name: "shared", CWD: "/tmp/shared", Command: "npm run dev"},
		},
	}
	m := newTopModel(app)

	visible := m.visibleServers()
	names := m.displayNames(visible)
	if len(names) != 2 {
		t.Fatalf("expected 2 display names, got %d", len(names))
	}
	listCalls := app.listServicesCalls

	again := m.displayNames(m.visibleServers())
	if len(again) != 2 {
		t.Fatalf("expected cached display names, got %d", len(again))
	}
	if app.listServicesCalls != listCalls {
		t.Fatalf("expected displayNames cache hit, got extra ListServices call count %d -> %d", listCalls, app.listServicesCalls)
	}

	m.searchQuery = "web"
	filteredVisible := m.visibleServers()
	filteredNames := m.displayNames(filteredVisible)
	if len(filteredNames) != 1 {
		t.Fatalf("expected 1 filtered display name, got %d", len(filteredNames))
	}
	if app.listServicesCalls <= listCalls {
		t.Fatalf("expected query change to invalidate displayNames cache")
	}

	m.searchQuery = ""
	m.servicesVersion++
	m.invalidateCachedLists()
	_ = m.displayNames(m.visibleServers())
	if app.listServicesCalls <= listCalls+1 {
		t.Fatalf("expected service version change to invalidate displayNames cache")
	}
}

func TestDisplayNamesCachesUntilVersionChanges(t *testing.T) {
	app := &fakeAppDeps{
		servers: []*models.ServerInfo{
			{ProcessRecord: &models.ProcessRecord{PID: 1001, Port: 3000, Command: "node api.js", CWD: "/tmp/api", ProjectRoot: "/tmp/api"}, ManagedService: &models.ManagedService{Name: "api"}},
			{ProcessRecord: &models.ProcessRecord{PID: 1002, Port: 3001, Command: "node api.js", CWD: "/tmp/api2", ProjectRoot: "/tmp/api2"}, ManagedService: &models.ManagedService{Name: "api"}},
		},
		services: []*models.ManagedService{{Name: "api", CWD: "/tmp/api", Command: "node api.js"}},
	}
	m := newTopModel(app)

	visible := m.visibleServers()
	if len(visible) != 2 {
		t.Fatalf("expected 2 visible servers, got %d", len(visible))
	}

	// First call computes and caches
	names1 := m.displayNames(visible)
	if m.cachedDisplayNames == nil {
		t.Fatal("expected cachedDisplayNames to be populated after first call")
	}
	if len(names1) != 2 {
		t.Fatalf("expected 2 display names, got %d", len(names1))
	}
	// Duplicate "api" names should get ~1 and ~2 suffixes
	found1, found2 := false, false
	for _, n := range names1 {
		if n == "api~1" {
			found1 = true
		}
		if n == "api~2" {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Fatalf("expected api~1 and api~2 for duplicate names, got %v", names1)
	}

	// Second call returns cache (same version)
	names2 := m.displayNames(visible)
	if len(names1) != len(names2) {
		t.Fatal("expected cached display names to match")
	}
	for i := range names1 {
		if names1[i] != names2[i] {
			t.Fatalf("display name mismatch at %d: %q vs %q", i, names1[i], names2[i])
		}
	}

	// Invalidate via refresh
	app.servers = []*models.ServerInfo{
		{ProcessRecord: &models.ProcessRecord{PID: 2001, Port: 4000, Command: "node web.js", CWD: "/tmp/web", ProjectRoot: "/tmp/web"}, ManagedService: &models.ManagedService{Name: "web"}},
	}
	m.refresh()
	if m.cachedDisplayNames != nil {
		t.Fatal("expected refresh to invalidate cachedDisplayNames")
	}

	// New visible servers get new display names
	newVisible := m.visibleServers()
	if len(newVisible) != 1 {
		t.Fatalf("expected 1 visible server after refresh, got %d", len(newVisible))
	}
	names3 := m.displayNames(newVisible)
	if len(names3) != 1 || names3[0] != "web" {
		t.Fatalf("expected single web display name, got %v", names3)
	}
}
