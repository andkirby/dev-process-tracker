package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/devports/devpt/pkg/health"
	"github.com/devports/devpt/pkg/models"
	"github.com/devports/devpt/pkg/process"
	"github.com/devports/devpt/pkg/registry"
	"github.com/devports/devpt/pkg/scanner"
)

var warnLegacyCommandsOnce sync.Once

// App is the main application handler
type App struct {
	config         models.ConfigPaths
	registry       *registry.Registry
	scanner        *scanner.ProcessScanner
	resolver       *scanner.ProjectResolver
	detector       *scanner.AgentDetector
	processManager *process.Manager
	healthChecker  *health.Checker
}

// NewApp creates and initializes the application
func NewApp() (*App, error) {
	config, err := models.GetConfigPaths()
	if err != nil {
		return nil, fmt.Errorf("failed to get config paths: %w", err)
	}

	if err := config.EnsureDirs(); err != nil {
		return nil, fmt.Errorf("failed to create config directories: %w", err)
	}

	reg := registry.NewRegistry(config.RegistryFile)
	if err := reg.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load registry: %v\n", err)
	}
	warnLegacyCommandsOnce.Do(func() {
		warnLegacyManagedCommands(reg, os.Stderr)
	})

	return &App{
		config:         config,
		registry:       reg,
		scanner:        scanner.NewProcessScanner(),
		resolver:       scanner.NewProjectResolver(),
		detector:       scanner.NewAgentDetector(),
		processManager: process.NewManager(config.LogsDir),
		healthChecker:  health.NewChecker(0),
	}, nil
}

// discoverServers combines scanning and detection into complete server info
func (a *App) discoverServers() ([]*models.ServerInfo, error) {
	processes, err := a.scanner.ScanListeningPorts()
	if err != nil {
		return nil, fmt.Errorf("failed to scan processes: %w", err)
	}

	// Filter to keep only development processes
	commandMap := a.getCommandMap(processes)
	processes = scanner.FilterDevProcesses(processes, commandMap)

	for _, proc := range processes {
		if proc.CWD != "" {
			proc.ProjectRoot = a.resolver.FindProjectRoot(proc.CWD)
		}
		a.detector.EnrichProcessRecord(proc)
	}

	var servers []*models.ServerInfo

	for _, proc := range processes {
		source := models.SourceManual
		if proc.AgentTag != nil {
			source = proc.AgentTag.Source
		}

		servers = append(servers, &models.ServerInfo{
			ProcessRecord: proc,
			Source:        source,
			Status:        "running",
		})
	}

	type managedIdentity struct {
		cwd  string
		root string
	}

	managedServices := a.registry.ListServices()
	portOwners := make(map[int][]*models.ManagedService)
	rootOwners := make(map[string]int)
	cwdOwners := make(map[string]int)
	identities := make(map[*models.ManagedService]managedIdentity, len(managedServices))
	for _, svc := range managedServices {
		svcCWD := normalizePath(svc.CWD)
		svcRoot := normalizePath(a.resolver.FindProjectRoot(svc.CWD))
		identities[svc] = managedIdentity{
			cwd:  svcCWD,
			root: svcRoot,
		}
		if svcCWD != "" {
			cwdOwners[svcCWD]++
		}
		if svcRoot != "" {
			rootOwners[svcRoot]++
		}
		for _, port := range svc.Ports {
			portOwners[port] = append(portOwners[port], svc)
		}
	}
	for _, svc := range managedServices {
		found := false
		identity := identities[svc]
		svcCWD := identity.cwd
		svcRoot := identity.root

		for _, server := range servers {
			if server.ProcessRecord == nil || server.ManagedService != nil {
				continue
			}
			procCWD := normalizePath(server.ProcessRecord.CWD)
			procRoot := normalizePath(server.ProcessRecord.ProjectRoot)
			if canMatchByPath(svcRoot, svcCWD, procRoot, procCWD, rootOwners, cwdOwners) {
				server.ManagedService = svc
				found = true
				break
			}
		}

		if !found && len(svc.Ports) > 0 {
			for _, port := range svc.Ports {
				if owners := portOwners[port]; len(owners) != 1 {
					continue
				}
				for _, server := range servers {
					if server.ProcessRecord != nil && server.ProcessRecord.Port == port && server.ManagedService == nil {
						procCWD := normalizePath(server.ProcessRecord.CWD)
						procRoot := normalizePath(server.ProcessRecord.ProjectRoot)
						if svcRoot != "" && procRoot != "" && svcRoot != procRoot {
							continue
						}
						if svcCWD != "" && procCWD != "" && svcCWD != procCWD {
							continue
						}
						server.ManagedService = svc
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		}

		if !found && svc.LastPID != nil && *svc.LastPID > 0 {
			for _, server := range servers {
				if server.ProcessRecord == nil || server.ManagedService != nil || server.ProcessRecord.PID != *svc.LastPID {
					continue
				}
				procCWD := normalizePath(server.ProcessRecord.CWD)
				procRoot := normalizePath(server.ProcessRecord.ProjectRoot)
				if serviceMatchesProcess(svc, server.ProcessRecord, svcRoot, procRoot, procCWD) {
					server.ManagedService = svc
					found = true
					break
				}
			}
		}

		if !found {
			status := "stopped"
			crashReason := ""
			crashLogTail := []string(nil)
			if svc.LastPID != nil && *svc.LastPID > 0 {
				status = "crashed"
				crashReason, crashLogTail = a.getCrashReport(svc.Name, 12)
			}
			servers = append(servers, &models.ServerInfo{
				ManagedService: svc,
				Source:         models.SourceManaged,
				Status:         status,
				CrashReason:    crashReason,
				CrashLogTail:   crashLogTail,
			})
		}
	}

	return servers, nil
}

func (a *App) getCrashReport(serviceName string, lines int) (string, []string) {
	if lines <= 0 {
		lines = 12
	}
	logLines, err := a.processManager.Tail(serviceName, lines)
	if err != nil {
		return "No logs captured for last run", nil
	}
	reason := inferCrashReason(logLines)
	if reason == "" {
		reason = "Process exited unexpectedly (no explicit error line detected)"
	}
	return reason, logLines
}

func inferCrashReason(lines []string) string {
	keywords := []string{
		"panic",
		"fatal",
		"exception",
		"traceback",
		"error:",
		"eaddrinuse",
		"address already in use",
		"segmentation fault",
		"killed",
		"exit status",
	}

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				return line
			}
		}
	}

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}

	return ""
}

