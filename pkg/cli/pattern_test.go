package cli

import (
	"strings"
	"testing"

	"github.com/devports/devpt/pkg/models"
	"github.com/stretchr/testify/assert"
)

// TestExpandPatterns_NoPattern returns literal arguments unchanged
func TestExpandPatterns_NoPattern(t *testing.T) {
	services := []*models.ManagedService{
		{Name: "api"},
		{Name: "worker"},
		{Name: "frontend"},
	}

	args := []string{"api", "worker"}
	result := ExpandPatterns(args, services)

	assert.Equal(t, []string{"api", "worker"}, result, "Literal service names should pass through unchanged")
}

// TestExpandPatterns_SingleWildcard matches prefix pattern
func TestExpandPatterns_SingleWildcard(t *testing.T) {
	services := []*models.ManagedService{
		{Name: "web-api"},
		{Name: "web-frontend"},
		{Name: "worker"},
	}

	args := []string{"web-*"}
	result := ExpandPatterns(args, services)

	// Should match web-api and web-frontend
	assert.Len(t, result, 2, "Pattern 'web-*' should match 2 services")
	assert.Contains(t, result, "web-api", "Should match web-api")
	assert.Contains(t, result, "web-frontend", "Should match web-frontend")
	assert.NotContains(t, result, "worker", "Should not match worker")
}

// TestExpandPatterns_SuffixWildcard matches suffix pattern
func TestExpandPatterns_SuffixWildcard(t *testing.T) {
	services := []*models.ManagedService{
		{Name: "frontend-api"},
		{Name: "backend-api"},
		{Name: "api-gateway"},
	}

	args := []string{"*-api"}
	result := ExpandPatterns(args, services)

	assert.Len(t, result, 2, "Pattern '*-api' should match 2 services")
	assert.Contains(t, result, "frontend-api", "Should match frontend-api")
	assert.Contains(t, result, "backend-api", "Should match backend-api")
	assert.NotContains(t, result, "api-gateway", "Should not match api-gateway")
}

// TestExpandPatterns_ContainsWildcard matches anywhere in string
func TestExpandPatterns_ContainsWildcard(t *testing.T) {
	services := []*models.ManagedService{
		{Name: "frontend-api"},
		{Name: "backend-api"},
		{Name: "api-gateway"},
	}

	args := []string{"*api*"}
	result := ExpandPatterns(args, services)

	assert.Len(t, result, 3, "Pattern '*api*' should match all 3 services")
	assert.Contains(t, result, "frontend-api", "Should match frontend-api")
	assert.Contains(t, result, "backend-api", "Should match backend-api")
	assert.Contains(t, result, "api-gateway", "Should match api-gateway")
}

// TestExpandPatterns_WildcardMatchesAll matches everything
func TestExpandPatterns_WildcardMatchesAll(t *testing.T) {
	services := []*models.ManagedService{
		{Name: "api"},
		{Name: "worker"},
		{Name: "frontend"},
	}

	args := []string{"*"}
	result := ExpandPatterns(args, services)

	assert.Len(t, result, 3, "Pattern '*' should match all services")
	assert.Contains(t, result, "api")
	assert.Contains(t, result, "worker")
	assert.Contains(t, result, "frontend")
}

// TestExpandPatterns_NoMatches returns original pattern for error handling
func TestExpandPatterns_NoMatches(t *testing.T) {
	services := []*models.ManagedService{
		{Name: "api"},
		{Name: "worker"},
	}

	args := []string{"nonexistent-*"}
	result := ExpandPatterns(args, services)

	// Pattern with no matches should return original for error detection
	assert.Equal(t, []string{"nonexistent-*"}, result, "Pattern with no matches should return original")
}

// TestExpandPatterns_CombinedPatternsAndLiteral expands patterns then combines with literals
func TestExpandPatterns_CombinedPatternsAndLiteral(t *testing.T) {
	services := []*models.ManagedService{
		{Name: "web-api"},
		{Name: "web-frontend"},
		{Name: "worker"},
		{Name: "database"},
	}

	args := []string{"web-*", "worker", "database"}
	result := ExpandPatterns(args, services)

	assert.Len(t, result, 4, "Should combine pattern matches with literal names")
	assert.Contains(t, result, "web-api")
	assert.Contains(t, result, "web-frontend")
	assert.Contains(t, result, "worker")
	assert.Contains(t, result, "database")
}

// TestExpandPatterns_EmptyArgs returns empty result
func TestExpandPatterns_EmptyArgs(t *testing.T) {
	services := []*models.ManagedService{
		{Name: "api"},
	}

	args := []string{}
	result := ExpandPatterns(args, services)

	assert.Empty(t, result, "Empty args should return empty result")
}

// TestExpandPatterns_MultiplePatterns each expands independently
func TestExpandPatterns_MultiplePatterns(t *testing.T) {
	services := []*models.ManagedService{
		{Name: "web-api"},
		{Name: "web-frontend"},
		{Name: "worker-api"},
		{Name: "database"},
	}

	args := []string{"web-*", "*-api"}
	result := ExpandPatterns(args, services)

	// Should have: web-api, web-frontend (from web-*) and web-api, worker-api (from *-api)
	// Duplicates should be preserved for now (order matters for batch execution)
	assert.Contains(t, result, "web-api")
	assert.Contains(t, result, "web-frontend")
	assert.Contains(t, result, "worker-api")
}

// TestExpandPatterns_PreservesOrder maintains argument order
func TestExpandPatterns_PreservesOrder(t *testing.T) {
	services := []*models.ManagedService{
		{Name: "a-service"},
		{Name: "b-service"},
		{Name: "c-service"},
	}

	args := []string{"b-*", "a-*", "c-*"}
	result := ExpandPatterns(args, services)

	// Order should be: b matches first, then a matches, then c matches
	firstB := -1
	firstA := -1
	firstC := -1

	for i, name := range result {
		if strings.HasPrefix(name, "b") && firstB == -1 {
			firstB = i
		}
		if strings.HasPrefix(name, "a") && firstA == -1 {
			firstA = i
		}
		if strings.HasPrefix(name, "c") && firstC == -1 {
			firstC = i
		}
	}

	assert.Less(t, firstB, firstA, "b-service should appear before a-service")
	assert.Less(t, firstA, firstC, "a-service should appear before c-service")
}

// TestExpandPatterns_EmptyRegistry returns patterns unchanged when no services exist
func TestExpandPatterns_EmptyRegistry(t *testing.T) {
	services := []*models.ManagedService{}

	args := []string{"api", "web-*"}
	result := ExpandPatterns(args, services)

	assert.Equal(t, []string{"api", "web-*"}, result, "With empty registry, patterns should return unchanged")
}

// TestExpandPatterns_DuplicateArgs preserves duplicates
func TestExpandPatterns_DuplicateArgs(t *testing.T) {
	services := []*models.ManagedService{
		{Name: "api"},
	}

	args := []string{"api", "api"}
	result := ExpandPatterns(args, services)

	assert.Equal(t, []string{"api", "api"}, result, "Duplicate arguments should be preserved")
}

// TestExpandPatterns_CaseSensitive performs case-sensitive matching
func TestExpandPatterns_CaseSensitive(t *testing.T) {
	services := []*models.ManagedService{
		{Name: "API"},
		{Name: "api"},
		{Name: "Api"},
	}

	args := []string{"API"}
	result := ExpandPatterns(args, services)

	assert.Len(t, result, 1, "Should match exact case only")
	assert.Equal(t, "API", result[0], "Should match only API (uppercase)")
}
