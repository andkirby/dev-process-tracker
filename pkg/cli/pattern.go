package cli

import (
	"path/filepath"
	"strings"

	"github.com/devports/devpt/pkg/models"
)

// ExpandPatterns expands glob patterns against service names.
// Only supports '*' wildcard (no regex or tag patterns).
// Returns patterns with no matches unchanged for error detection.
// Preserves argument order and duplicates.
func ExpandPatterns(args []string, services []*models.ManagedService) []string {
	if len(args) == 0 {
		return []string{}
	}

	// Build a set of all service names for quick lookup
	serviceNames := make(map[string]bool)
	for _, svc := range services {
		serviceNames[svc.Name] = true
	}

	var result []string

	for _, arg := range args {
		// If no wildcard, treat as literal
		if !strings.Contains(arg, "*") {
			result = append(result, arg)
			continue
		}

		// Expand pattern
		matches := expandPattern(arg, serviceNames)
		if len(matches) == 0 {
			// No matches: return original pattern for error detection
			result = append(result, arg)
		} else {
			// Add all matches in sorted order for consistency
			result = append(result, matches...)
		}
	}

	return result
}

// expandPattern expands a single glob pattern against service names.
// Returns sorted matches for consistent ordering within a pattern.
func expandPattern(pattern string, serviceNames map[string]bool) []string {
	var matches []string

	for name := range serviceNames {
		matched, err := filepath.Match(pattern, name)
		if err != nil {
			// Invalid pattern: treat as no match
			continue
		}
		if matched {
			matches = append(matches, name)
		}
	}

	// Sort matches for consistent ordering
	// Use simple bubble sort for small lists (most registries have < 100 services)
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[i] > matches[j] {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	return matches
}
