package lifecycle

import (
	"github.com/devports/devpt/pkg/models"
)

// LifecycleManager is the facade that orchestrates lifecycle operations.
// It holds dependencies and delegates to the individual flow functions.
type LifecycleManager struct {
	deps Deps
}

// NewLifecycleManager creates a new LifecycleManager with the given dependencies.
func NewLifecycleManager(deps Deps) *LifecycleManager {
	return &LifecycleManager{deps: deps}
}

// Start executes the start lifecycle command.
func (m *LifecycleManager) Start(svc *models.ManagedService) Result {
	return StartService(m.deps, svc)
}

// Stop executes the stop lifecycle command.
func (m *LifecycleManager) Stop(svc *models.ManagedService) Result {
	return StopService(m.deps, svc)
}

// Restart executes the restart lifecycle command.
func (m *LifecycleManager) Restart(svc *models.ManagedService) Result {
	return RestartService(m.deps, svc)
}
