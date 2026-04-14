package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/devports/devpt/pkg/health"
	"github.com/devports/devpt/pkg/models"
	"github.com/devports/devpt/pkg/process"
)

// ListCmd handles the 'ls' command
func (a *App) ListCmd(detailed bool) error {
	servers, err := a.discoverServers()
	if err != nil {
		return err
	}

	return PrintServerTable(a.outWriter(), servers, detailed)
}

// AddCmd registers a new managed service
func (a *App) AddCmd(name, cwd, command string, ports []int) error {
	if err := validateManagedCommand(command); err != nil {
		return err
	}

	svc := &models.ManagedService{
		Name:    name,
		CWD:     cwd,
		Command: command,
		Ports:   ports,
	}

	if err := a.registry.AddService(svc); err != nil {
		return err
	}

	fmt.Fprintf(a.outWriter(), "Service %q registered successfully\n", name)
	return nil
}

// RemoveCmd removes a managed service
func (a *App) RemoveCmd(name string) error {
	return a.registry.RemoveService(name)
}

// StartCmd starts a managed service
func (a *App) StartCmd(name string) error {
	// Supports name:port format for disambiguation
	allServices := a.registry.ListServices()
	svc, errs := LookupServiceWithFallback(name, allServices)
	if svc == nil {
		return fmt.Errorf("service %q not found: %s", name, strings.Join(errs, "; "))
	}

	fmt.Fprintf(a.outWriter(), "Starting %q...\n", svc.Name)
	pid, err := a.processManager.Start(svc)
	if err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Update registry with new PID
	if err := a.registry.UpdateServicePID(svc.Name, pid); err != nil {
		fmt.Fprintf(a.errWriter(), "Warning: failed to update registry: %v\n", err)
	}

	fmt.Fprintf(a.outWriter(), "Started %q\n", svc.Name)
	return nil
}

// StopCmd stops a service by name or port
func (a *App) StopCmd(identifier string) error {
	var targetPID int
	targetServiceName := ""

	// Check if identifier is a service name
	if svc, _ := LookupServiceWithFallback(identifier, a.registry.ListServices()); svc != nil {
		targetServiceName = svc.Name
		pid, err := a.validatedManagedPID(svc)
		if err != nil {
			return err
		}
		targetPID = pid
	} else {
		// Try parsing as port number
		port, err := strconv.Atoi(identifier)
		if err != nil {
			return fmt.Errorf("invalid service name or port: %s", identifier)
		}

		// Find process by port
		servers, err := a.discoverServers()
		if err != nil {
			return err
		}

		for _, srv := range servers {
			if srv.ProcessRecord != nil && srv.ProcessRecord.Port == port {
				targetPID = srv.ProcessRecord.PID
				if srv.ManagedService != nil {
					targetServiceName = srv.ManagedService.Name
				}
				break
			}
		}

		if targetPID == 0 {
			return fmt.Errorf("no process found on port %d", port)
		}
	}

	if targetPID == 0 {
		return fmt.Errorf("cannot determine PID to stop")
	}

	// Stop the process
	fmt.Fprintf(a.outWriter(), "Stopping PID %d...\n", targetPID)
	if err := a.processManager.Stop(targetPID, 5000000000); err != nil { // 5 second timeout
		if errors.Is(err, process.ErrNeedSudo) {
			return fmt.Errorf("requires sudo to terminate PID %d", targetPID)
		}
		if isProcessFinishedErr(err) {
			if targetServiceName != "" {
				if clrErr := a.registry.ClearServicePID(targetServiceName); clrErr != nil {
					fmt.Fprintf(a.errWriter(), "Warning: failed to clear PID for %q: %v\n", targetServiceName, clrErr)
				}
			}
			return nil
		}
		return fmt.Errorf("failed to stop process: %w", err)
	}

	fmt.Fprintf(a.outWriter(), "Process %d stopped\n", targetPID)
	if targetServiceName != "" {
		if err := a.registry.ClearServicePID(targetServiceName); err != nil {
			fmt.Fprintf(a.errWriter(), "Warning: failed to clear PID for %q: %v\n", targetServiceName, err)
		}
	}
	return nil
}

// RestartCmd restarts a managed service
func (a *App) RestartCmd(name string) error {
	// Supports name:port format for disambiguation
	allServices := a.registry.ListServices()
	svc, errs := LookupServiceWithFallback(name, allServices)
	if svc == nil {
		return fmt.Errorf("service %q not found: %s", name, strings.Join(errs, "; "))
	}

	// Stop if running
	if pid, err := a.validatedManagedPID(svc); err != nil {
		return err
	} else if pid > 0 {
		fmt.Fprintf(a.outWriter(), "Stopping service %q...\n", svc.Name)
		if err := a.processManager.Stop(pid, 5000000000); err != nil { // 5 second timeout
			fmt.Fprintf(a.errWriter(), "Warning: failed to stop service: %v\n", err)
		}
	}

	// Start
	fmt.Fprintf(a.outWriter(), "Starting %q...\n", svc.Name)
	pid, err := a.processManager.Start(svc)
	if err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Update registry
	if err := a.registry.UpdateServicePID(svc.Name, pid); err != nil {
		fmt.Fprintf(a.errWriter(), "Warning: failed to update registry: %v\n", err)
	}

	fmt.Fprintf(a.outWriter(), "Restarted %q\n", svc.Name)
	return nil
}

