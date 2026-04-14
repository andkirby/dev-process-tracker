package cli

import (
	"fmt"

	"github.com/devports/devpt/pkg/models"
)

// serviceLister provides access to the list of managed services.
type serviceLister interface {
	ListServices() []*models.ManagedService
}

// BatchOpResult holds the outcome of a single batch operation.
type BatchOpResult struct {
	Name    string
	Success bool
	PID     int
	Error   string
	Warning string
}

// BatchContext provides per-service context to a BatchOp closure.
type BatchContext struct {
	Name     string
	Service  *models.ManagedService
	Registry serviceLister
}

// BatchOp is a callback that processes a single service within a batch.
type BatchOp func(ctx BatchContext) BatchOpResult

// RunBatch executes a batch operation over named services.
// It expands glob patterns, resolves each name to a service, and invokes op
// sequentially. It returns structured results with no IO side-effects.
func RunBatch(names []string, op BatchOp, reg serviceLister) []BatchOpResult {
	// Empty-input guard
	if len(names) == 0 {
		return []BatchOpResult{
			{Name: "", Success: false, Error: "no service names provided"},
		}
	}

	// Expand glob patterns
	services := reg.ListServices()
	expanded := ExpandPatterns(names, services)

	if len(expanded) == 0 {
		return []BatchOpResult{
			{Name: "", Success: false, Error: "no services found matching patterns"},
		}
	}

	results := make([]BatchOpResult, 0, len(expanded))

	for _, name := range expanded {
		allServices := reg.ListServices()
		svc, errs := LookupServiceWithFallback(name, allServices)
		if svc == nil {
			results = append(results, BatchOpResult{
				Name:    name,
				Success: false,
				Error:   fmt.Sprintf("service %q not found: %s", name, joinErrs(errs)),
			})
			continue
		}

		result := op(BatchContext{
			Name:     name,
			Service:  svc,
			Registry: reg,
		})
		results = append(results, result)
	}

	return results
}

func joinErrs(errs []string) string {
	joined := ""
	for i, e := range errs {
		if i > 0 {
			joined += "; "
		}
		joined += e
	}
	return joined
}
