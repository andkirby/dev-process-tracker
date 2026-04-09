package tui

import (
	"fmt"
	"regexp"

	"github.com/devports/devpt/pkg/models"
)

// namespaceRegex matches: leading non-alphanumeric chars + first alphanumeric sequence
// Examples: "_offgrid-be" matches "_offgrid", "api-gateway" matches "api"
var namespaceRegex = regexp.MustCompile(`^([^a-zA-Z0-9]*[a-zA-Z0-9]+)[^a-zA-Z0-9]`)

// extractNamespace returns the namespace prefix of a service name,
// including any leading special characters (e.g., _). The namespace is
// everything from start up to the first separator (non-alphanumeric)
// after the first alphanumeric character.
// Examples:
//   "_offgrid-api" → "_offgrid"
//   "offgrid-be" → "offgrid"
//   "api-gateway" → "api"
// Returns "-" for empty, whitespace-only, or strings with no alphanumeric characters.
func extractNamespace(name string) string {
	if name == "" {
		return "-"
	}
	// Try to match the pattern: [leading specials][alphanumerics][separator]
	matches := namespaceRegex.FindStringSubmatch(name)
	if len(matches) < 2 {
		// No separator found, check if string has any alphanumerics at all
		for _, r := range name {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				return name // Entire string is namespace
			}
		}
		return "-" // No alphanumeric characters
	}
	return matches[1]
}

// groupForNamespace returns all visible servers matching the given namespace prefix.
// The function uses the current focus and search filter to determine visibility:
// - In focusRunning: returns visible servers whose service name shares the namespace.
// - In focusManaged: returns visible servers for managed services matching the namespace.
func groupForNamespace(m *topModel, namespace string) []*models.ServerInfo {
	if namespace == "" || namespace == "-" {
		return nil
	}

	var group []*models.ServerInfo

	switch m.focus {
	case focusRunning:
		for _, srv := range m.visibleServers() {
			if srv == nil || srv.ProcessRecord == nil {
				continue
			}
			name := m.serviceNameFor(srv)
			if extractNamespace(name) == namespace {
				group = append(group, srv)
			}
		}
	case focusManaged:
		// For managed focus, we return running ServerInfo entries that
		// correspond to managed services matching the namespace and visible
		// under the current search filter.
		managed := m.managedServices()
		managedSet := make(map[string]bool)
		for _, svc := range managed {
			if extractNamespace(svc.Name) == namespace {
				managedSet[svc.Name] = true
			}
		}
		for _, srv := range m.visibleServers() {
			if srv == nil || srv.ManagedService == nil {
				continue
			}
			if managedSet[srv.ManagedService.Name] {
				group = append(group, srv)
			}
		}
	}

	return group
}

// namespaceOfSelected returns the namespace of the currently selected service.
func namespaceOfSelected(m *topModel) string {
	switch m.focus {
	case focusRunning:
		visible := m.visibleServers()
		if m.selected < 0 || m.selected >= len(visible) {
			return "-"
		}
		srv := visible[m.selected]
		name := m.serviceNameFor(srv)
		return extractNamespace(name)
	case focusManaged:
		managed := m.managedServices()
		if m.managedSel < 0 || m.managedSel >= len(managed) {
			return "-"
		}
		return extractNamespace(managed[m.managedSel].Name)
	default:
		return "-"
	}
}

// groupServiceNames extracts service names from a group of ServerInfo.
func groupServiceNames(group []*models.ServerInfo) []string {
	names := make([]string, 0, len(group))
	for _, srv := range group {
		if srv != nil && srv.ManagedService != nil {
			names = append(names, srv.ManagedService.Name)
		} else if srv != nil && srv.ProcessRecord != nil {
			names = append(names, fmt.Sprintf("pid:%d", srv.ProcessRecord.PID))
		}
	}
	return names
}

// groupPIDs extracts PIDs from a group of ServerInfo.
func groupPIDs(group []*models.ServerInfo) []int {
	pids := make([]int, 0, len(group))
	for _, srv := range group {
		if srv != nil && srv.ProcessRecord != nil && srv.ProcessRecord.PID > 0 {
			pids = append(pids, srv.ProcessRecord.PID)
		}
	}
	return pids
}