// BatchStartCmd starts multiple services in sequence.
// Expands glob patterns against service names before execution.
// Continues processing after failures (partial failure handling).
// Returns error if any service fails to start.
func (a *App) BatchStartCmd(names []string) error {
	if len(names) == 0 {
		return fmt.Errorf("no service names provided")
	}

	// Expand glob patterns against registry
	services := a.registry.ListServices()
	expandedNames := ExpandPatterns(names, services)

	if len(expandedNames) == 0 {
		return fmt.Errorf("no services found matching patterns")
	}

	var anyFailure bool
	var firstErr error

	for _, name := range expandedNames {
		// Check if service exists (supports name:port format)
		allServices := a.registry.ListServices()
		svc, errs := LookupServiceWithFallback(name, allServices)
		if svc == nil {
			fmt.Fprintf(os.Stderr, "Error: service identifier %q not found: %s\n", name, strings.Join(errs, ", "))
			anyFailure = true
			if firstErr == nil {
				firstErr = fmt.Errorf("service %q not found: %s", name, strings.Join(errs, "; "))
			}
			continue
		}

		// Check if already running
		runningPID, err := a.validatedManagedPID(svc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			anyFailure = true
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if runningPID > 0 {
			fmt.Fprintf(os.Stderr, "Warning: service %q already running (PID %d)\n", name, runningPID)
			continue
		}

		// Attempt to start
		fmt.Printf("Starting %q...\n", name)
		pid, err := a.processManager.Start(svc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to start service %q: %v\n", name, err)
			anyFailure = true
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to start %q: %w", name, err)
			}
			continue
		}

		// Update registry with new PID
		if updateErr := a.registry.UpdateServicePID(svc.Name, pid); updateErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update registry for %q: %v\n", name, updateErr)
		}

		fmt.Printf("Started %q\n", name)
	}

	if anyFailure {
		return firstErr
	}
	return nil
}

// BatchStopCmd stops multiple services in sequence.
// Expands glob patterns against service names before execution.
// Continues processing after failures (partial failure handling).
// Returns error if any service fails to stop.
func (a *App) BatchStopCmd(names []string) error {
	if len(names) == 0 {
		return fmt.Errorf("no service names provided")
	}

	// Expand glob patterns against registry
	services := a.registry.ListServices()
	expandedNames := ExpandPatterns(names, services)

	if len(expandedNames) == 0 {
		return fmt.Errorf("no services found matching patterns")
	}

	var anyFailure bool
	var firstErr error

	for _, name := range expandedNames {
		// Check if service exists (supports name:port format)
		allServices := a.registry.ListServices()
		svc, errs := LookupServiceWithFallback(name, allServices)
		if svc == nil {
			fmt.Fprintf(os.Stderr, "Error: service identifier %q not found: %s\n", name, strings.Join(errs, ", "))
			anyFailure = true
			if firstErr == nil {
				firstErr = fmt.Errorf("service %q not found: %s", name, strings.Join(errs, "; "))
			}
			continue
		}

		// Determine PID to stop
		targetPID, err := a.validatedManagedPID(svc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			anyFailure = true
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if targetPID == 0 {
			fmt.Fprintf(os.Stderr, "Warning: service %q is not running\n", name)
			continue
		}

		// Attempt to stop
		fmt.Printf("Stopping service %q (PID %d)...\n", name, targetPID)
		if err := a.processManager.Stop(targetPID, 5000000000); err != nil { // 5 second timeout
			if errors.Is(err, process.ErrNeedSudo) {
				fmt.Fprintf(os.Stderr, "Error: requires sudo to terminate service %q (PID %d)\n", name, targetPID)
			} else if isProcessFinishedErr(err) {
				// Process already finished - clear PID and continue
				if clrErr := a.registry.ClearServicePID(svc.Name); clrErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to clear PID for %q: %v\n", name, clrErr)
				}
				fmt.Printf("Service %q already stopped\n", name)
				continue
			} else {
				fmt.Fprintf(os.Stderr, "Error: failed to stop service %q: %v\n", name, err)
				anyFailure = true
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to stop %q: %w", name, err)
				}
				continue
			}
		}

		fmt.Printf("Service %q stopped (PID %d)\n", name, targetPID)
		if clrErr := a.registry.ClearServicePID(svc.Name); clrErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clear PID for %q: %v\n", name, clrErr)
		}
	}

	if anyFailure {
		return firstErr
	}
	return nil
}

