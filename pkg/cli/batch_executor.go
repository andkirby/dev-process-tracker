package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devports/devpt/pkg/lifecycle"
	"github.com/devports/devpt/pkg/models"
)

// serviceLister provides access to the list of managed services.
type serviceLister interface {
	ListServices() []*models.ManagedService
}

// LifecycleBatchResult holds the outcome of a single lifecycle batch operation.
type LifecycleBatchResult struct {
	Name    string
	Outcome lifecycle.Outcome
	Message string
	PID     int
}

// BatchSummary holds the aggregate summary of a batch operation (contract §7.4).
type BatchSummary struct {
	Total     int
	Succeeded int
	Noop      int
	Blocked   int
	Failed    int
	Invalid   int
	NotFound  int
	Results   []LifecycleBatchResult
}

// RunLifecycleBatch executes a batch operation using the lifecycle manager.
// It processes services in stable order and returns a structured summary.
func RunLifecycleBatch(
	names []string,
	op func(svc *models.ManagedService) lifecycle.Result,
	reg serviceLister,
) BatchSummary {
	summary := BatchSummary{}

	if len(names) == 0 {
		summary.Results = []LifecycleBatchResult{
			{Name: "", Outcome: lifecycle.OutcomeInvalid, Message: "no service names provided"},
		}
		summary.Total = 1
		summary.Invalid = 1
		return summary
	}

	// Expand glob patterns
	services := reg.ListServices()
	expanded := ExpandPatterns(names, services)

	if len(expanded) == 0 {
		summary.Results = []LifecycleBatchResult{
			{Name: "", Outcome: lifecycle.OutcomeNotFound, Message: "no services found matching patterns"},
		}
		summary.Total = 1
		summary.NotFound = 1
		return summary
	}

	// Sort for stable, deterministic order
	sort.Strings(expanded)

	summary.Results = make([]LifecycleBatchResult, 0, len(expanded))
	summary.Total = len(expanded)

	for _, name := range expanded {
		allServices := reg.ListServices()
		svc, errs := LookupServiceWithFallback(name, allServices)
		if svc == nil {
			summary.Results = append(summary.Results, LifecycleBatchResult{
				Name:    name,
				Outcome: lifecycle.OutcomeNotFound,
				Message: fmt.Sprintf("service %q not found: %s", name, joinErrs(errs)),
			})
			summary.NotFound++
			continue
		}

		result := op(svc)
		batchResult := LifecycleBatchResult{
			Name:    name,
			Outcome: result.Outcome,
			Message: result.Message,
			PID:     result.PID,
		}
		summary.Results = append(summary.Results, batchResult)

		switch result.Outcome {
		case lifecycle.OutcomeSuccess:
			summary.Succeeded++
		case lifecycle.OutcomeNoop:
			summary.Noop++
		case lifecycle.OutcomeBlocked:
			summary.Blocked++
		case lifecycle.OutcomeFailed:
			summary.Failed++
		case lifecycle.OutcomeInvalid:
			summary.Invalid++
		case lifecycle.OutcomeNotFound:
			summary.NotFound++
		}
	}

	return summary
}

// FormatBatchSummary formats a BatchSummary as a human-readable string
// following the contract §7.4 summary format.
func FormatBatchSummary(summary BatchSummary) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Matched %d services\n", summary.Total)

	parts := []string{}
	if summary.Succeeded > 0 {
		parts = append(parts, fmt.Sprintf("%d succeeded", summary.Succeeded))
	}
	if summary.Noop > 0 {
		parts = append(parts, fmt.Sprintf("%d noop", summary.Noop))
	}
	if summary.Blocked > 0 {
		parts = append(parts, fmt.Sprintf("%d blocked", summary.Blocked))
	}
	if summary.Failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", summary.Failed))
	}
	if summary.Invalid > 0 {
		parts = append(parts, fmt.Sprintf("%d invalid", summary.Invalid))
	}
	if summary.NotFound > 0 {
		parts = append(parts, fmt.Sprintf("%d not found", summary.NotFound))
	}
	fmt.Fprintln(&sb, strings.Join(parts, ", "))

	// Per-service details
	for _, r := range summary.Results {
		if r.Outcome == lifecycle.OutcomeSuccess {
			action := extractAction(r.Message)
			fmt.Fprintf(&sb, "- %s: %s\n", r.Name, action)
		} else {
			fmt.Fprintf(&sb, "- %s: %s\n", r.Name, r.Message)
		}
	}

	return sb.String()
}

func extractAction(message string) string {
	if idx := strings.Index(message, ": "); idx >= 0 {
		return message[idx+2:]
	}
	return message
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