// getCommandMap creates a map of PID to command string
func (a *App) getCommandMap(processes []*models.ProcessRecord) map[int]string {
	cmdMap := make(map[int]string)
	for _, proc := range processes {
		if proc != nil {
			cmdMap[proc.PID] = proc.Command
		}
	}
	return cmdMap
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimRight(p, "/")
	return p
}

func canMatchByPath(svcRoot, svcCWD, procRoot, procCWD string, rootOwners, cwdOwners map[string]int) bool {
	if svcRoot != "" && procRoot != "" && svcRoot == procRoot && rootOwners[svcRoot] == 1 {
		return true
	}
	if svcCWD != "" && procCWD != "" && svcCWD == procCWD && cwdOwners[svcCWD] == 1 {
		return true
	}
	return false
}

func serviceMatchesProcess(svc *models.ManagedService, proc *models.ProcessRecord, svcRoot, procRoot, procCWD string) bool {
	if svc == nil || proc == nil {
		return false
	}

	svcCWD := normalizePath(svc.CWD)
	if svcCWD != "" && procCWD != "" && svcCWD == procCWD {
		return true
	}
	if svcRoot != "" && procRoot != "" && svcRoot == procRoot {
		return true
	}
	for _, port := range svc.Ports {
		if port > 0 && proc.Port == port {
			return true
		}
	}
	return false
}

func warnLegacyManagedCommands(reg *registry.Registry, out io.Writer) {
	if reg == nil || out == nil {
		return
	}
	services := reg.ListServices()
	if len(services) == 0 {
		return
	}

	var warnings []string
	for _, svc := range services {
		if svc == nil {
			continue
		}
		if p, ok := firstBlockedShellPattern(svc.Command); ok {
			warnings = append(warnings, fmt.Sprintf("  - %s (pattern %q)", svc.Name, p))
		}
	}
	if len(warnings) == 0 {
		return
	}
	sort.Strings(warnings)
	fmt.Fprintln(out, "Warning: legacy managed commands detected that rely on shell patterns.")
	fmt.Fprintln(out, "These commands may fail under strict execution. Update them to direct executable form.")
	for _, w := range warnings {
		fmt.Fprintln(out, w)
	}
}
