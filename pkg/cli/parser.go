package cli

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/devports/devpt/pkg/models"
)

// ParseNamePortIdentifier parses "name:port" format
// Returns (name, port, hasPort) tuple
// Examples:
//   - "web-api:3000" → ("web-api", 3000, true)
//   - "some:thing:1234" → ("some:thing", 1234, true) - last colon is port separator
//   - "web-api" → ("web-api", 0, false)
func ParseNamePortIdentifier(arg string) (name string, port int, hasPort bool) {
	if arg == "" {
		return "", 0, false
	}

	// Regex to find the last colon followed by digits (port)
	// This handles service names with colons in them (e.g., "some:thing")
	// Also handles edge case of just ":port" (empty name)
	re := regexp.MustCompile(`^(.*):(\d+)$`)
	matches := re.FindStringSubmatch(arg)

	if matches == nil {
		return arg, 0, false
	}

	port, err := strconv.Atoi(matches[2])
	if err != nil {
		return arg, 0, false
	}

	return matches[1], port, true
}

// LookupServiceWithFallback tries name+port match, then exact name match
// Returns (service, errorMessages) where errorMessages contains details of failed attempts
// Examples:
//   - "web-api:3000" with web-api on port 3000 → (service, nil)
//   - "some:thing" with service named "some:thing" → (service, nil) - literal name match
//   - "foo:5678" with no matches → (nil, ["tried name=foo port=5678 (not found)", "tried name=foo:5678 (not found)"])
func LookupServiceWithFallback(identifier string, services []*models.ManagedService) (*models.ManagedService, []string) {
	if identifier == "" {
		return nil, []string{"empty identifier"}
	}

	name, port, hasPort := ParseNamePortIdentifier(identifier)
	errors := []string{}

	if hasPort {
		// Try: name + port match
		for _, svc := range services {
			if svc.Name == name {
				for _, p := range svc.Ports {
					if p == port {
						return svc, nil
					}
				}
			}
		}
		errors = append(errors, fmt.Sprintf("tried name=%s port=%d (not found)", name, port))

		// Try: exact name match (for services with colons in literal names)
		for _, svc := range services {
			if svc.Name == identifier {
				return svc, nil
			}
		}
		errors = append(errors, fmt.Sprintf("tried name=%s (not found)", identifier))
		return nil, errors
	}

	// No port: try exact name match only
	for _, svc := range services {
		if svc.Name == identifier {
			return svc, nil
		}
	}
	errors = append(errors, fmt.Sprintf("tried name=%s (not found)", identifier))
	return nil, errors
}
