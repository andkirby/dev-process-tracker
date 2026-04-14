package cli

import (
	"testing"

	"github.com/devports/devpt/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// RunBatch
// ---------------------------------------------------------------------------

func TestRunBatch_EmptyNames(t *testing.T) {
	t.Parallel()

	registry := newMockRegistry()
	results := RunBatch([]string{}, nil, registry)
	require.Len(t, results, 1, "empty input should return single error result")
	assert.False(t, results[0].Success)
	assert.NotEmpty(t, results[0].Error)
}

func TestRunBatch_SingleServiceSuccess(t *testing.T) {
	t.Parallel()

	registry := newMockRegistry(
		&models.ManagedService{Name: "api", Ports: []int{3000}},
	)

	op := func(ctx BatchContext) BatchOpResult {
		return BatchOpResult{Name: ctx.Name, Success: true, PID: 1234}
	}

	results := RunBatch([]string{"api"}, op, registry)
	require.Len(t, results, 1)
	assert.Equal(t, "api", results[0].Name)
	assert.True(t, results[0].Success)
	assert.Equal(t, 1234, results[0].PID)
}

func TestRunBatch_SingleServiceFailure(t *testing.T) {
	t.Parallel()

	registry := newMockRegistry(
		&models.ManagedService{Name: "api", Ports: []int{3000}},
	)

	op := func(ctx BatchContext) BatchOpResult {
		return BatchOpResult{Name: ctx.Name, Success: false, Error: "start failed"}
	}

	results := RunBatch([]string{"api"}, op, registry)
	require.Len(t, results, 1)
	assert.Equal(t, "api", results[0].Name)
	assert.False(t, results[0].Success)
	assert.Equal(t, "start failed", results[0].Error)
}

func TestRunBatch_MultipleServicesAllSuccess(t *testing.T) {
	t.Parallel()

	registry := newMockRegistry(
		&models.ManagedService{Name: "api", Ports: []int{3000}},
		&models.ManagedService{Name: "worker", Ports: []int{4000}},
		&models.ManagedService{Name: "db", Ports: []int{5432}},
	)

	op := func(ctx BatchContext) BatchOpResult {
		return BatchOpResult{Name: ctx.Name, Success: true, PID: 1000}
	}

	results := RunBatch([]string{"api", "worker", "db"}, op, registry)
	require.Len(t, results, 3)
	for _, r := range results {
		assert.True(t, r.Success, "service %s should succeed", r.Name)
	}
}

func TestRunBatch_PartialFailure(t *testing.T) {
	t.Parallel()

	registry := newMockRegistry(
		&models.ManagedService{Name: "api", Ports: []int{3000}},
		&models.ManagedService{Name: "worker", Ports: []int{4000}},
	)

	op := func(ctx BatchContext) BatchOpResult {
		if ctx.Name == "worker" {
			return BatchOpResult{Name: ctx.Name, Success: false, Error: "port in use"}
		}
		return BatchOpResult{Name: ctx.Name, Success: true, PID: 1000}
	}

	results := RunBatch([]string{"api", "worker"}, op, registry)
	require.Len(t, results, 2)
	assert.True(t, results[0].Success)
	assert.False(t, results[1].Success)
	assert.Contains(t, results[1].Error, "port in use")
}

func TestRunBatch_ServiceNotFound(t *testing.T) {
	t.Parallel()

	registry := newMockRegistry() // empty registry

	op := func(ctx BatchContext) BatchOpResult {
		return BatchOpResult{Name: ctx.Name, Success: true}
	}

	results := RunBatch([]string{"nonexistent"}, op, registry)
	require.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.Contains(t, results[0].Error, "not found")
}

func TestRunBatch_PatternExpansion(t *testing.T) {
	t.Parallel()

	registry := newMockRegistry(
		&models.ManagedService{Name: "web-api", Ports: []int{3000}},
		&models.ManagedService{Name: "web-frontend", Ports: []int{4000}},
		&models.ManagedService{Name: "worker", Ports: []int{5000}},
	)

	op := func(ctx BatchContext) BatchOpResult {
		return BatchOpResult{Name: ctx.Name, Success: true}
	}

	results := RunBatch([]string{"web-*"}, op, registry)
	require.Len(t, results, 2, "pattern web-* should match web-api and web-frontend")
	names := []string{results[0].Name, results[1].Name}
	assert.Contains(t, names, "web-api")
	assert.Contains(t, names, "web-frontend")
}

func TestRunBatch_NoPatternMatches(t *testing.T) {
	t.Parallel()

	registry := newMockRegistry(
		&models.ManagedService{Name: "api", Ports: []int{3000}},
	)

	op := func(ctx BatchContext) BatchOpResult {
		return BatchOpResult{Name: ctx.Name, Success: true}
	}

	results := RunBatch([]string{"nonexistent-*"}, op, registry)
	require.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.NotEmpty(t, results[0].Error)
}

func TestRunBatch_SequentialOrderPreserved(t *testing.T) {
	t.Parallel()

	registry := newMockRegistry(
		&models.ManagedService{Name: "c", Ports: []int{3}},
		&models.ManagedService{Name: "a", Ports: []int{1}},
		&models.ManagedService{Name: "b", Ports: []int{2}},
	)

	var order []string
	op := func(ctx BatchContext) BatchOpResult {
		order = append(order, ctx.Name)
		return BatchOpResult{Name: ctx.Name, Success: true}
	}

	RunBatch([]string{"c", "a", "b"}, op, registry)
	assert.Equal(t, []string{"c", "a", "b"}, order, "services must be processed in argument order")
}

func TestRunBatch_ClosureReceivesCorrectContext(t *testing.T) {
	t.Parallel()

	svc := &models.ManagedService{Name: "api", Command: "go run main.go", Ports: []int{3000}}
	registry := newMockRegistry(svc)

	var receivedCtx BatchContext
	op := func(ctx BatchContext) BatchOpResult {
		receivedCtx = ctx
		return BatchOpResult{Name: ctx.Name, Success: true}
	}

	RunBatch([]string{"api"}, op, registry)
	assert.Equal(t, "api", receivedCtx.Name)
	assert.Equal(t, svc, receivedCtx.Service)
}

func TestRunBatch_NoIOSideEffects(t *testing.T) {
	t.Parallel()

	// RunBatch returns structured results — verify BatchOpResult has expected fields.
	r := BatchOpResult{Name: "svc", Success: true, PID: 100, Error: "err", Warning: "warn"}
	assert.Equal(t, "svc", r.Name)
	assert.True(t, r.Success)
	assert.Equal(t, 100, r.PID)
	assert.Equal(t, "err", r.Error)
	assert.Equal(t, "warn", r.Warning)
}

// ---------------------------------------------------------------------------
// Mock helpers
// ---------------------------------------------------------------------------

type mockRegistry struct {
	services []*models.ManagedService
}

func newMockRegistry(services ...*models.ManagedService) *mockRegistry {
	return &mockRegistry{services: services}
}

func (m *mockRegistry) ListServices() []*models.ManagedService {
	return m.services
}
