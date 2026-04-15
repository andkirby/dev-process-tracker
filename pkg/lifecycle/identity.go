package lifecycle

import (
	"strings"

	"github.com/devports/devpt/pkg/models"
)

// IdentityResult holds the result of an identity verification.
type IdentityResult struct {
	Verified bool
	Process  *models.ProcessRecord
	Status   string // "verified", "unknown", "not_found"
}

// ProjectResolver resolves a project root from a CWD path.
// Returns the project root, or empty string if unresolvable.
type ProjectResolver func(cwd string) string

// VerifyIdentity checks whether a live process matches a managed service
// using the ordered evidence chain from the behavioral contract:
//  1. Exact CWD match (unique)
//  2. Exact project root match (unique)
//  3. Declared port owned by exactly one plausible managed service
//  4. Stored PID + matching path evidence
//  5. Command fingerprint (supporting signal only, never sole proof)
func VerifyIdentity(
	svc *models.ManagedService,
	processes []*models.ProcessRecord,
	allServices []*models.ManagedService,
) IdentityResult {
	return VerifyIdentityWithResolver(svc, processes, allServices, nil)
}

// VerifyIdentityWithResolver is like VerifyIdentity but accepts an optional
// project root resolver for more accurate project root matching.
func VerifyIdentityWithResolver(
	svc *models.ManagedService,
	processes []*models.ProcessRecord,
	allServices []*models.ManagedService,
	resolver ProjectResolver,
) IdentityResult {
	if svc == nil {
		return IdentityResult{Status: "not_found"}
	}

	// Precompute per-service identity data across all services
	type svcIdentity struct {
		cwd   string
		root  string
		ports map[int]bool
	}

	resolve := resolver
	if resolve == nil {
		resolve = func(cwd string) string { return cwd }
	}

	identities := make(map[*models.ManagedService]svcIdentity, len(allServices))
	cwdCount := make(map[string]int)
	rootCount := make(map[string]int)
	portCount := make(map[int]int) // how many managed services declare this port

	for _, s := range allServices {
		if s == nil {
			continue
		}
		svcCWD := normalizePath(s.CWD)
		svcRoot := normalizePath(resolve(s.CWD))
		ports := make(map[int]bool, len(s.Ports))
		for _, p := range s.Ports {
			ports[p] = true
		}
		identities[s] = svcIdentity{
			cwd:   svcCWD,
			root:  svcRoot,
			ports: ports,
		}
		if identities[s].cwd != "" {
			cwdCount[identities[s].cwd]++
		}
		if identities[s].root != "" {
			rootCount[identities[s].root]++
		}
		for p := range ports {
			portCount[p]++
		}
	}

	myID := identities[svc]

	// Evidence 1: Exact CWD match (must be unique among managed services)
	if myID.cwd != "" && cwdCount[myID.cwd] == 1 {
		for _, proc := range processes {
			if proc == nil {
				continue
			}
			procCWD := normalizePath(proc.CWD)
			if procCWD != "" && procCWD == myID.cwd {
				return IdentityResult{
					Verified: true,
					Process:  proc,
					Status:   "verified",
				}
			}
		}
	}

	// Evidence 2: Exact project root match (must be unique among managed services)
	if myID.root != "" && rootCount[myID.root] == 1 {
		for _, proc := range processes {
			if proc == nil {
				continue
			}
			procRoot := normalizePath(proc.ProjectRoot)
			if procRoot != "" && procRoot == myID.root {
				return IdentityResult{
					Verified: true,
					Process:  proc,
					Status:   "verified",
				}
			}
		}
	}

	// Evidence 3: Declared port owned by exactly one plausible managed service
	for _, port := range svc.Ports {
		if port <= 0 {
			continue
		}
		if portCount[port] != 1 {
			continue // Not uniquely owned
		}
		for _, proc := range processes {
			if proc == nil || proc.Port != port {
				continue
			}
			// If both service and process have CWD info that conflicts, skip
			procCWD := normalizePath(proc.CWD)
			if myID.cwd != "" && procCWD != "" && myID.cwd != procCWD {
				continue
			}
			// If both have root info that conflicts, skip
			procRoot := normalizePath(proc.ProjectRoot)
			if myID.root != "" && procRoot != "" && myID.root != procRoot {
				continue
			}
			return IdentityResult{
				Verified: true,
				Process:  proc,
				Status:   "verified",
			}
		}
	}

	// Evidence 4: Stored PID + matching path evidence
	if svc.LastPID != nil && *svc.LastPID > 0 {
		for _, proc := range processes {
			if proc == nil || proc.PID != *svc.LastPID {
				continue
			}
			// Need path-based corroboration — CWD or project root must match
			procCWD := normalizePath(proc.CWD)
			procRoot := normalizePath(proc.ProjectRoot)
			if myID.cwd != "" && procCWD != "" && myID.cwd == procCWD {
				return IdentityResult{
					Verified: true,
					Process:  proc,
					Status:   "verified",
				}
			}
			if myID.root != "" && procRoot != "" && myID.root == procRoot {
				return IdentityResult{
					Verified: true,
					Process:  proc,
					Status:   "verified",
				}
			}
			// PID matches but no path evidence — ambiguous, don't verify
			break
		}
	}

	// Evidence 5: Command fingerprint — supporting signal only, never sole proof.
	// We do NOT return verified based on command alone.

	return IdentityResult{
		Verified: false,
		Status:   "not_found",
	}
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimRight(p, "/")
	return p
}

// resolveProjectRoot returns the CWD itself as a simplistic project root.
// In production, this would use scanner.ProjectResolver, but we avoid that
// dependency here to keep the function pure and testable.
func resolveProjectRoot(cwd string) string {
	return cwd
}