// BatchRestartCmd restarts multiple services in sequence.
// Expands glob patterns against service names before execution.
// Continues processing after failures (partial failure handling).
// Returns error if any service fails to restart.
func (a *App) BatchRestartCmd(names []string) error {
	if len(names) == 0 {
		return fmt.Errorf("no service names provided")
	}

	// Expand glob patterns against registry
	services := a.registry.ListServices()
	expandedNames := ExpandPatterns(names, services)

	if len(expandedNames) == 0 {
		return fmt.Errorf("no services found matching patterns")
	}

	var anyFailure bool
	var firstErr error

	for _, name := range expandedNames {
		// Check if service exists (supports name:port format)
		allServices := a.registry.ListServices()
		svc, errs := LookupServiceWithFallback(name, allServices)
		if svc == nil {
			fmt.Fprintf(os.Stderr, "Error: service identifier %q not found: %s\n", name, strings.Join(errs, ", "))
			anyFailure = true
			if firstErr == nil {
				firstErr = fmt.Errorf("service %q not found: %s", name, strings.Join(errs, "; "))
			}
			continue
		}

		// Stop if running
		runningPID, err := a.validatedManagedPID(svc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			anyFailure = true
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if runningPID > 0 {
			fmt.Printf("Stopping service %q (PID %d)...\n", name, runningPID)
			if stopErr := a.processManager.Stop(runningPID, 5000000000); stopErr != nil {
				if !errors.Is(stopErr, process.ErrNeedSudo) && !isProcessFinishedErr(stopErr) {
					fmt.Fprintf(os.Stderr, "Warning: failed to stop service %q: %v\n", name, stopErr)
				}
			}
		}

		// Start service
		fmt.Printf("Starting %q...\n", name)
		pid, err := a.processManager.Start(svc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to start service %q: %v\n", name, err)
			anyFailure = true
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to restart %q: %w", name, err)
			}
			continue
		}

		// Update registry with new PID
		if updateErr := a.registry.UpdateServicePID(svc.Name, pid); updateErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update registry for %q: %v\n", name, updateErr)
		}

		fmt.Printf("Restarted %q\n", name)
	}

	if anyFailure {
		return firstErr
	}
	return nil
}

// LogsCmd displays recent logs for a service
func (a *App) LogsCmd(name string, lines int) error {
	// Supports name:port format for disambiguation
	allServices := a.registry.ListServices()
	svc, errs := LookupServiceWithFallback(name, allServices)
	if svc == nil {
		return fmt.Errorf("service %q not found: %s", name, strings.Join(errs, "; "))
	}

	logLines, err := a.processManager.Tail(svc.Name, lines)
	if err != nil {
		return err
	}

	fmt.Printf("Logs for service %q:\n", svc.Name)
	for _, line := range logLines {
		fmt.Println(line)
	}

	return nil
}

func (a *App) validatedManagedPID(svc *models.ManagedService) (int, error) {
	servers, err := a.discoverServers()
	if err != nil {
		return 0, err
	}
	return ValidateRunningPID(svc, servers, a.processManager.IsRunning)
}

// StatusCmd shows detailed info for one or more servers.
// Identifiers may be exact names, port numbers, or glob patterns (e.g. "offg*").
// When multiple services match, status is shown for ALL of them.
func (a *App) StatusCmd(identifiers []string) error {
	servers, err := a.discoverServers()
	if err != nil {
		return err
	}

	// Build a set of all managed service names for pattern expansion.
	allServices := a.registry.ListServices()

	var matched []*models.ServerInfo

	for _, id := range identifiers {
		if strings.Contains(id, "*") {
			// Glob pattern: expand against service names
			expanded := ExpandPatterns([]string{id}, allServices)
			for _, name := range expanded {
				for _, srv := range servers {
					if srv.ManagedService != nil && srv.ManagedService.Name == name {
						matched = append(matched, srv)
						break
					}
				}
			}
		} else {
			// Exact match: by name or port
			for _, srv := range servers {
				if srv.ManagedService != nil && srv.ManagedService.Name == id {
					matched = append(matched, srv)
					break
				}
				if srv.ProcessRecord != nil && fmt.Sprintf("%d", srv.ProcessRecord.Port) == id {
					matched = append(matched, srv)
					break
				}
			}
		}
	}

	if len(matched) == 0 {
		return fmt.Errorf("no servers found matching %s", strings.Join(identifiers, ", "))
	}

	for _, srv := range matched {
		var hc *health.HealthCheck
		if srv.ProcessRecord != nil {
			hc = a.healthChecker.Check(srv.ProcessRecord.Port)
		}
		if err := PrintServerStatus(a.outWriter(), srv, hc); err != nil {
			return err
		}
	}
	return nil
}

// printServerStatus prints detailed status for a server (App method wrapper).
// Delegates to the package-level PrintServerStatus function with health check.
func (a *App) printServerStatus(srv *models.ServerInfo) error {
	var hc *health.HealthCheck
	if srv.ProcessRecord != nil {
		hc = a.healthChecker.Check(srv.ProcessRecord.Port)
	}
	return PrintServerStatus(a.outWriter(), srv, hc)
}
