package cli

import (
	"testing"

	"github.com/devports/devpt/pkg/lifecycle"
	"github.com/devports/devpt/pkg/models"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// RunLifecycleBatch
// ---------------------------------------------------------------------------

func TestRunLifecycleBatch_EmptyInput(t *testing.T) {
	t.Parallel()

	registry := newMockRegistry()
	summary := RunLifecycleBatch([]string{}, func(svc *models.ManagedService) lifecycle.Result {
		return lifecycle.Result{Outcome: lifecycle.OutcomeSuccess}
	}, registry)

	assert.Equal(t, 1, summary.Total)
	assert.Equal(t, 1, summary.Invalid)
}

func TestRunLifecycleBatch_AllSuccess(t *testing.T) {
	t.Parallel()

	registry := newMockRegistry(
		&models.ManagedService{Name: "api", Ports: []int{3000}},
		&models.ManagedService{Name: "worker", Ports: []int{4000}},
	)

	summary := RunLifecycleBatch([]string{"api", "worker"}, func(svc *models.ManagedService) lifecycle.Result {
		return lifecycle.Result{Outcome: lifecycle.OutcomeSuccess, Message: "started", PID: 1234}
	}, registry)

	assert.Equal(t, 2, summary.Total)
	assert.Equal(t, 2, summary.Succeeded)
}

func TestRunLifecycleBatch_MixedOutcomes(t *testing.T) {
	t.Parallel()

	registry := newMockRegistry(
		&models.ManagedService{Name: "api", Ports: []int{3000}},
		&models.ManagedService{Name: "worker", Ports: []int{4000}},
		&models.ManagedService{Name: "web", Ports: []int{5000}},
	)

	i := 0
	outcomes := []lifecycle.Outcome{lifecycle.OutcomeSuccess, lifecycle.OutcomeNoop, lifecycle.OutcomeBlocked}
	summary := RunLifecycleBatch([]string{"api", "worker", "web"}, func(svc *models.ManagedService) lifecycle.Result {
		outcome := outcomes[i]
		i++
		return lifecycle.Result{Outcome: outcome, Message: string(outcome)}
	}, registry)

	assert.Equal(t, 3, summary.Total)
	assert.Equal(t, 1, summary.Succeeded)
	assert.Equal(t, 1, summary.Noop)
	assert.Equal(t, 1, summary.Blocked)
}

func TestRunLifecycleBatch_NotFound(t *testing.T) {
	t.Parallel()

	registry := newMockRegistry()
	summary := RunLifecycleBatch([]string{"nonexistent"}, func(svc *models.ManagedService) lifecycle.Result {
		return lifecycle.Result{Outcome: lifecycle.OutcomeSuccess}
	}, registry)

	assert.Equal(t, 1, summary.Total)
	assert.Equal(t, 1, summary.NotFound)
}

func TestRunLifecycleBatch_StableOrder(t *testing.T) {
	t.Parallel()

	registry := newMockRegistry(
		&models.ManagedService{Name: "c", Ports: []int{3}},
		&models.ManagedService{Name: "a", Ports: []int{1}},
		&models.ManagedService{Name: "b", Ports: []int{2}},
	)

	summary := RunLifecycleBatch([]string{"c", "a", "b"}, func(svc *models.ManagedService) lifecycle.Result {
		return lifecycle.Result{Outcome: lifecycle.OutcomeSuccess, Message: "ok"}
	}, registry)

	names := make([]string, len(summary.Results))
	for i, r := range summary.Results {
		names[i] = r.Name
	}
	assert.Equal(t, []string{"a", "b", "c"}, names, "lifecycle batch should process in sorted order")
}

func TestFormatBatchSummary(t *testing.T) {
	t.Parallel()

	summary := BatchSummary{
		Total:     4,
		Succeeded: 2,
		Noop:      1,
		Blocked:   1,
		Results: []LifecycleBatchResult{
			{Name: "api", Outcome: lifecycle.OutcomeSuccess, Message: "Success: started"},
			{Name: "worker", Outcome: lifecycle.OutcomeSuccess, Message: "Success: started"},
			{Name: "web", Outcome: lifecycle.OutcomeNoop, Message: "No-op: already running"},
			{Name: "redis", Outcome: lifecycle.OutcomeBlocked, Message: "Blocked: port 6379 is in use"},
		},
	}

	formatted := FormatBatchSummary(summary)
	assert.Contains(t, formatted, "Matched 4 services")
	assert.Contains(t, formatted, "2 succeeded")
	assert.Contains(t, formatted, "1 noop")
	assert.Contains(t, formatted, "1 blocked")
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
